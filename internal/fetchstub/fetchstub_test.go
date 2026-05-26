package fetchstub_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sea-noise/internal/crawl"
	"sea-noise/internal/fetchstub"
)

func TestProcessInbox_HydratesStub(t *testing.T) {
	vault := t.TempDir()
	writeNote(t, vault, "inbox/clip.md", `---
title: Some Article
source: https://example.com/post
tags: [clippings]
---

just a line
`)

	crawler := fakeCrawler{"https://example.com/post": "Crawled body text here."}
	res, err := fetchstub.ProcessInbox(context.Background(), crawler, fetchstub.InboxInput{
		VaultRoot: vault,
		Now:       fixedNow(),
	})
	if err != nil {
		t.Fatalf("ProcessInbox: %v", err)
	}
	if !sliceEqual(res.Hydrated, []string{"inbox/clip.md"}) {
		t.Errorf("Hydrated = %v", res.Hydrated)
	}
	got := readNote(t, vault, "inbox/clip.md")
	if !strings.HasSuffix(got, "\n\nCrawled body text here.\n") {
		t.Errorf("crawl content not appended:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(vault, "_meta/crawl-failures.json")); err == nil {
		t.Errorf("manifest written on success path")
	}
}

func TestProcessInbox_SkipsNonStub(t *testing.T) {
	vault := t.TempDir()
	// http source + clippings tag but body >= MinBodyChars.
	long := strings.Repeat("body line.\n", 50)
	writeNote(t, vault, "inbox/full.md", `---
title: Big
source: https://example.com/big
tags: [clippings]
---

`+long)
	// http source but no clippings tag.
	writeNote(t, vault, "inbox/no-tag.md", `---
source: https://example.com/no-tag
tags: [research]
---

x
`)
	// no http source.
	writeNote(t, vault, "inbox/no-source.md", `---
tags: [clippings]
---

x
`)

	crawler := fakeCrawler{}
	res, err := fetchstub.ProcessInbox(context.Background(), crawler, fetchstub.InboxInput{VaultRoot: vault, Now: fixedNow()})
	if err != nil {
		t.Fatalf("ProcessInbox: %v", err)
	}
	if len(res.Hydrated) != 0 {
		t.Errorf("Hydrated = %v, want empty", res.Hydrated)
	}
	if len(res.Skipped) != 3 {
		t.Errorf("Skipped = %v, want 3", res.Skipped)
	}
}

func TestProcessInbox_DedupesAgainstVault(t *testing.T) {
	vault := t.TempDir()
	writeNote(t, vault, "notes/references/existing.md", `---
title: Existing
source: https://example.com/dup?utm_source=newsletter
---

body
`)
	writeNote(t, vault, "inbox/clip.md", `---
title: Clip
source: https://example.com/dup
tags: [clippings]
---

stub
`)

	crawler := fakeCrawler{"https://example.com/dup": "SHOULD NOT BE CRAWLED"}
	res, err := fetchstub.ProcessInbox(context.Background(), crawler, fetchstub.InboxInput{VaultRoot: vault, Now: fixedNow()})
	if err != nil {
		t.Fatalf("ProcessInbox: %v", err)
	}
	if len(res.Deduped) != 1 {
		t.Fatalf("Deduped = %v, want 1", res.Deduped)
	}
	// File moved into _meta/discarded/.
	if _, err := os.Stat(filepath.Join(vault, "inbox/clip.md")); !os.IsNotExist(err) {
		t.Errorf("inbox clip should be gone, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(vault, "_meta/discarded/clip.md")); err != nil {
		t.Errorf("expected discard copy: %v", err)
	}
}

func TestProcessInbox_FailedCrawlLogsToManifest(t *testing.T) {
	vault := t.TempDir()
	writeNote(t, vault, "inbox/clip.md", `---
source: https://example.com/bad
tags: [clippings]
---

x
`)

	crawler := fakeCrawler{} // empty → returns nil pages, treated as failure
	now := fixedNow()
	res, err := fetchstub.ProcessInbox(context.Background(), crawler, fetchstub.InboxInput{VaultRoot: vault, Now: now})
	if err != nil {
		t.Fatalf("ProcessInbox: %v", err)
	}
	if len(res.Failed) != 1 {
		t.Errorf("Failed = %v", res.Failed)
	}
	entries := readManifest(t, vault)
	if len(entries) != 1 {
		t.Fatalf("manifest entries = %d, want 1", len(entries))
	}
	e := entries[0]
	if e.URL != "https://example.com/bad" {
		t.Errorf("manifest URL = %q", e.URL)
	}
	if e.RetryCount != 0 {
		t.Errorf("RetryCount = %d, want 0", e.RetryCount)
	}
	wantNext := now.Add(fetchstub.BackoffSchedule[0]).Format(time.RFC3339Nano)
	if e.NextRetryAfter != wantNext {
		t.Errorf("NextRetryAfter = %q, want %q", e.NextRetryAfter, wantNext)
	}
	if e.Abandoned {
		t.Errorf("entry marked Abandoned prematurely")
	}
}

func TestRetryFailures_DueEntrySucceeds(t *testing.T) {
	vault := t.TempDir()
	writeNote(t, vault, "inbox/clip.md", `---
source: https://example.com/bad
tags: [clippings]
---

x
`)
	now := fixedNow()
	due := now.Add(-time.Hour).Format(time.RFC3339Nano)
	writeManifest(t, vault, []fetchstub.ManifestEntry{{
		URL:             "https://example.com/bad",
		NotePath:        "inbox/clip.md",
		ErrorType:       "crawl_failed",
		RetryCount:      0,
		NextRetryAfter:  due,
		FirstFailure:    due,
		LastAttempt:     due,
	}})

	crawler := fakeCrawler{"https://example.com/bad": "Now it worked."}
	res, err := fetchstub.RetryFailures(context.Background(), crawler, fetchstub.RetryInput{VaultRoot: vault, Now: now})
	if err != nil {
		t.Fatalf("RetryFailures: %v", err)
	}
	if res.Retried != 1 {
		t.Errorf("Retried = %d, want 1", res.Retried)
	}
	if !sliceEqual(res.Succeeded, []string{"inbox/clip.md"}) {
		t.Errorf("Succeeded = %v", res.Succeeded)
	}
	if entries := readManifest(t, vault); len(entries) != 0 {
		t.Errorf("manifest = %v, want empty", entries)
	}
	if !strings.Contains(readNote(t, vault, "inbox/clip.md"), "Now it worked.") {
		t.Errorf("note not hydrated")
	}
}

func TestRetryFailures_SkipsNotYetDue(t *testing.T) {
	vault := t.TempDir()
	now := fixedNow()
	future := now.Add(time.Hour).Format(time.RFC3339Nano)
	writeManifest(t, vault, []fetchstub.ManifestEntry{{
		URL:            "https://example.com/wait",
		RetryCount:     0,
		NextRetryAfter: future,
	}})
	res, err := fetchstub.RetryFailures(context.Background(), failingCrawler{}, fetchstub.RetryInput{VaultRoot: vault, Now: now})
	if err != nil {
		t.Fatalf("RetryFailures: %v", err)
	}
	if res.Retried != 0 {
		t.Errorf("Retried = %d, want 0", res.Retried)
	}
	if res.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", res.Skipped)
	}
}

func TestRetryFailures_AdvancesBackoffOnFailure(t *testing.T) {
	vault := t.TempDir()
	now := fixedNow()
	due := now.Add(-time.Hour).Format(time.RFC3339Nano)
	writeManifest(t, vault, []fetchstub.ManifestEntry{{
		URL:            "https://example.com/bad",
		RetryCount:     1, // next retry uses BackoffSchedule[2]
		NextRetryAfter: due,
	}})

	res, err := fetchstub.RetryFailures(context.Background(), failingCrawler{}, fetchstub.RetryInput{VaultRoot: vault, Now: now})
	if err != nil {
		t.Fatalf("RetryFailures: %v", err)
	}
	if res.StillFailed != 1 {
		t.Errorf("StillFailed = %d, want 1", res.StillFailed)
	}
	entries := readManifest(t, vault)
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	e := entries[0]
	if e.RetryCount != 2 {
		t.Errorf("RetryCount = %d, want 2", e.RetryCount)
	}
	wantNext := now.Add(fetchstub.BackoffSchedule[2]).Format(time.RFC3339Nano)
	if e.NextRetryAfter != wantNext {
		t.Errorf("NextRetryAfter = %q, want %q", e.NextRetryAfter, wantNext)
	}
}

func TestRetryFailures_AbandonsAfterMaxRetries(t *testing.T) {
	vault := t.TempDir()
	now := fixedNow()
	due := now.Add(-time.Hour).Format(time.RFC3339Nano)
	writeManifest(t, vault, []fetchstub.ManifestEntry{{
		URL:            "https://example.com/bad",
		RetryCount:     fetchstub.MaxRetries - 1,
		NextRetryAfter: due,
	}})
	if _, err := fetchstub.RetryFailures(context.Background(), failingCrawler{}, fetchstub.RetryInput{VaultRoot: vault, Now: now}); err != nil {
		t.Fatalf("RetryFailures: %v", err)
	}
	entries := readManifest(t, vault)
	if len(entries) != 1 {
		t.Fatalf("entries = %d", len(entries))
	}
	if !entries[0].Abandoned {
		t.Errorf("entry should be Abandoned after exhausting backoff")
	}
}

func TestRetryFailures_SkipsAbandoned(t *testing.T) {
	vault := t.TempDir()
	writeManifest(t, vault, []fetchstub.ManifestEntry{{
		URL:       "https://example.com/done",
		Abandoned: true,
	}})
	res, err := fetchstub.RetryFailures(context.Background(), failingCrawler{}, fetchstub.RetryInput{VaultRoot: vault, Now: fixedNow()})
	if err != nil {
		t.Fatalf("RetryFailures: %v", err)
	}
	if res.Retried != 0 || res.Skipped != 1 {
		t.Errorf("Retried=%d Skipped=%d, want 0/1", res.Retried, res.Skipped)
	}
}

func TestNormalizeURL_StripsTrackingAndFragment(t *testing.T) {
	// Each `in` value should normalize equivalent to its `want` under
	// the prototype's tracking-param + fragment + trailing-slash rules.
	// Parity note: the prototype's regex consumes the leading `?` or
	// `&` of the tracking param, which means utm-as-first-param-with-
	// other-params produces `path&y=1`. Test inputs avoid that shape
	// since the canonical form would diverge.
	cases := []struct{ in, want string }{
		{"https://example.com/post?y=1&utm_source=x", "https://example.com/post?y=1"},
		{"https://example.com/post#section", "https://example.com/post"},
		{"https://example.com/post/", "https://example.com/post"},
		{"https://example.com/post?utm_medium=email", "https://example.com/post"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			vault := t.TempDir()
			// Provoke normalization through the dedup path: drop one note
			// with the canonical URL and one inbox stub with the raw URL.
			writeNote(t, vault, "notes/references/canonical.md", "---\nsource: "+c.want+"\n---\nbody\n")
			writeNote(t, vault, "inbox/raw.md", "---\nsource: "+c.in+"\ntags: [clippings]\n---\nstub\n")
			res, err := fetchstub.ProcessInbox(context.Background(), fakeCrawler{}, fetchstub.InboxInput{VaultRoot: vault, Now: fixedNow()})
			if err != nil {
				t.Fatalf("ProcessInbox: %v", err)
			}
			if len(res.Deduped) != 1 {
				t.Errorf("expected dedup match for %q vs %q, got %+v", c.in, c.want, res)
			}
		})
	}
}

// --- helpers ---

func writeNote(t *testing.T, vault, rel, content string) {
	t.Helper()
	p := filepath.Join(vault, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func readNote(t *testing.T, vault, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(vault, rel))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(b)
}

func writeManifest(t *testing.T, vault string, entries []fetchstub.ManifestEntry) {
	t.Helper()
	p := filepath.Join(vault, "_meta/crawl-failures.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(p, body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func readManifest(t *testing.T, vault string) []fetchstub.ManifestEntry {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(vault, "_meta/crawl-failures.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var out []fetchstub.ManifestEntry
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	return out
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
}

type fakeCrawler map[string]string

func (f fakeCrawler) Crawl(_ context.Context, url string, _, _ int) ([]crawl.Page, error) {
	if md, ok := f[url]; ok && md != "" {
		return []crawl.Page{{URL: url, Markdown: md}}, nil
	}
	return nil, nil
}

type failingCrawler struct{}

func (failingCrawler) Crawl(_ context.Context, _ string, _, _ int) ([]crawl.Page, error) {
	return nil, errors.New("nope")
}
