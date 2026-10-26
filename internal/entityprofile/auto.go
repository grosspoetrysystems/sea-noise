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
	"sea-noise/internal/vault"
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

// CandidateClassification is one row of the filter-entity-candidates
// skill's JSON output. The prototype prompt expects exactly these keys.
type CandidateClassification struct {
	Name string `json:"name"`
	Keep bool   `json:"keep"`
	Type string `json:"type"`
}

// FilterEntityCandidatesInput is the data the filter-entity-candidates
// skill template expects. Exported so parity-discovery callers can
// render the same prompt the live workflow would assemble without
// invoking the LLM.
type FilterEntityCandidatesInput struct {
	CandidatesText string
}

// rankedCandidate is one entity candidate plus its mention count, in
// the ranked + threshold-filtered + log-deduped order GenerateAuto
// would pass to the filter skill.
type rankedCandidate struct {
	Name  string
	Count int
}

// EntityFilterCandidate bundles the candidate names that survived
// ranking + threshold + log-dedup, paired with the exact skill input
// the live workflow would pass to the filter-entity-candidates skill.
// Names is empty when no candidate qualifies (the live workflow would
// no-op in that case).
type EntityFilterCandidate struct {
	Names      []string
	SkillInput FilterEntityCandidatesInput
}

// EntityProfileCandidate is one post-filter, post-cap entity that the
// live workflow would profile, paired with the exact skill input
// Generate would assemble for the generate-entity-profile skill and
// the raw mentions used to construct the profile note's backlinks
// section.
type EntityProfileCandidate struct {
	Entity     string
	EntityType string
	Mentions   []Mention
	SkillInput GenerateEntityProfileInput
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

	candidates, skippedFromLog, err := rankCandidates(in)
	if err != nil {
		return result, err
	}
	result.Skipped = append(result.Skipped, skippedFromLog...)
	result.Considered = len(candidates)
	if len(candidates) == 0 {
		return result, nil
	}

	// Bulk classification via the filter-entity-candidates skill.
	candidatesText := buildCandidatesText(candidates)
	raw, err := exec.Execute(ctx, "filter-entity-candidates", FilterEntityCandidatesInput{
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
			keep[c.Name] = "concept"
		}
	}

	// Pre-compute profile candidates (with mentions) against the
	// INITIAL vault state. Mirrors the prototype's auto_mode shape:
	// `notes = collect_vault_notes()` once, then `find_notes_mentioning`
	// operates on the cached set. Re-walking between profile writes
	// would let earlier-written profile files mutate later mention
	// counts — a parity break against the prototype.
	profileCandidates, err := WalkEntityProfileCandidates(in, keep)
	if err != nil {
		return result, err
	}

	if in.Today.IsZero() {
		in.Today = time.Now()
	}

	for _, c := range profileCandidates {
		body, gerr := exec.Execute(ctx, "generate-entity-profile", c.SkillInput)
		if gerr != nil {
			result.Failed = append(result.Failed, c.Entity)
			continue
		}
		profileRel, werr := writeProfile(in.VaultRoot, c.Entity, c.EntityType, c.Mentions, body, in.Today)
		if werr != nil {
			return result, werr
		}
		logRel, lerr := appendLog(in.VaultRoot, c.Entity, profileRel, in.Today)
		if lerr != nil {
			return result, lerr
		}
		result.Profiled = append(result.Profiled, Result{
			ProfilePath: profileRel,
			LogPath:     logRel,
			Mentions:    len(c.Mentions),
		})
	}

	// Surface entities that the keep set didn't include (the live
	// workflow's filter step dropped them). Mirrors the prototype's
	// "show what was filtered out" affordance.
	for _, c := range candidates {
		if _, ok := keep[c.Name]; !ok {
			result.Skipped = append(result.Skipped, c.Name)
		}
	}
	return result, nil
}

// WalkEntityFilterCandidate runs the candidate ranking + threshold +
// log-dedup the live workflow would perform and returns the post-rank
// candidate names paired with the exact skill input the
// filter-entity-candidates skill would receive. Pure read; never
// invokes the LLM. The returned EntityFilterCandidate's Names is empty
// (and SkillInput.CandidatesText is empty) when no candidate would
// survive — equivalent to the live workflow no-opping.
//
// Used by parity-discovery (`sea-noise parity-discover`) to render the
// filter-pass prompt for envelope capture.
func WalkEntityFilterCandidate(in AutoInput) (EntityFilterCandidate, error) {
	candidates, _, err := rankCandidates(in)
	if err != nil {
		return EntityFilterCandidate{}, err
	}
	if len(candidates) == 0 {
		return EntityFilterCandidate{}, nil
	}
	names := make([]string, len(candidates))
	for i, c := range candidates {
		names[i] = c.Name
	}
	return EntityFilterCandidate{
		Names: names,
		SkillInput: FilterEntityCandidatesInput{
			CandidatesText: buildCandidatesText(candidates),
		},
	}, nil
}

// WalkEntityProfileCandidates re-runs the candidate ranking, applies
// the supplied keep set (post-filter name → entity-type classifications,
// typically parsed from a captured filter envelope), caps at MaxPerRun,
// finds mentions per kept entity, and returns one EntityProfileCandidate
// per kept entity in ranked order. Pure read; never invokes the LLM.
//
// Used by parity-discovery (`sea-noise parity-discover`) to render the
// per-entity profile prompts on the second discover pass, once the
// filter envelope has been captured and its keep set is known.
func WalkEntityProfileCandidates(in AutoInput, keep map[string]string) ([]EntityProfileCandidate, error) {
	candidates, _, err := rankCandidates(in)
	if err != nil {
		return nil, err
	}
	cap := in.MaxPerRun
	if cap <= 0 {
		cap = DefaultMaxAutoPerRun
	}

	out := make([]EntityProfileCandidate, 0, len(candidates))
	for _, c := range candidates {
		etype, ok := keep[c.Name]
		if !ok {
			continue
		}
		if len(out) >= cap {
			break
		}
		mentions, err := findMentions(in.VaultRoot, c.Name)
		if err != nil {
			return nil, err
		}
		if len(mentions) == 0 {
			continue
		}
		out = append(out, EntityProfileCandidate{
			Entity:     c.Name,
			EntityType: etype,
			Mentions:   mentions,
			SkillInput: GenerateEntityProfileInput{
				Entity:       c.Name,
				MentionsText: formatMentions(mentions),
				MentionCount: len(mentions),
			},
		})
	}
	return out, nil
}

// rankCandidates performs steps 1-3 of GenerateAuto's pipeline:
// extract entity counts, dedup against the profiles log, threshold,
// and sort by (count desc, name asc) truncated at
// MaxCandidatesBeforeFilter. Returns the ranked candidates plus the
// names skipped because they were already in the log (callers
// surface these in AutoResult.Skipped).
//
// Single-sources the qualification logic so the live workflow and
// parity-discovery can't drift.
func rankCandidates(in AutoInput) ([]rankedCandidate, []string, error) {
	threshold := in.Threshold
	if threshold <= 0 {
		threshold = DefaultAutoThreshold
	}

	counts, err := extractEntityCounts(in.VaultRoot)
	if err != nil {
		return nil, nil, err
	}
	profiled, err := readProfilesLog(in.VaultRoot)
	if err != nil {
		return nil, nil, err
	}

	ranks := make([]rankedCandidate, 0, len(counts))
	for name, count := range counts {
		ranks = append(ranks, rankedCandidate{Name: name, Count: count})
	}
	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].Count != ranks[j].Count {
			return ranks[i].Count > ranks[j].Count
		}
		return ranks[i].Name < ranks[j].Name
	})
	if len(ranks) > MaxCandidatesBeforeFilter {
		ranks = ranks[:MaxCandidatesBeforeFilter]
	}

	var skippedFromLog []string
	candidates := make([]rankedCandidate, 0, len(ranks))
	for _, r := range ranks {
		if r.Count < threshold {
			continue
		}
		if profiled[r.Name] {
			skippedFromLog = append(skippedFromLog, r.Name)
			continue
		}
		candidates = append(candidates, r)
	}
	return candidates, skippedFromLog, nil
}

// buildCandidatesText renders the candidate list into the exact
// `- <name>` newline-separated form the filter-entity-candidates skill
// template's CandidatesText field expects.
func buildCandidatesText(candidates []rankedCandidate) string {
	names := make([]string, len(candidates))
	for i, c := range candidates {
		names[i] = c.Name
	}
	return strings.Join(prefixDashes(names), "\n")
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
		raw, rerr := vault.ReadFileRetry(path)
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
	raw, err := vault.ReadFileRetry(path)
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

func parseClassifications(raw string) ([]CandidateClassification, error) {
	m := jsonArrayPattern.FindString(raw)
	if m == "" {
		return nil, errors.New("entityprofile: no JSON array in filter response")
	}
	var out []CandidateClassification
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
