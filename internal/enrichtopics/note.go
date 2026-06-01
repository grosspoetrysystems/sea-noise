package enrichtopics

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sea-noise/internal/ai/skill"
	"sea-noise/internal/markdown"
)

// MaxNoteContentChars caps how much of the note body gets sent to
// the enrich-single-note skill. Mirrors references/scripts/
// enrich-topics.py:collect_note_context's 3000-char truncation.
const MaxNoteContentChars = 3000

// MaxWebSectionChars caps the appended web-research block. Mirrors
// the prototype's 30000-char cap in enrich_single_note.
const MaxWebSectionChars = 30000

// ErrNoteNotFound is returned when EnrichNote is handed a path that
// doesn't exist on disk.
var ErrNoteNotFound = errors.New("enrichtopics: note not found")

// ErrAlreadyEnriched is returned when the note already contains a
// `## Research Context` or `## Follow-up Questions` section. The
// prototype skips silently in this case; the Go port surfaces it as
// a sentinel so callers can distinguish from "no skill output."
var ErrAlreadyEnriched = errors.New("enrichtopics: note already enriched")

// NoteInput configures an EnrichNote run.
type NoteInput struct {
	VaultRoot string
	// NotePath is the vault-relative path of the note to enrich. An
	// absolute path is also accepted; it is converted to vault-
	// relative for the log line.
	NotePath string
	Today    time.Time
}

// NoteResult reports outcomes.
type NoteResult struct {
	NotePath string // vault-relative
	LogPath  string // vault-relative path of the log entry
	Crawled  int    // number of crawled records folded into the prompt
}

// EnrichSingleNoteInput is the data the enrich-single-note skill
// template expects. Exported so parity-discovery callers can render
// the same prompt the live workflow would assemble without invoking
// the LLM.
type EnrichSingleNoteInput struct {
	Title      string
	Tags       string
	Source     string
	Content    string
	WebSection string
}

// NoteCandidate is one note that would be enriched, paired with the
// exact skill input EnrichNote would assemble. Produced by
// BuildEnrichNoteInput; consumed by parity-discovery.
type NoteCandidate struct {
	Rel        string                // vault-relative path
	Crawled    int                   // number of crawled records folded into the prompt
	SkillInput EnrichSingleNoteInput // exact input EnrichNote would pass to Execute
}

// BuildEnrichNoteInput performs all the pre-LLM work EnrichNote does
// — read the note, parse frontmatter, check already-enriched, build
// the prompt body (with Research Context stripped and content
// truncated), build the web section via the supplied Crawler, and
// assemble the EnrichSingleNoteInput. Pure read; never invokes the
// LLM and never writes back.
//
// Used by EnrichNote internally (single-sources the pre-LLM logic)
// and by parity-discovery to render Go-side prompts for envelope
// capture. crawler may be nil (treated as NoopCrawler).
func BuildEnrichNoteInput(ctx context.Context, vaultRoot, notePath string, crawler Crawler) (NoteCandidate, error) {
	abs := notePath
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(vaultRoot, notePath)
	}
	rel, _ := filepath.Rel(vaultRoot, abs)

	raw, err := os.ReadFile(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return NoteCandidate{}, fmt.Errorf("%w: %s", ErrNoteNotFound, notePath)
		}
		return NoteCandidate{}, fmt.Errorf("enrichtopics: read %s: %w", abs, err)
	}
	content := string(raw)
	fm, err := markdown.ParseFrontmatterYAML(content)
	if err != nil {
		return NoteCandidate{}, fmt.Errorf("enrichtopics: parse frontmatter %s: %w", abs, err)
	}
	body := fm.Body
	if strings.Contains(body, "## Research Context") || strings.Contains(body, "## Follow-up Questions") {
		return NoteCandidate{}, fmt.Errorf("%w: %s", ErrAlreadyEnriched, rel)
	}

	promptBody := body
	if i := strings.Index(promptBody, "## Research Context"); i > 0 {
		promptBody = strings.TrimRight(promptBody[:i], " \t\n")
	}
	if len(promptBody) > MaxNoteContentChars {
		promptBody = promptBody[:MaxNoteContentChars]
	}

	title := coerceString(fm.Fields["title"])
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(abs), ".md")
	}
	source := coerceString(fm.Fields["source"])
	tags := strings.Join(tagsFromField(fm.Fields["tags"]), ", ")

	webSection := ""
	crawled := 0
	if source != "" && strings.HasPrefix(source, "http") && crawler != nil {
		pages, _ := crawler.Crawl(ctx, source, 10, 1)
		if len(pages) > 0 {
			parts := make([]string, 0, len(pages))
			for _, p := range pages {
				parts = append(parts, fmt.Sprintf("### %s\n%s", p.URL, truncate(p.Markdown, 3000)))
			}
			joined := strings.Join(parts, "\n\n---\n\n")
			if len(joined) > MaxWebSectionChars {
				joined = joined[:MaxWebSectionChars]
			}
			webSection = "\n\n## Web Research\n" + joined
			crawled = len(pages)
		}
	}

	return NoteCandidate{
		Rel:     rel,
		Crawled: crawled,
		SkillInput: EnrichSingleNoteInput{
			Title:      title,
			Tags:       tags,
			Source:     source,
			Content:    promptBody,
			WebSection: webSection,
		},
	}, nil
}

// EnrichNote loads a single note, optionally crawls its source URL,
// generates the enrichment via the enrich-single-note skill, and
// appends the result to the note's body. The note's frontmatter
// gains an `enriched: YYYY-MM-DD` key.
func EnrichNote(ctx context.Context, exec *skill.Executor, crawler Crawler, in NoteInput) (NoteResult, error) {
	today := in.Today
	if today.IsZero() {
		today = time.Now()
	}

	cand, err := BuildEnrichNoteInput(ctx, in.VaultRoot, in.NotePath, crawler)
	if err != nil {
		return NoteResult{}, err
	}

	// Re-read the raw file for the final write-back. BuildEnrichNoteInput
	// did its own read but only kept the parsed pieces it needs; we need
	// the full bytes to splice the enrichment onto the end and update
	// the frontmatter.
	abs := in.NotePath
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(in.VaultRoot, in.NotePath)
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return NoteResult{}, fmt.Errorf("enrichtopics: re-read %s: %w", abs, err)
	}
	content := string(raw)

	enrichment, err := exec.Execute(ctx, "enrich-single-note", cand.SkillInput)
	if err != nil {
		return NoteResult{}, fmt.Errorf("enrichtopics: enrich-single-note: %w", err)
	}

	withSection := strings.TrimRight(content, " \t\n\v\f\r") + "\n\n" + strings.TrimSpace(enrichment) + "\n"
	withDated, err := appendEnrichedKey(withSection, today.Format("2006-01-02"))
	if err != nil {
		return NoteResult{}, err
	}
	if err := os.WriteFile(abs, []byte(withDated), 0o644); err != nil {
		return NoteResult{}, fmt.Errorf("enrichtopics: write %s: %w", abs, err)
	}

	logRel, err := appendLog(in.VaultRoot, strings.TrimSuffix(filepath.Base(abs), ".md"), cand.Rel, today)
	if err != nil {
		return NoteResult{}, err
	}
	return NoteResult{NotePath: cand.Rel, LogPath: logRel, Crawled: cand.Crawled}, nil
}

// appendEnrichedKey inserts `enriched: YYYY-MM-DD` immediately before
// the closing `---` of the note's frontmatter. If the key already
// exists, the value is left unchanged (the prototype overwrites; the
// Go port treats the key as immutable so re-runs don't churn the
// frontmatter every day).
func appendEnrichedKey(content, date string) (string, error) {
	if !strings.HasPrefix(content, "---") {
		return "", errors.New("enrichtopics: note has no frontmatter")
	}
	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", errors.New("enrichtopics: frontmatter not closed")
	}
	block := content[:3+idx]
	if strings.Contains(block, "\nenriched:") {
		return content, nil
	}
	insertAt := 3 + idx
	var b strings.Builder
	b.WriteString(strings.TrimRight(content[:insertAt], " \t\n"))
	b.WriteString("\nenriched: '")
	b.WriteString(date)
	b.WriteString("'")
	b.WriteString(content[insertAt:])
	return b.String(), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
