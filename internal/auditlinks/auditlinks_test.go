package auditlinks

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestBuildReport_EmptyVault(t *testing.T) {
	dir := t.TempDir()
	report, err := BuildReport(dir, mustParseDate(t, "2026-05-21"))
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	want := "# Broken Links Audit — 2026-05-21\n\nNo broken links found.\n"
	if report != want {
		t.Errorf("report =\n%q\nwant\n%q", report, want)
	}
}

func TestBuildReport_ResolvesValidLinks(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, dir, "alpha.md", "# Alpha\n\nsee [[Beta]]\n")
	mustWriteFile(t, dir, "Beta.md", "# Beta\n")

	report, err := BuildReport(dir, mustParseDate(t, "2026-05-21"))
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	want := "# Broken Links Audit — 2026-05-21\n\nNo broken links found.\n"
	if report != want {
		t.Errorf("report =\n%q\nwant\n%q (link should resolve case-insensitively)", report, want)
	}
}

func TestBuildReport_DetectsBrokenLinks(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, dir, "alpha.md", "see [[Missing]] and [[Also-Missing]]\n")
	mustWriteFile(t, dir, "sub/beta.md", "[[Missing]] again\n")

	report, err := BuildReport(dir, mustParseDate(t, "2026-05-21"))
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}

	want := `# Broken Links Audit — 2026-05-21

Found **3** broken wikilinks:


### alpha.md
- [[Also-Missing]]
- [[Missing]]

### sub/beta.md
- [[Missing]]
`
	if report != want {
		t.Errorf("report =\n%q\nwant\n%q", report, want)
	}
}

func TestBuildReport_ParityAgainstFixture(t *testing.T) {
	root := repoRoot(t)
	fixture := filepath.Join(root, "testdata", "fixture-vault")
	goldenPath := filepath.Join(root, "testdata", "expected", "audit-links", "report.md")

	got, err := BuildReport(fixture, mustParseDate(t, "2026-05-21"))
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	if got != string(want) {
		// Find first mismatch for a useful failure message.
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

// === helpers ===

func mustWriteFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustParseDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
