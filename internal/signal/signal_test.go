package signal_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"sea-noise/internal/signal"
)

// writeVault materialises a synthetic vault inside t.TempDir() from the
// given rel → content map and returns the root.
func writeVault(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", abs, err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", abs, err)
		}
	}
	return root
}

func TestBuild_EmptyVault(t *testing.T) {
	root := writeVault(t, map[string]string{})
	body, n, err := signal.Build(root, time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if n != 0 {
		t.Errorf("count = %d, want 0", n)
	}
	if !strings.Contains(body, "Queue empty") {
		t.Errorf("empty-queue marker missing in body:\n%s", body)
	}
}

func TestBuild_EligibilityFilters(t *testing.T) {
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	root := writeVault(t, map[string]string{
		// Excluded — no `consumed:` field at all.
		"notes/references/no-consumed.md": "---\ntitle: NoFlag\n---\nbody\n",
		// Excluded — consumed: true.
		"notes/references/already.md": "---\ntitle: Already\nconsumed: true\n---\nbody\n",
		// Excluded — low-signal: true.
		"notes/references/low.md": "---\ntitle: Low\nconsumed: false\nlow-signal: true\n---\nbody\n",
		// Excluded — mood anchor.
		"notes/references/anchor.md": "---\ntitle: Anchor\nconsumed: false\ncontent-type: anchor\n---\nbody\n",
		// Excluded — mood: true.
		"notes/references/mood.md": "---\ntitle: Moody\nconsumed: false\nmood: true\n---\nbody\n",
		// Included.
		"notes/references/keep.md": "---\ntitle: Keep\nconsumed: false\ntags: [reference]\n---\nbody\n",
	})

	body, n, err := signal.Build(root, now)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if n != 1 {
		t.Errorf("count = %d, want 1; body:\n%s", n, body)
	}
	if !strings.Contains(body, "[[keep|Keep]]") {
		t.Errorf("expected keep.md present; body:\n%s", body)
	}
	for _, banned := range []string{"NoFlag", "Already", "Low", "Anchor", "Moody"} {
		if strings.Contains(body, banned) {
			t.Errorf("filtered note %q leaked into output:\n%s", banned, body)
		}
	}
}

func TestBuild_PriorityScoring(t *testing.T) {
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	root := writeVault(t, map[string]string{
		// Assertion that contributes tier-2 tag tech/ai.
		"notes/assertions/explore-ai.md": "---\ntitle: Explore AI\nstatus: active\ntags: [assertion, tech/ai]\n---\nWhy\n",
		// Inactive assertion — should be ignored.
		"notes/assertions/inactive.md": "---\ntitle: Old\nstatus: paused\ntags: [tech/inactive]\n---\nWhy\n",

		// Scored +3 (matches assertion tag) +1 (recency, date == now): 4.
		"notes/references/match.md": "---\ntitle: Match\nconsumed: false\ndate: 2026-05-23\ntags: [reference, tech/ai]\n---\nbody\n",
		// Scored 0 (irrelevant tag, old date).
		"notes/references/noise.md": "---\ntitle: Noise\nconsumed: false\ndate: 2024-01-01\ntags: [reference, life/unrelated]\n---\nbody\n",
	})

	body, n, err := signal.Build(root, now)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if n != 2 {
		t.Errorf("count = %d, want 2", n)
	}
	if !strings.Contains(body, "## Priority (assertion-aligned)") {
		t.Errorf("Priority section missing; body:\n%s", body)
	}
	// Match must appear in priority; Noise must not.
	priIdx := strings.Index(body, "## Priority")
	deepIdx := strings.Index(body, "## Deep Reads")
	if priIdx < 0 || deepIdx < 0 {
		t.Fatalf("section markers missing; body:\n%s", body)
	}
	prioritySection := body[priIdx:deepIdx]
	if !strings.Contains(prioritySection, "Match") {
		t.Errorf("expected Match in priority section:\n%s", prioritySection)
	}
	if strings.Contains(prioritySection, "Noise") {
		t.Errorf("Noise (score 0) leaked into priority section:\n%s", prioritySection)
	}
}

func TestBuild_BucketingByContentType(t *testing.T) {
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	root := writeVault(t, map[string]string{
		// listen-watch via source URL.
		"notes/references/yt.md": "---\ntitle: YT\nconsumed: false\nsource: https://www.youtube.com/watch?v=x\n---\nbody\n",
		// listen-watch via ## Transcript section.
		"notes/references/podcast.md": "---\ntitle: Pod\nconsumed: false\nsource: https://example.com\n---\n## Transcript\nlong content\n",
		// tool-reference via keyword.
		"notes/references/manual.md": "---\ntitle: Manual\nconsumed: false\nsource: https://docs.example.com\n---\n# Title\n\nGetting Started with the API...\n",
		// deep-read fallback.
		"notes/references/essay.md": "---\ntitle: Essay\nconsumed: false\nsource: https://blog.example.com\n---\nA long essay about ideas.\n",
	})

	body, _, err := signal.Build(root, now)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	mustContain := map[string]string{
		"## Deep Reads":        "Essay",
		"## Listen / Watch":    "YT",
		"## Tools & Reference": "Manual",
	}
	for section, item := range mustContain {
		idx := strings.Index(body, section)
		if idx < 0 {
			t.Errorf("section %q missing; body:\n%s", section, body)
			continue
		}
		// Find the slice up to the next section header.
		rest := body[idx+len(section):]
		end := strings.Index(rest, "\n## ")
		if end < 0 {
			end = len(rest)
		}
		if !strings.Contains(rest[:end], item) {
			t.Errorf("item %q expected in %q section; section content:\n%s", item, section, rest[:end])
		}
	}

	// podcast.md has ## Transcript so should be listen-watch.
	if !strings.Contains(body, "Pod") {
		t.Errorf("podcast note missing entirely; body:\n%s", body)
	}
}

func TestBuild_StaleBucket(t *testing.T) {
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	root := writeVault(t, map[string]string{
		// 30 days old → stale.
		"notes/references/old.md": "---\ntitle: Old\nconsumed: false\ndate: 2026-04-23\n---\nbody\n",
		// 5 days old → not stale.
		"notes/references/fresh.md": "---\ntitle: Fresh\nconsumed: false\ndate: 2026-05-18\n---\nbody\n",
	})

	body, _, err := signal.Build(root, now)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	staleIdx := strings.Index(body, "## Stale Queue")
	if staleIdx < 0 {
		t.Fatalf("expected stale section; body:\n%s", body)
	}
	staleSection := body[staleIdx:]
	if !strings.Contains(staleSection, "Old") {
		t.Errorf("Old (30 days) expected in stale section:\n%s", staleSection)
	}
	if strings.Contains(staleSection, "Fresh") {
		t.Errorf("Fresh (5 days) leaked into stale section:\n%s", staleSection)
	}
}

func TestBuild_RisingTopicsFromMetaFile(t *testing.T) {
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	root := writeVault(t, map[string]string{
		"_meta/topic-frequencies.md": `# Topics

| tag | count | last_seen | trend |
|-----|-------|-----------|-------|
| tech/ai | 12 | 2026-05-22 | rising |
| life/cooking | 3 | 2026-05-22 | stable |
`,
		// +2 from rising topic match.
		"notes/references/risen.md": "---\ntitle: Risen\nconsumed: false\ntags: [reference, tech/ai]\n---\nbody\n",
	})

	body, _, err := signal.Build(root, now)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// tech/ai is rising → score +2 (not assertion). Score < 3 so not
	// priority but should still be in Deep Reads.
	if !strings.Contains(body, "[[risen|Risen]]") {
		t.Errorf("risen.md missing from output:\n%s", body)
	}
}

func TestBuildAndWrite_Roundtrip(t *testing.T) {
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	root := writeVault(t, map[string]string{
		"notes/references/x.md": "---\ntitle: X\nconsumed: false\n---\nbody\n",
	})

	count, rel, err := signal.BuildAndWrite(root, now)
	if err != nil {
		t.Fatalf("BuildAndWrite: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
	if rel != "signal/queue.md" {
		t.Errorf("rel = %q, want signal/queue.md", rel)
	}
	got, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(got), "[[x|X]]") {
		t.Errorf("expected x.md item in output:\n%s", string(got))
	}
}

func TestBuild_MissingScanDirs(t *testing.T) {
	// Vault with only assertions; no notes/references or notes/fleeting.
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	root := writeVault(t, map[string]string{
		"notes/assertions/a.md": "---\nstatus: active\ntags: [tech/ai]\n---\n",
	})
	body, n, err := signal.Build(root, now)
	if err != nil {
		t.Fatalf("Build (missing scan dirs should not error): %v", err)
	}
	if n != 0 {
		t.Errorf("count = %d, want 0 (no scannable dirs)", n)
	}
	if !strings.Contains(body, "Queue empty") {
		t.Errorf("expected empty-queue marker:\n%s", body)
	}
}

func TestBuild_ParityAgainstFixture(t *testing.T) {
	root := repoRoot(t)
	fixture := filepath.Join(root, "testdata", "fixture-vault")
	goldenPath := filepath.Join(root, "testdata", "expected", "signal", "queue.md")

	today, err := time.Parse("2006-01-02", "2026-05-21")
	if err != nil {
		t.Fatal(err)
	}

	got, _, err := signal.Build(fixture, today)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	if got != string(want) {
		min := len(got)
		if len(want) < min {
			min = len(want)
		}
		for i := 0; i < min; i++ {
			if got[i] != want[i] {
				start := i - 40
				if start < 0 {
					start = 0
				}
				end := i + 40
				if end > min {
					end = min
				}
				t.Fatalf("first mismatch at byte %d:\n  got:  %q\n  want: %q", i, got[start:end], string(want[start:end]))
			}
		}
		t.Fatalf("length mismatch: got %d, want %d", len(got), len(want))
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func TestErrSignal_Is(t *testing.T) {
	// BuildAndWrite into a path that can't be created: pass a vaultRoot
	// that points at a file, not a directory. mkdir will fail.
	f, err := os.CreateTemp(t.TempDir(), "not-a-dir-*")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	f.Close()

	_, _, err = signal.BuildAndWrite(f.Name(), time.Now())
	if err == nil {
		t.Fatal("expected error from BuildAndWrite against a file path")
	}
	if !errors.Is(err, signal.ErrSignal) {
		t.Errorf("err = %v, want ErrSignal", err)
	}
}
