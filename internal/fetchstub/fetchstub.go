// Package fetchstub ports references/scripts/fetch-stub-content.py.
//
// Two surfaces:
//
//   - HydrateStubs walks inbox/*.md, discards notes whose source URL
//     already appears in the vault, then hydrates remaining stubs
//     (notes with an http source, a `clippings` tag, and a body under
//     MinBodyChars characters) by calling the configured crawler and
//     appending the markdown to the note. Failed crawls land in a
//     persistent manifest with an exponential backoff schedule.
//   - RetryFailures reads the manifest, retries any entry whose
//     next-retry-after timestamp has passed, hydrates the matching
//     note on success, and advances the backoff on failure.
//
// The prototype's "platform_apis" pre-fetch path and the post-pass
// crawl upgrade (Substack-style content widening) are intentionally
// out of scope — they depend on a prototype-local module that is not
// in references/. .tempor/KNOWN-DELTA.md captures the gap.
package fetchstub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"sea-noise/internal/crawl"
	"sea-noise/internal/markdown"
)

// MinBodyChars matches the prototype's MIN_BODY_CHARS threshold for
// classifying a note as a hydration-needing stub.
const MinBodyChars = 200

// MaxCrawlBytes matches the prototype's 80_000-char truncation before
// appending crawl results to a note.
const MaxCrawlBytes = 80_000

// ManifestRelPath is the vault-relative path to the crawl-failure
// manifest. The prototype writes it under _meta/.
const ManifestRelPath = "_meta/crawl-failures.json"

// DiscardRelDir is the vault-relative folder duplicate inbox notes
// get moved into.
const DiscardRelDir = "_meta/discarded"

// ClippingsTag is the frontmatter tag the prototype keys stub
// detection on. Notes without it are left alone.
const ClippingsTag = "clippings"

// DefaultDedupDirs is the vault-relative set scanned for already-
// captured source URLs when deduplicating new inbox arrivals.
var DefaultDedupDirs = []string{
	"notes/fleeting",
	"notes/references",
	"notes/ideas",
}

// DefaultRetrySearchDirs is the vault-relative set scanned in retry
// mode to find the note whose source URL matches a manifest entry.
var DefaultRetrySearchDirs = []string{
	"inbox",
	"notes/fleeting",
	"notes/references",
}

// BackoffSchedule is the per-retry-count delay before the next
// attempt. Matches the prototype's 6h → 24h → 72h → 168h schedule.
// MaxRetries is len(BackoffSchedule); a manifest entry exceeds the
// schedule by being marked abandoned.
var BackoffSchedule = []time.Duration{
	6 * time.Hour,
	24 * time.Hour,
	72 * time.Hour,
	168 * time.Hour,
}

// MaxRetries equals len(BackoffSchedule).
var MaxRetries = len(BackoffSchedule)

// HydrateInput configures a HydrateStubs run.
type HydrateInput struct {
	VaultRoot string
	Now       time.Time // injected clock; defaults to time.Now().UTC()
}

// HydrateResult reports counts. All paths are vault-relative.
type HydrateResult struct {
	Hydrated []string
	Deduped  []string
	Failed   []string
	Skipped  []string // notes scanned that weren't stubs
}

// RetryInput configures a RetryFailures run.
type RetryInput struct {
	VaultRoot   string
	Now         time.Time
	SearchDirs  []string // override DefaultRetrySearchDirs
}

// RetryResult reports counts.
type RetryResult struct {
	Retried    int
	Succeeded  []string // vault-relative paths re-hydrated
	StillFailed int
	Skipped    int // abandoned or not-yet-due
}

// ManifestEntry is one row in _meta/crawl-failures.json. JSON
// representation mirrors the prototype's field shape so a manifest
// written by either runtime can be read by the other.
type ManifestEntry struct {
	URL             string `json:"url"`
	NotePath        string `json:"note_path"`
	ErrorType       string `json:"error_type"`
	RetryCount      int    `json:"retry_count"`
	FirstFailure    string `json:"first_failure"`
	LastAttempt     string `json:"last_attempt"`
	LastError       string `json:"last_error,omitempty"`
	NextRetryAfter  string `json:"next_retry_after,omitempty"`
	Abandoned       bool   `json:"abandoned"`
}

// HydrateStubs walks <vault>/inbox/, finds notes that are URL stubs
// (clippings-tagged with body shorter than MIN_BODY_CHARS), crawls
// the URL to fetch full page content, and replaces the stub body
// with the hydrated content. Also dedupes against existing vault
// URLs to discard already-clipped pages.
//
// Used as the first step of the inbox pipeline so that downstream
// classification (Claude via process-inbox) sees full content rather
// than just URLs.
//
// Ports the default mode of references/scripts/fetch-stub-content.py.
func HydrateStubs(ctx context.Context, crawler crawl.Crawler, in HydrateInput) (HydrateResult, error) {
	var result HydrateResult
	now := nowOrDefault(in.Now)
	inboxDir := filepath.Join(in.VaultRoot, "inbox")
	entries, err := os.ReadDir(inboxDir)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("fetchstub: read inbox: %w", err)
	}

	existingURLs, err := collectExistingURLs(in.VaultRoot, DefaultDedupDirs)
	if err != nil {
		return result, err
	}

	paths := mdPaths(entries, inboxDir)
	for _, p := range paths {
		rel, _ := filepath.Rel(in.VaultRoot, p)
		raw, err := os.ReadFile(p)
		if err != nil {
			return result, fmt.Errorf("fetchstub: read %s: %w", p, err)
		}
		content := string(raw)
		fm, err := markdown.ParseFrontmatterYAML(content)
		if err != nil {
			result.Skipped = append(result.Skipped, rel)
			continue
		}
		source, _ := fm.Fields["source"].(string)
		if strings.HasPrefix(source, "http") {
			n := normalizeURL(source)
			if existingURLs[n] {
				newPath, err := moveToDiscard(in.VaultRoot, p)
				if err != nil {
					return result, err
				}
				newRel, _ := filepath.Rel(in.VaultRoot, newPath)
				result.Deduped = append(result.Deduped, newRel)
				continue
			}
			existingURLs[n] = true
		}

		if !isStub(fm, content) {
			result.Skipped = append(result.Skipped, rel)
			continue
		}

		markdownText, err := crawlOne(ctx, crawler, source)
		if err != nil || markdownText == "" {
			if err := appendFailure(in.VaultRoot, ManifestEntry{
				URL:       source,
				NotePath:  rel,
				ErrorType: "crawl_failed",
			}, now); err != nil {
				return result, err
			}
			result.Failed = append(result.Failed, rel)
			continue
		}

		if len(markdownText) > MaxCrawlBytes {
			markdownText = markdownText[:MaxCrawlBytes]
		}
		newContent := strings.TrimRight(content, " \t\n\v\f\r") + "\n\n" + markdownText + "\n"
		if err := os.WriteFile(p, []byte(newContent), 0o644); err != nil {
			return result, fmt.Errorf("fetchstub: write %s: %w", p, err)
		}
		result.Hydrated = append(result.Hydrated, rel)
	}
	return result, nil
}

// RetryFailures is the --retry-mode entry point.
func RetryFailures(ctx context.Context, crawler crawl.Crawler, in RetryInput) (RetryResult, error) {
	var result RetryResult
	now := nowOrDefault(in.Now)
	manifest, err := loadManifest(in.VaultRoot)
	if err != nil {
		return result, err
	}
	if len(manifest) == 0 {
		return result, nil
	}
	dirs := in.SearchDirs
	if len(dirs) == 0 {
		dirs = DefaultRetrySearchDirs
	}

	remaining := make([]ManifestEntry, 0, len(manifest))
	for _, e := range manifest {
		if e.Abandoned {
			result.Skipped++
			remaining = append(remaining, e)
			continue
		}
		if e.NextRetryAfter != "" {
			if t, err := time.Parse(time.RFC3339Nano, e.NextRetryAfter); err == nil && now.Before(t) {
				result.Skipped++
				remaining = append(remaining, e)
				continue
			}
		}

		result.Retried++
		markdownText, err := crawlOne(ctx, crawler, e.URL)
		if err != nil || markdownText == "" {
			updated := advanceBackoff(e, "retry_failed", now)
			remaining = append(remaining, updated)
			result.StillFailed++
			continue
		}
		if len(markdownText) > MaxCrawlBytes {
			markdownText = markdownText[:MaxCrawlBytes]
		}
		notePath, err := findNoteBySourceURL(in.VaultRoot, dirs, e.URL)
		if err != nil {
			return result, err
		}
		if notePath == "" {
			// Content fetched but note vanished — drop manifest entry anyway.
			continue
		}
		raw, err := os.ReadFile(notePath)
		if err != nil {
			return result, fmt.Errorf("fetchstub: read %s: %w", notePath, err)
		}
		newContent := strings.TrimRight(string(raw), " \t\n\v\f\r") + "\n\n" + markdownText + "\n"
		if err := os.WriteFile(notePath, []byte(newContent), 0o644); err != nil {
			return result, fmt.Errorf("fetchstub: write %s: %w", notePath, err)
		}
		rel, _ := filepath.Rel(in.VaultRoot, notePath)
		result.Succeeded = append(result.Succeeded, rel)
	}

	if err := saveManifest(in.VaultRoot, remaining); err != nil {
		return result, err
	}
	return result, nil
}

// --- crawl ---

func crawlOne(ctx context.Context, crawler crawl.Crawler, url string) (string, error) {
	pages, err := crawler.Crawl(ctx, url, 1, 0)
	if err != nil {
		return "", err
	}
	for _, p := range pages {
		if p.Markdown != "" {
			return p.Markdown, nil
		}
	}
	return "", nil
}

// --- dedup ---

var trackingParam = regexp.MustCompile(`[?&](utm_\w+|ref|fbclid|gclid|mc_[a-z]+)=[^&]*`)

func normalizeURL(url string) string {
	url = strings.TrimSpace(url)
	url = trackingParam.ReplaceAllString(url, "")
	url = strings.ReplaceAll(url, "?&", "?")
	url = strings.TrimSuffix(url, "?")
	if i := strings.Index(url, "#"); i >= 0 {
		url = url[:i]
	}
	url = strings.TrimRight(url, "/")
	return url
}

func collectExistingURLs(vaultRoot string, dirs []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, rel := range dirs {
		dir := filepath.Join(vaultRoot, rel)
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return filepath.SkipDir
				}
				return err
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
			source, _ := fm.Fields["source"].(string)
			if strings.HasPrefix(source, "http") {
				out[normalizeURL(source)] = true
			}
			return nil
		})
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("fetchstub: walk %s: %w", dir, err)
		}
	}
	return out, nil
}

// moveToDiscard moves notePath into vaultRoot/_meta/discarded/,
// appending "-dup" to the stem if the destination already exists.
func moveToDiscard(vaultRoot, notePath string) (string, error) {
	dst := filepath.Join(vaultRoot, DiscardRelDir)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return "", fmt.Errorf("fetchstub: mkdir discard: %w", err)
	}
	dstPath := filepath.Join(dst, filepath.Base(notePath))
	if _, err := os.Stat(dstPath); err == nil {
		stem := strings.TrimSuffix(filepath.Base(notePath), ".md")
		dstPath = filepath.Join(dst, stem+"-dup.md")
	}
	if err := os.Rename(notePath, dstPath); err != nil {
		return "", fmt.Errorf("fetchstub: rename to discard: %w", err)
	}
	return dstPath, nil
}

// --- stub detection ---

func isStub(fm markdown.YAMLFrontmatter, fullContent string) bool {
	source, _ := fm.Fields["source"].(string)
	if !strings.HasPrefix(source, "http") {
		return false
	}
	if !hasTag(fm.StringTags(), ClippingsTag) {
		return false
	}
	body := strings.TrimSpace(fm.Body)
	return len(body) < MinBodyChars
}

func hasTag(tags []string, want string) bool {
	for _, t := range tags {
		if strings.EqualFold(strings.TrimPrefix(t, "#"), want) {
			return true
		}
	}
	return false
}

// --- retry mode ---

func findNoteBySourceURL(vaultRoot string, dirs []string, url string) (string, error) {
	normalized := normalizeURL(url)
	for _, rel := range dirs {
		dir := filepath.Join(vaultRoot, rel)
		var match string
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return filepath.SkipDir
				}
				return err
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
			source, _ := fm.Fields["source"].(string)
			if strings.HasPrefix(source, "http") && normalizeURL(source) == normalized {
				match = path
				return filepath.SkipDir
			}
			return nil
		})
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("fetchstub: walk %s: %w", dir, err)
		}
		if match != "" {
			return match, nil
		}
	}
	return "", nil
}

// --- manifest ---

func loadManifest(vaultRoot string) ([]ManifestEntry, error) {
	path := filepath.Join(vaultRoot, ManifestRelPath)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fetchstub: read manifest: %w", err)
	}
	var entries []ManifestEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		// Match the prototype's posture: malformed manifest is silently
		// treated as empty so a botched edit doesn't strand the workflow.
		return nil, nil
	}
	return entries, nil
}

func saveManifest(vaultRoot string, entries []ManifestEntry) error {
	path := filepath.Join(vaultRoot, ManifestRelPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("fetchstub: mkdir manifest: %w", err)
	}
	body, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("fetchstub: encode manifest: %w", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("fetchstub: write manifest: %w", err)
	}
	return nil
}

func appendFailure(vaultRoot string, e ManifestEntry, now time.Time) error {
	entries, err := loadManifest(vaultRoot)
	if err != nil {
		return err
	}
	for i, existing := range entries {
		if existing.URL == e.URL {
			entries[i] = advanceBackoff(existing, e.ErrorType, now)
			return saveManifest(vaultRoot, entries)
		}
	}
	e.RetryCount = 0
	stamp := now.Format(time.RFC3339Nano)
	e.FirstFailure = stamp
	e.LastAttempt = stamp
	e.LastError = e.ErrorType
	e.NextRetryAfter = now.Add(BackoffSchedule[0]).Format(time.RFC3339Nano)
	entries = append(entries, e)
	return saveManifest(vaultRoot, entries)
}

func advanceBackoff(e ManifestEntry, errType string, now time.Time) ManifestEntry {
	e.RetryCount++
	e.LastError = errType
	e.LastAttempt = now.Format(time.RFC3339Nano)
	if e.RetryCount < MaxRetries {
		e.NextRetryAfter = now.Add(BackoffSchedule[e.RetryCount]).Format(time.RFC3339Nano)
	} else {
		e.NextRetryAfter = ""
		e.Abandoned = true
	}
	return e
}

// --- file helpers ---

func mdPaths(entries []os.DirEntry, dir string) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	return out
}

func nowOrDefault(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t
}
