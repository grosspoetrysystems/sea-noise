// Package enrichtopics ports the manual-topic path of
// references/scripts/enrich-topics.py. Auto-mode (driven by
// _meta/topic-frequencies.md) and the per-note enrichment path are
// deferred — both depend on follow-up infrastructure (frequency
// generation, transcript-aware note enrichment) not yet ported.
//
// Manual mode: given a topic string, scan the vault for notes whose
// tags or filename slug contain the topic, optionally crawl their
// source URLs via the Crawler interface (Cloudflare by default,
// no-op fallback), render the generate-research-brief skill, and
// write the resulting brief to notes/references/briefs/.
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

// Sentinel errors. Wrap with %w; check via errors.Is.
var (
	// ErrTopicNotFound is returned when no vault notes match the
	// supplied topic via tag-substring or slug-substring match.
	ErrTopicNotFound = errors.New("enrichtopics: topic has no matching notes in vault")

	// ErrEnrich is the wrapper for skill execution and IO failures
	// from EnrichTopic.
	ErrEnrich = errors.New("enrichtopics: enrichment failed")
)

// Default paths and limits mirror the prototype.
const (
	briefsRelDir = "notes/references/briefs"
	logRelPath   = "_meta/enrichment-log.md"

	// MaxCrawlSeeds caps how many note source URLs are crawled per
	// topic. Mirrors `source_urls[:3]` in the prototype.
	MaxCrawlSeeds = 3
	// CrawlLimit / CrawlDepth mirror crawl_url(url, limit=5, depth=1).
	CrawlLimit = 5
	CrawlDepth = 1
	// MaxRecordsInPrompt caps how many crawled records get folded
	// into the LLM prompt. Mirrors all_records[:10].
	MaxRecordsInPrompt = 10
	// CrawlMarkdownChars caps per-record markdown sent to the LLM.
	CrawlMarkdownChars = 3000
	// ExcerptChars caps each note's body excerpt in the prompt.
	ExcerptChars = 500
)

// excludeDirs mirrors the prototype's EXCLUDE_DIRS for vault scanning.
var excludeDirs = map[string]struct{}{
	"_rag": {}, "_meta": {}, "templates": {}, ".git": {}, ".obsidian": {},
	"_scripts": {}, ".claude": {},
}

// Input configures an EnrichTopic run.
type Input struct {
	VaultRoot string
	Topic     string

	// Today is injected for deterministic file naming in tests. Zero
	// value -> time.Now().
	Today time.Time
}

// Result reports outcomes.
type Result struct {
	BriefPath string // vault-relative path of the new brief
	LogPath   string // vault-relative path of the log entry
	Notes     int    // number of vault notes matched on this topic
	Crawled   int    // number of crawled records folded into the prompt
}

// matchedNote is one vault note that matched the topic.
type matchedNote struct {
	Path    string // vault-relative
	Stem    string // filename without .md
	Title   string
	Tags    []string
	Source  string
	Excerpt string // first ExcerptChars characters of body
}

// GenerateResearchBriefInput is the data the generate-research-brief
// skill template expects. Field names map to the {{.Topic}}, {{.NotesSummary}}
// etc. references in the skill body. Exported so parity-discovery callers
// can render the same prompt the live workflow would assemble without
// invoking the LLM.
type GenerateResearchBriefInput struct {
	Topic        string
	NoteCount    int
	NotesSummary string
	WebSection   string
}

// EnrichTopic scans the vault for notes matching the topic, optionally
// crawls their source URLs through crawler, invokes the
// generate-research-brief skill, and writes the brief.
//
// crawler may be NoopCrawler to skip web context; the prototype
// behaves the same way when CF credentials are absent.
func EnrichTopic(ctx context.Context, exec *skill.Executor, crawler Crawler, in Input) (Result, error) {
	if in.Today.IsZero() {
		in.Today = time.Now()
	}
	if crawler == nil {
		crawler = NoopCrawler{}
	}

	notes, err := collectNotesForTopic(in.VaultRoot, in.Topic)
	if err != nil {
		return Result{}, err
	}
	if len(notes) == 0 {
		return Result{}, fmt.Errorf("%w: %q", ErrTopicNotFound, in.Topic)
	}

	crawled, webSection := assembleWebSection(ctx, crawler, notes)

	body, err := exec.Execute(ctx, "generate-research-brief", GenerateResearchBriefInput{
		Topic:        in.Topic,
		NoteCount:    len(notes),
		NotesSummary: formatNotesSummary(notes),
		WebSection:   webSection,
	})
	if err != nil {
		return Result{}, fmt.Errorf("%w: skill: %v", ErrEnrich, err)
	}

	briefRel, err := writeBrief(in.VaultRoot, in.Topic, notes, body, in.Today)
	if err != nil {
		return Result{}, err
	}
	logRel, err := appendLog(in.VaultRoot, in.Topic, briefRel, in.Today)
	if err != nil {
		return Result{}, err
	}

	return Result{
		BriefPath: briefRel,
		LogPath:   logRel,
		Notes:     len(notes),
		Crawled:   crawled,
	}, nil
}

// collectNotesForTopic walks the vault for *.md files (skipping
// excludeDirs) and returns those whose tags or filename slug match the
// topic substring. Match semantics mirror the prototype exactly:
//
//   - tag_match: any tag (with leading '#' stripped) contains the topic
//     as a substring
//   - slug_match: topic with '-' → ' ' is a substring of stem with
//     '-' → ' '
func collectNotesForTopic(vaultRoot, topic string) ([]matchedNote, error) {
	var out []matchedNote
	err := filepath.WalkDir(vaultRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if _, skip := excludeDirs[d.Name()]; skip && path != vaultRoot {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, err := filepath.Rel(vaultRoot, path)
		if err != nil {
			return err
		}
		stem := strings.TrimSuffix(filepath.Base(path), ".md")
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable, mirror prototype's try/except
		}
		fm, err := markdown.ParseFrontmatterYAML(string(raw))
		if err != nil {
			return nil
		}
		tags := tagsFromField(fm.Fields["tags"])

		// Tag match: substring inside any tag value, after stripping '#'.
		tagMatch := false
		for _, t := range tags {
			t = strings.TrimPrefix(strings.TrimSpace(t), "#")
			if strings.Contains(t, topic) {
				tagMatch = true
				break
			}
		}
		// Slug match: topic with dashes → spaces inside stem with dashes → spaces.
		slugMatch := strings.Contains(
			strings.ReplaceAll(stem, "-", " "),
			strings.ReplaceAll(topic, "-", " "),
		)

		if !tagMatch && !slugMatch {
			return nil
		}

		title := coerceString(fm.Fields["title"])
		if title == "" {
			title = stem
		}
		excerpt := fm.Body
		if len(excerpt) > ExcerptChars {
			excerpt = excerpt[:ExcerptChars]
		}
		out = append(out, matchedNote{
			Path:    filepath.ToSlash(rel),
			Stem:    stem,
			Title:   title,
			Tags:    tags,
			Source:  coerceString(fm.Fields["source"]),
			Excerpt: excerpt,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: walk: %v", ErrEnrich, err)
	}
	return out, nil
}

// assembleWebSection crawls up to MaxCrawlSeeds note source URLs and
// returns (number-of-records-folded-in, formatted-section-string).
// Empty section when nothing was crawled.
func assembleWebSection(ctx context.Context, crawler Crawler, notes []matchedNote) (int, string) {
	var seeds []string
	for _, n := range notes {
		if n.Source != "" && strings.HasPrefix(n.Source, "http") {
			seeds = append(seeds, n.Source)
		}
		if len(seeds) >= MaxCrawlSeeds {
			break
		}
	}
	if len(seeds) == 0 {
		return 0, ""
	}

	var records []Page
	for _, u := range seeds {
		pages, err := crawler.Crawl(ctx, u, CrawlLimit, CrawlDepth)
		if err != nil {
			// Per the prototype, crawl failures are tolerated — they
			// log and proceed. We do the same: drop the seed and move on.
			continue
		}
		records = append(records, pages...)
	}
	if len(records) == 0 {
		return 0, ""
	}
	if len(records) > MaxRecordsInPrompt {
		records = records[:MaxRecordsInPrompt]
	}

	var sb strings.Builder
	sb.WriteString("\n\n## Web Research\n")
	for i, r := range records {
		if i > 0 {
			sb.WriteString("\n\n---\n\n")
		}
		md := r.Markdown
		if len(md) > CrawlMarkdownChars {
			md = md[:CrawlMarkdownChars]
		}
		fmt.Fprintf(&sb, "### %s\n%s", nonEmpty(r.URL, "unknown"), md)
	}
	return len(records), sb.String()
}

// formatNotesSummary renders the notes list in the prototype's shape:
//
//	- <title> (<path>): <excerpt[:200]>...
func formatNotesSummary(notes []matchedNote) string {
	var sb strings.Builder
	for i, n := range notes {
		if i > 0 {
			sb.WriteString("\n")
		}
		snippet := n.Excerpt
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		fmt.Fprintf(&sb, "- %s (%s): %s...", n.Title, n.Path, snippet)
	}
	return sb.String()
}

// writeBrief writes the research-brief markdown to
// notes/references/briefs/<date>-<slug>-research-brief.md.
func writeBrief(vaultRoot, topic string, notes []matchedNote, enrichment string, today time.Time) (string, error) {
	slug := slugify(topic)
	filename := fmt.Sprintf("%s-%s-research-brief.md", today.Format("2006-01-02"), slug)
	relDir := briefsRelDir
	absDir := filepath.Join(vaultRoot, relDir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return "", fmt.Errorf("%w: mkdir %s: %v", ErrEnrich, absDir, err)
	}
	abs := filepath.Join(absDir, filename)

	var sb strings.Builder
	fmt.Fprintf(&sb, "---\n")
	fmt.Fprintf(&sb, "title: \"Research Brief: %s\"\n", topic)
	fmt.Fprintf(&sb, "tags:\n  - reference\n  - reference/brief\n  - research-brief\n  - %s\n", slug)
	fmt.Fprintf(&sb, "date: %s\n", today.Format("2006-01-02"))
	fmt.Fprintf(&sb, "source: enrich-topics\n")
	fmt.Fprintf(&sb, "---\n\n")
	fmt.Fprintf(&sb, "# Research Brief: %s\n\n", topic)
	fmt.Fprintf(&sb, "Auto-generated enrichment for topic cluster **%s** (%d notes, threshold crossed).\n\n", topic, len(notes))
	fmt.Fprintf(&sb, "## Vault Notes in This Cluster\n")
	for _, n := range notes {
		fmt.Fprintf(&sb, "- [[%s]]\n", n.Stem)
	}
	fmt.Fprintf(&sb, "\n%s\n", enrichment)

	if err := os.WriteFile(abs, []byte(sb.String()), 0o644); err != nil {
		return "", fmt.Errorf("%w: write %s: %v", ErrEnrich, abs, err)
	}
	return filepath.ToSlash(filepath.Join(relDir, filename)), nil
}

// appendLog adds one line to _meta/enrichment-log.md.
func appendLog(vaultRoot, topic, briefRel string, today time.Time) (string, error) {
	abs := filepath.Join(vaultRoot, logRelPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", fmt.Errorf("%w: mkdir log: %v", ErrEnrich, err)
	}
	line := fmt.Sprintf("- %s — %s — %s\n", topic, today.Format("2006-01-02"), briefRel)
	existing, err := os.ReadFile(abs)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("%w: read log: %v", ErrEnrich, err)
		}
		existing = []byte("# Enrichment Log\n\n")
	}
	out := append(existing, []byte(line)...)
	if err := os.WriteFile(abs, out, 0o644); err != nil {
		return "", fmt.Errorf("%w: write log: %v", ErrEnrich, err)
	}
	return filepath.ToSlash(logRelPath), nil
}

// slugify lowercases the topic and replaces internal whitespace with '-'.
// Mirrors `topic.lower().replace(" ", "-")` in the prototype.
func slugify(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), " ", "-")
}

// --- coercion helpers (same shape as internal/signal) ---

func coerceString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case time.Time:
		return x.Format("2006-01-02")
	}
	return fmt.Sprint(v)
}

func tagsFromField(v any) []string {
	switch x := v.(type) {
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	case string:
		parts := strings.Split(x, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return nil
}

func nonEmpty(a, fallback string) string {
	if a == "" {
		return fallback
	}
	return a
}
