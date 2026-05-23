// Package signal generates the rolling signal/queue.md from unconsumed
// notes in the vault, mirroring references/scripts/generate-signal.py.
//
// Pure-Go deterministic transform — no LLM, no network. Scoring weights
// and bucket thresholds match the Python prototype byte-for-byte.
package signal

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"sea-noise/internal/markdown"
)

// Tunables — mirror references/scripts/generate-signal.py module constants.
const (
	WPMArticle    = 250
	WPMTranscript = 150
	StaleDays     = 14
)

// Default vault-relative paths used by the prototype. The walker scans
// these directories only — full-vault scan is intentionally NOT used
// here (matches Python's SCAN_DIRS list).
var (
	scanDirs      = []string{"notes/fleeting", "notes/references"}
	assertionsDir = "notes/assertions"
	topicFreqPath = "_meta/topic-frequencies.md"
	signalDir     = "signal"
	queueRelPath  = "signal/queue.md"
)

// ErrSignal is the sentinel wrapper for failures from BuildQueue. Wrap
// with %w; check via errors.Is.
var ErrSignal = errors.New("signal: build failed")

// BuildAndWrite is the entry point: assemble the queue markdown and
// write it to <vaultRoot>/signal/queue.md. Returns the count of items
// in the queue and the relative output path on success.
func BuildAndWrite(vaultRoot string, now time.Time) (count int, outRel string, err error) {
	body, n, err := Build(vaultRoot, now)
	if err != nil {
		return 0, "", err
	}
	outDir := filepath.Join(vaultRoot, signalDir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return 0, "", fmt.Errorf("%w: mkdir %s: %v", ErrSignal, outDir, err)
	}
	out := filepath.Join(outDir, "queue.md")
	if err := os.WriteFile(out, []byte(body), 0o644); err != nil {
		return 0, "", fmt.Errorf("%w: write %s: %v", ErrSignal, out, err)
	}
	return n, queueRelPath, nil
}

// Build returns the rendered queue.md content for vaultRoot at now,
// without writing to disk. Useful for tests and callers that want to
// emit the queue somewhere other than the default location.
func Build(vaultRoot string, now time.Time) (string, int, error) {
	assertionTags, err := loadAssertionTags(vaultRoot)
	if err != nil {
		return "", 0, err
	}
	risingTopics, err := loadRisingTopics(vaultRoot)
	if err != nil {
		return "", 0, err
	}

	items, err := collectItems(vaultRoot, assertionTags, risingTopics, now)
	if err != nil {
		return "", 0, err
	}

	// Sort: score descending; ties broken by date ascending (matches
	// Python: items.sort(key=..., reverse=False) followed by stable
	// sort by -score; net effect is score desc, then date asc).
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].dateStr < items[j].dateStr
	})
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})

	return renderQueue(items, now), len(items), nil
}

// item is one row in the queue, post-scoring.
type item struct {
	score       int
	minutes     int
	title       string
	relPath     string
	tags        []string
	source      string
	dateStr     string
	contentType string
}

// loadAssertionTags scans notes/assertions/*.md for active assertions
// and returns the union of tier-2 domain tags (tags containing "/").
// Missing directory is not an error (matches Python).
func loadAssertionTags(vaultRoot string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	dir := filepath.Join(vaultRoot, assertionsDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return out, nil
		}
		return nil, fmt.Errorf("%w: read %s: %v", ErrSignal, dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		fm, ferr := markdown.ParseFrontmatterYAML(string(raw))
		if ferr != nil {
			continue
		}
		status := strings.ToLower(coerceString(fm.Fields["status"]))
		if status == "" {
			status = "active"
		}
		if status != "active" {
			continue
		}
		for _, t := range tagsFromField(fm.Fields["tags"]) {
			t = normalizeTag(t)
			if strings.Contains(t, "/") {
				out[t] = struct{}{}
			}
		}
	}
	return out, nil
}

// loadRisingTopics scans _meta/topic-frequencies.md for table rows whose
// fourth column contains the word "rising" (case-insensitive). The first
// column of each such row, stripped of leading "#", is the rising tag.
// Missing file is not an error (matches Python).
func loadRisingTopics(vaultRoot string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	path := filepath.Join(vaultRoot, topicFreqPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return out, nil
		}
		return nil, fmt.Errorf("%w: read %s: %v", ErrSignal, path, err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.Contains(strings.ToLower(line), "rising") {
			continue
		}
		// Parse table row: | tag | count | last_seen | trend |
		parts := splitTableRow(line)
		if len(parts) == 0 {
			continue
		}
		tag := strings.TrimPrefix(strings.TrimSpace(parts[0]), "#")
		if tag != "" {
			out[tag] = struct{}{}
		}
	}
	return out, nil
}

// splitTableRow splits a markdown table row on "|" and drops empty
// segments. Mirrors `[p.strip() for p in line.split("|") if p.strip()]`.
func splitTableRow(line string) []string {
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// collectItems walks the scan directories and returns one scored item
// per unconsumed eligible note.
func collectItems(vaultRoot string, assertionTags, risingTopics map[string]struct{}, now time.Time) ([]item, error) {
	var items []item
	for _, rel := range scanDirs {
		dir := filepath.Join(vaultRoot, rel)
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				if errors.Is(walkErr, fs.ErrNotExist) {
					return fs.SkipDir
				}
				return walkErr
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
				return nil
			}
			it, ok, err := scoreFile(vaultRoot, path, assertionTags, risingTopics, now)
			if err != nil {
				return nil // skip unreadable files, match Python's try/except
			}
			if !ok {
				return nil
			}
			items = append(items, it)
			return nil
		})
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: walk %s: %v", ErrSignal, dir, err)
		}
	}
	return items, nil
}

// scoreFile reads one note, applies eligibility filters, and produces
// an item. The second return is false when the note is filtered out.
func scoreFile(vaultRoot, abs string, assertionTags, risingTopics map[string]struct{}, now time.Time) (item, bool, error) {
	raw, err := os.ReadFile(abs)
	if err != nil {
		return item{}, false, err
	}
	fm, err := markdown.ParseFrontmatterYAML(string(raw))
	if err != nil {
		return item{}, false, nil
	}

	// Only include explicitly unconsumed notes (consumed: false).
	consumed, present := fm.Fields["consumed"]
	if !present {
		return item{}, false, nil
	}
	if b, ok := consumed.(bool); !ok || b {
		return item{}, false, nil
	}

	if asBool(fm.Fields["low-signal"]) {
		return item{}, false, nil
	}
	if asBool(fm.Fields["mood"]) {
		return item{}, false, nil
	}
	if coerceString(fm.Fields["content-type"]) == "anchor" {
		return item{}, false, nil
	}

	relPath, _ := filepath.Rel(vaultRoot, abs)
	relSlash := filepath.ToSlash(relPath)

	title := coerceString(fm.Fields["title"])
	if title == "" {
		stem := strings.TrimSuffix(filepath.Base(abs), ".md")
		title = strings.ReplaceAll(stem, "-", " ")
	}

	source := coerceString(fm.Fields["source"])
	if source != "" && !strings.HasPrefix(source, "http") {
		source = ""
	}

	tags := tagsFromField(fm.Fields["tags"])
	minutes := estimateMinutes(fm.Body)
	score := scoreNote(tags, fm.Fields["date"], assertionTags, risingTopics, now)
	dateStr := coerceString(fm.Fields["date"])

	ct := coerceString(fm.Fields["content-type"])
	if ct == "" {
		ct = inferContentType(source, fm.Body)
	}

	return item{
		score:       score,
		minutes:     minutes,
		title:       title,
		relPath:     relSlash,
		tags:        tags,
		source:      source,
		dateStr:     dateStr,
		contentType: ct,
	}, true, nil
}

// scoreNote applies the prototype's scoring weights: +3 per assertion
// domain tag match, +2 per rising topic match, +1 if note date is
// within the last 7 days.
func scoreNote(tags []string, dateField any, assertionTags, risingTopics map[string]struct{}, now time.Time) int {
	score := 0
	for _, t := range tags {
		t = normalizeTag(t)
		if _, ok := assertionTags[t]; ok {
			score += 3
		}
		if _, ok := risingTopics[t]; ok {
			score += 2
		}
	}
	if d, ok := parseDate(coerceString(dateField)); ok {
		if now.Sub(d).Hours()/24 <= 7 {
			score += 1
		}
	}
	return score
}

// parseDate accepts YYYY-MM-DD strings. Returns the date at UTC midnight.
func parseDate(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, false
	}
	return d, true
}

// estimateMinutes returns reading-time minutes for a note body. Mirrors
// the prototype: 150 wpm if body contains "## Transcript", else 250 wpm.
// Minimum is 1.
func estimateMinutes(body string) int {
	wpm := WPMArticle
	if strings.Contains(body, "## Transcript") {
		wpm = WPMTranscript
	}
	wc := len(strings.Fields(body))
	if wc == 0 {
		return 1
	}
	m := int((float64(wc) / float64(wpm)) + 0.5)
	if m < 1 {
		return 1
	}
	return m
}

// inferContentType matches the Python heuristic when frontmatter
// "content-type" is absent. Buckets are listen-watch, tool-reference,
// deep-read.
func inferContentType(source, body string) string {
	src := strings.ToLower(source)
	for _, h := range []string{"youtube.com", "youtu.be", "spotify.com", "podcasts.apple", "overcast.fm"} {
		if strings.Contains(src, h) {
			return "listen-watch"
		}
	}
	if strings.Contains(body, "## Transcript") {
		return "listen-watch"
	}
	prefix := strings.ToLower(body)
	if len(prefix) > 500 {
		prefix = prefix[:500]
	}
	for _, k := range []string{"api reference", "documentation", "manual", "getting started", "installation", "## usage", "```"} {
		if strings.Contains(prefix, k) {
			return "tool-reference"
		}
	}
	return "deep-read"
}

// renderQueue formats the markdown output for items. Sections mirror
// the Python prototype's order and limits.
func renderQueue(items []item, now time.Time) string {
	var b strings.Builder
	fmt.Fprintln(&b, "---")
	fmt.Fprintln(&b, "generated: true")
	fmt.Fprintf(&b, "updated: %s\n", now.Format("2006-01-02 15:04"))
	fmt.Fprintln(&b, "---")
	fmt.Fprintln(&b, "")
	fmt.Fprintf(&b, "# Signal Queue (%d unconsumed)\n", len(items))
	fmt.Fprintln(&b, "")

	priority := filterItems(items, func(it item) bool { return it.score >= 3 })
	deepReads := filterItems(items, func(it item) bool { return it.contentType == "deep-read" })
	listenWatch := filterItems(items, func(it item) bool { return it.contentType == "listen-watch" })
	toolRef := filterItems(items, func(it item) bool { return it.contentType == "tool-reference" })

	stale := []item{}
	for _, it := range items {
		if d, ok := parseDate(it.dateStr); ok {
			if int(now.Sub(d).Hours()/24) > StaleDays {
				stale = append(stale, it)
			}
		}
	}

	domainCounts := map[string]int{}
	for _, it := range items {
		for _, t := range it.tags {
			t = normalizeTag(t)
			if strings.Contains(t, "/") {
				domainCounts[t]++
			}
		}
	}

	if len(priority) > 0 {
		fmt.Fprintln(&b, "## Priority (assertion-aligned)")
		fmt.Fprintln(&b, "")
		for _, it := range truncate(priority, 10) {
			fmt.Fprintln(&b, formatItem(it))
		}
		fmt.Fprintln(&b, "")
	}

	if len(deepReads) > 0 {
		fmt.Fprintf(&b, "## Deep Reads — %d items\n", len(deepReads))
		fmt.Fprintln(&b, "")
		for _, it := range truncate(deepReads, 20) {
			fmt.Fprintln(&b, formatItem(it))
		}
		fmt.Fprintln(&b, "")
	}

	if len(listenWatch) > 0 {
		fmt.Fprintf(&b, "## Listen / Watch — %d items\n", len(listenWatch))
		fmt.Fprintln(&b, "")
		for _, it := range truncate(listenWatch, 20) {
			fmt.Fprintln(&b, formatItem(it))
		}
		fmt.Fprintln(&b, "")
	}

	if len(toolRef) > 0 {
		fmt.Fprintf(&b, "## Tools & Reference — %d items\n", len(toolRef))
		fmt.Fprintln(&b, "")
		for _, it := range truncate(toolRef, 20) {
			fmt.Fprintln(&b, formatItem(it))
		}
		fmt.Fprintln(&b, "")
	}

	if len(domainCounts) > 0 {
		fmt.Fprintln(&b, "## By Domain")
		fmt.Fprintln(&b, "")
		fmt.Fprintln(&b, "| Domain | Count |")
		fmt.Fprintln(&b, "|--------|-------|")
		// Sort by count desc, then domain asc for stability.
		domains := make([]string, 0, len(domainCounts))
		for d := range domainCounts {
			domains = append(domains, d)
		}
		sort.Slice(domains, func(i, j int) bool {
			if domainCounts[domains[i]] != domainCounts[domains[j]] {
				return domainCounts[domains[i]] > domainCounts[domains[j]]
			}
			return domains[i] < domains[j]
		})
		for _, d := range domains {
			fmt.Fprintf(&b, "| %s | %d |\n", d, domainCounts[d])
		}
		fmt.Fprintln(&b, "")
	}

	if len(stale) > 0 {
		fmt.Fprintf(&b, "## Stale Queue (>%d days unconsumed) — %d items\n", StaleDays, len(stale))
		fmt.Fprintln(&b, "")
		for _, it := range truncate(stale, 10) {
			fmt.Fprintln(&b, formatItem(it))
		}
		fmt.Fprintln(&b, "")
	}

	if len(items) == 0 {
		fmt.Fprintln(&b, "*Queue empty — nothing unconsumed.*")
		fmt.Fprintln(&b, "")
	}

	return b.String()
}

func truncate(in []item, n int) []item {
	if len(in) > n {
		return in[:n]
	}
	return in
}

func filterItems(in []item, keep func(item) bool) []item {
	out := make([]item, 0, len(in))
	for _, it := range in {
		if keep(it) {
			out = append(out, it)
		}
	}
	return out
}

// formatItem renders one queue line:
//
//	- [ ] [[<stem>|<title>]] (~<minutes> min) `t1` `t2` `t3` — [source](<url>)
func formatItem(it item) string {
	stem := strings.TrimSuffix(filepath.Base(it.relPath), ".md")
	var b strings.Builder
	fmt.Fprintf(&b, "- [ ] [[%s|%s]] (~%d min)", stem, it.title, it.minutes)
	tagBudget := 3
	for _, t := range it.tags {
		if tagBudget == 0 {
			break
		}
		if t == "" {
			continue
		}
		fmt.Fprintf(&b, " `%s`", t)
		tagBudget--
	}
	if it.source != "" {
		fmt.Fprintf(&b, " — [source](%s)", it.source)
	}
	return b.String()
}

// --- coercion helpers ---

// coerceString converts an any-typed frontmatter field to its string
// form. YAML scalars come back as string / int / float / bool / time.Time
// depending on the parser; mirror Python's str(...) coercion. Date scalars
// (unquoted "YYYY-MM-DD" in YAML) are normalized back to ISO date strings
// so callers can re-parse them with the prototype's expected format.
func coerceString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	case time.Time:
		return x.Format("2006-01-02")
	}
	return fmt.Sprint(v)
}

func asBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// tagsFromField converts a frontmatter "tags" field into []string,
// handling YAML list form, single-string form, and comma-separated.
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

func normalizeTag(t string) string {
	return strings.TrimPrefix(strings.TrimSpace(t), "#")
}
