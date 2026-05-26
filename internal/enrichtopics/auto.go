package enrichtopics

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"sea-noise/internal/ai/skill"
)

// DefaultAutoThreshold mirrors references/scripts/enrich-topics.py's
// THRESHOLD (3 mentions before a topic becomes eligible for auto
// enrichment).
const DefaultAutoThreshold = 3

// DefaultMaxAutoPerRun caps how many topics get enriched in a single
// `enrich-topics --auto` invocation. Mirrors the prototype's posture
// of "don't fire ten claude calls in a row by accident."
const DefaultMaxAutoPerRun = 5

// TopicFreqRelPath is the canonical vault-relative path the prototype
// reads. The inbox-process skill writes to this same path.
const TopicFreqRelPath = "_meta/topic-frequencies.md"

// TopicEntry is one row of _meta/topic-frequencies.md. Only the topic
// name and count are load-bearing for the auto flow; FirstSeen and
// LastSeen are surfaced for downstream callers but are optional in
// the file (older 4-column tables omit FirstSeen).
type TopicEntry struct {
	Topic     string
	Count     int
	FirstSeen string
	LastSeen  string
	Trend     string
}

// AutoInput configures EnrichTopicsAuto.
type AutoInput struct {
	VaultRoot string
	Threshold int       // 0 → DefaultAutoThreshold
	MaxPerRun int       // 0 → DefaultMaxAutoPerRun
	Today     time.Time // 0 → time.Now()
}

// AutoResult reports per-run outcomes.
type AutoResult struct {
	Considered int
	Enriched   []Result
	Skipped    []string // topic names that didn't qualify (already enriched, below threshold)
	Failed     []string // topic names where EnrichTopic returned an error
}

// EnrichTopicsAuto reads `_meta/topic-frequencies.md`, filters topics
// to those at or above the threshold and not already in the
// enrichment log, caps the survivors at MaxPerRun, and runs
// EnrichTopic for each.
//
// Per-topic errors are recorded in Failed without aborting the run.
// IO errors against the topic-frequencies file or enrichment log
// surface immediately.
func EnrichTopicsAuto(ctx context.Context, exec *skill.Executor, crawler Crawler, in AutoInput) (AutoResult, error) {
	var result AutoResult
	threshold := in.Threshold
	if threshold <= 0 {
		threshold = DefaultAutoThreshold
	}
	cap := in.MaxPerRun
	if cap <= 0 {
		cap = DefaultMaxAutoPerRun
	}

	topics, err := ReadTopicFrequencies(in.VaultRoot)
	if err != nil {
		return result, err
	}
	enriched, err := ReadEnrichedTopics(in.VaultRoot)
	if err != nil {
		return result, err
	}

	for _, t := range topics {
		result.Considered++
		if enriched[t.Topic] {
			result.Skipped = append(result.Skipped, t.Topic)
			continue
		}
		if t.Count < threshold {
			result.Skipped = append(result.Skipped, t.Topic)
			continue
		}
		if len(result.Enriched) >= cap {
			result.Skipped = append(result.Skipped, t.Topic)
			continue
		}
		res, err := EnrichTopic(ctx, exec, crawler, Input{
			VaultRoot: in.VaultRoot,
			Topic:     t.Topic,
			Today:     in.Today,
		})
		if err != nil {
			result.Failed = append(result.Failed, t.Topic)
			continue
		}
		result.Enriched = append(result.Enriched, res)
	}
	return result, nil
}

// ReadTopicFrequencies parses `_meta/topic-frequencies.md` and
// returns one TopicEntry per data row. Accepts both the 4-column
// shape `| tag | count | last_seen | trend |` (`internal/signal/`'s
// canonical layout, also what the inbox-process skill writes) and
// the prototype's older 5-column shape `| topic | first_seen |
// last_seen | count | trend |` — the parser locates the count by
// scanning columns for the first integer, so the column order is not
// load-bearing as long as the topic is in column 1.
//
// Header rows (containing the literal word "topic" or "tag" without
// a numeric column) and separator rows (containing "---") are
// skipped. A missing file is not an error.
func ReadTopicFrequencies(vaultRoot string) ([]TopicEntry, error) {
	path := filepath.Join(vaultRoot, TopicFreqRelPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("enrichtopics: read %s: %w", path, err)
	}
	var out []TopicEntry
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		parts := splitPipe(line)
		if len(parts) == 0 {
			continue
		}
		if strings.Contains(line, "---") {
			continue
		}
		head := strings.ToLower(strings.TrimSpace(parts[0]))
		if head == "topic" || head == "tag" {
			continue
		}
		entry := TopicEntry{Topic: strings.TrimPrefix(strings.TrimSpace(parts[0]), "#")}
		// Locate the count column: first part that parses as int and
		// isn't column 0. Mirrors the prototype's tolerance for column
		// reordering across vaults.
		for i := 1; i < len(parts); i++ {
			if n, err := strconv.Atoi(strings.TrimSpace(parts[i])); err == nil {
				entry.Count = n
				break
			}
		}
		// Best-effort tagging of date-shaped columns. Not load-bearing
		// for the auto flow but exposed for downstream callers.
		var dates []string
		for i := 1; i < len(parts); i++ {
			p := strings.TrimSpace(parts[i])
			if isDateLike(p) {
				dates = append(dates, p)
			}
		}
		if len(dates) >= 1 {
			entry.LastSeen = dates[len(dates)-1]
		}
		if len(dates) >= 2 {
			entry.FirstSeen = dates[0]
		}
		// Trend = last non-numeric, non-date column past column 0.
		for i := len(parts) - 1; i >= 1; i-- {
			p := strings.TrimSpace(parts[i])
			if p == "" {
				continue
			}
			if _, err := strconv.Atoi(p); err == nil {
				continue
			}
			if isDateLike(p) {
				continue
			}
			entry.Trend = p
			break
		}
		if entry.Topic != "" {
			out = append(out, entry)
		}
	}
	return out, nil
}

// ReadEnrichedTopics returns the set of topic names already present
// in `_meta/enrichment-log.md`. Mirrors the prototype's
// get_enriched_topics regex `^- (.+?) —`. Missing log is not an error.
func ReadEnrichedTopics(vaultRoot string) (map[string]bool, error) {
	path := filepath.Join(vaultRoot, logRelPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("enrichtopics: read %s: %w", path, err)
	}
	out := map[string]bool{}
	re := regexp.MustCompile(`(?m)^- (.+?) —`)
	for _, m := range re.FindAllStringSubmatch(string(raw), -1) {
		out[m[1]] = true
	}
	return out, nil
}

func splitPipe(line string) []string {
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}

var dateLike = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func isDateLike(s string) bool {
	return dateLike.MatchString(s)
}
