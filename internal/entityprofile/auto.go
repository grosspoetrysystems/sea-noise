package entityprofile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"sea-noise/internal/ai/skill"
	"sea-noise/internal/markdown"
)

// DefaultAutoThreshold mirrors references/scripts/profile-entities.py's
// THRESHOLD constant. An entity needs at least this many mentions to
// be a candidate.
const DefaultAutoThreshold = 5

// DefaultMaxAutoPerRun mirrors MAX_PROFILES_PER_RUN — the cap on how
// many profiles can ship in one auto invocation.
const DefaultMaxAutoPerRun = 5

// MaxCandidatesBeforeFilter mirrors `entities.most_common(50)` — the
// regex NER set is capped at this many top candidates before the
// bulk Claude filter call.
const MaxCandidatesBeforeFilter = 50

// profilesLogRelPath is the vault-relative path of the auto-profile log.
const profilesLogRelPath = "_meta/entity-profiles-log.md"

// excludeDirs mirrors the prototype's EXCLUDE_DIRS — names that
// rglob steps over while collecting candidates.
var excludeDirs = map[string]struct{}{
	"_rag": {}, "_meta": {}, "templates": {}, ".git": {}, ".obsidian": {},
	"_scripts": {}, ".claude": {},
}

// nerStopwords mirrors the prototype's STOPWORDS list. Lowercase
// match against any space-split word in a candidate name; if any word
// is in this set, the candidate is dropped before the Claude filter
// call.
var nerStopwords = map[string]struct{}{
	"the": {}, "and": {}, "for": {}, "with": {}, "that": {}, "this": {},
	"from": {}, "are": {}, "was": {}, "has": {}, "have": {}, "not": {},
	"but": {}, "can": {}, "will": {}, "how": {}, "what": {}, "why": {},
	"who": {}, "when": {}, "where": {}, "your": {}, "you": {}, "all": {},
	"its": {}, "use": {}, "new": {}, "one": {}, "two": {}, "more": {},
	"about": {}, "into": {}, "just": {}, "also": {}, "like": {},
	"them": {}, "than": {}, "been": {}, "some": {}, "could": {},
	"would": {}, "should": {}, "their": {}, "each": {}, "which": {},
	"these": {}, "other": {}, "only": {}, "over": {}, "such": {},
	"very": {}, "does": {}, "most": {}, "being": {}, "between": {},
	"after": {}, "before": {}, "using": {}, "need": {}, "code": {},
	"data": {}, "note": {}, "notes": {}, "vault": {}, "idea": {},
	"ideas": {}, "reference": {}, "todo": {}, "work": {}, "personal": {},
	"raw": {}, "tags": {}, "source": {},
}

// nerPattern mirrors the prototype's capitalized-2-to-4-word phrase
// regex used as cheap NER.
var nerPattern = regexp.MustCompile(`\b([A-Z][a-z]+(?:\s+[A-Z][a-z]+){1,3})\b`)

// AutoInput configures GenerateAuto.
type AutoInput struct {
	VaultRoot string
	Threshold int       // 0 → DefaultAutoThreshold
	MaxPerRun int       // 0 → DefaultMaxAutoPerRun
	Today     time.Time // 0 → time.Now()
}

// AutoResult reports per-run outcomes.
type AutoResult struct {
	Considered int      // raw entity candidates above threshold (pre-Claude filter)
	Profiled   []Result // per-entity profile output from Generate
	Skipped    []string // candidate names dropped by stopword filter, log dedup, threshold, or Claude filter
	Failed     []string // candidate names where Generate returned an error
}

// candidateClassification is one row of the filter-entity-candidates
// skill's JSON output. The prototype prompt expects exactly these keys.
type candidateClassification struct {
	Name string `json:"name"`
	Keep bool   `json:"keep"`
	Type string `json:"type"`
}

// filterEntityCandidatesInput is the data the filter-entity-candidates
// skill template expects.
type filterEntityCandidatesInput struct {
	CandidatesText string
}

// GenerateAuto is the vault-wide auto-mode entry point. It scans
// every markdown note outside excludeDirs, applies cheap regex NER
// plus a stopword filter, picks the top MaxCandidatesBeforeFilter
// names that are at or above Threshold and not already in the
// auto-profile log, asks the configured executor's
// `filter-entity-candidates` skill to classify the candidates as
// keep/drop, caps the survivors at MaxPerRun, and finally runs the
// existing single-entity Generate on each kept entity.
func GenerateAuto(ctx context.Context, exec *skill.Executor, in AutoInput) (AutoResult, error) {
	var result AutoResult
	threshold := in.Threshold
	if threshold <= 0 {
		threshold = DefaultAutoThreshold
	}
	cap := in.MaxPerRun
	if cap <= 0 {
		cap = DefaultMaxAutoPerRun
	}

	counts, err := extractEntityCounts(in.VaultRoot)
	if err != nil {
		return result, err
	}
	profiled, err := readProfilesLog(in.VaultRoot)
	if err != nil {
		return result, err
	}

	// Top-N filter, threshold, dedup against the log.
	type ranked struct {
		name  string
		count int
	}
	ranks := make([]ranked, 0, len(counts))
	for name, count := range counts {
		ranks = append(ranks, ranked{name: name, count: count})
	}
	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].count != ranks[j].count {
			return ranks[i].count > ranks[j].count
		}
		return ranks[i].name < ranks[j].name
	})
	if len(ranks) > MaxCandidatesBeforeFilter {
		ranks = ranks[:MaxCandidatesBeforeFilter]
	}
	candidates := make([]ranked, 0, len(ranks))
	for _, r := range ranks {
		if r.count < threshold {
			continue
		}
		if profiled[r.name] {
			result.Skipped = append(result.Skipped, r.name)
			continue
		}
		candidates = append(candidates, r)
	}
	result.Considered = len(candidates)
	if len(candidates) == 0 {
		return result, nil
	}

	// Bulk classification via the filter-entity-candidates skill.
	names := make([]string, len(candidates))
	for i, c := range candidates {
		names[i] = c.name
	}
	candidatesText := strings.Join(prefixDashes(names), "\n")
	raw, err := exec.Execute(ctx, "filter-entity-candidates", filterEntityCandidatesInput{
		CandidatesText: candidatesText,
	})
	keep := map[string]string{} // name → entity type
	if err == nil {
		if classified, perr := parseClassifications(raw); perr == nil {
			for _, c := range classified {
				if c.Keep {
					keep[c.Name] = c.Type
				}
			}
		}
	}
	// Fall-back posture mirrors the prototype: on filter failure or
	// empty parse, keep every candidate so the user still gets results.
	if len(keep) == 0 {
		for _, c := range candidates {
			keep[c.name] = "concept"
		}
	}

	// Cap + execute. Iterate in the ranked order so the cap selects
	// the highest-count candidates deterministically.
	for _, c := range candidates {
		etype, ok := keep[c.name]
		if !ok {
			result.Skipped = append(result.Skipped, c.name)
			continue
		}
		if len(result.Profiled) >= cap {
			result.Skipped = append(result.Skipped, c.name)
			continue
		}
		res, gerr := Generate(ctx, exec, Input{
			VaultRoot:  in.VaultRoot,
			Entity:     c.name,
			EntityType: etype,
			Today:      in.Today,
		})
		if gerr != nil {
			result.Failed = append(result.Failed, c.name)
			continue
		}
		result.Profiled = append(result.Profiled, res)
	}
	return result, nil
}

// --- helpers ---

// extractEntityCounts walks the vault and counts how many notes each
// regex-detected entity appears in (mirrors the prototype: counts
// per-mention, not per-note, but with the same regex shape).
func extractEntityCounts(vaultRoot string) (map[string]int, error) {
	out := map[string]int{}
	err := filepath.Walk(vaultRoot, func(path string, info os.FileInfo, werr error) error {
		if werr != nil {
			if errors.Is(werr, fs.ErrNotExist) {
				return filepath.SkipDir
			}
			return werr
		}
		rel, _ := filepath.Rel(vaultRoot, path)
		// Skip excluded directories at any depth.
		for _, seg := range strings.Split(rel, string(filepath.Separator)) {
			if _, ok := excludeDirs[seg]; ok {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		fm, perr := markdown.ParseFrontmatterYAML(string(raw))
		if perr != nil {
			return nil
		}
		// Strip "## Research Context" enrichment + markdown headings
		// before NER, matching the prototype's strip_headings_and_
		// structure + research-context cut.
		body := fm.Body
		if i := strings.Index(body, "## Research Context"); i >= 0 {
			body = body[:i]
		}
		body = stripStructuralLines(body)
		for _, m := range nerPattern.FindAllStringSubmatch(body, -1) {
			name := m[1]
			if isFilteredEntity(name) {
				continue
			}
			out[name]++
		}
		if title, ok := fm.Fields["title"].(string); ok && title != "" {
			for _, m := range nerPattern.FindAllStringSubmatch(title, -1) {
				name := m[1]
				if isFilteredEntity(name) {
					continue
				}
				out[name]++
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("entityprofile: walk %s: %w", vaultRoot, err)
	}
	return out, nil
}

// stripStructuralLines mirrors strip_headings_and_structure: drop
// lines that start with `#` (markdown heading) or `|` (table row)
// before running NER over the remainder.
func stripStructuralLines(text string) string {
	var b strings.Builder
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "|") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// isFilteredEntity returns true when any space-split word of the
// candidate is in the stopword set. Mirrors the prototype.
func isFilteredEntity(name string) bool {
	for _, w := range strings.Fields(strings.ToLower(name)) {
		if _, ok := nerStopwords[w]; ok {
			return true
		}
	}
	return false
}

// readProfilesLog returns the set of entities already in
// `_meta/entity-profiles-log.md`. Mirrors the prototype's regex
// `^- (.+?) —` over the log's contents.
func readProfilesLog(vaultRoot string) (map[string]bool, error) {
	path := filepath.Join(vaultRoot, profilesLogRelPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("entityprofile: read %s: %w", path, err)
	}
	out := map[string]bool{}
	re := regexp.MustCompile(`(?m)^- (.+?) —`)
	for _, m := range re.FindAllStringSubmatch(string(raw), -1) {
		out[m[1]] = true
	}
	return out, nil
}

// parseClassifications extracts the first JSON array from the skill
// response (the response may be wrapped in markdown fences) and
// decodes it. Mirrors the prototype's `re.search(r'\[.*\]', result,
// re.DOTALL)` extraction.
var jsonArrayPattern = regexp.MustCompile(`(?s)\[.*\]`)

func parseClassifications(raw string) ([]candidateClassification, error) {
	m := jsonArrayPattern.FindString(raw)
	if m == "" {
		return nil, errors.New("entityprofile: no JSON array in filter response")
	}
	var out []candidateClassification
	if err := json.Unmarshal([]byte(m), &out); err != nil {
		return nil, fmt.Errorf("entityprofile: decode filter response: %w", err)
	}
	return out, nil
}

func prefixDashes(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = "- " + n
	}
	return out
}
