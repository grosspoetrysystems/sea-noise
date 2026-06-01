package parity_test

import (
	"os"
	"path/filepath"
	"testing"

	"sea-noise/internal/parity"
)

func TestDiff_EquivalentTreesYieldNoDifferences(t *testing.T) {
	want := t.TempDir()
	got := t.TempDir()
	tree := map[string]string{
		"a.md":     "---\ntitle: A\ntags: [x, y]\n---\nbody A [[B]]\n",
		"sub/b.md": "---\ntitle: B\n---\nbody B\n",
	}
	for r := range tree {
		writeFile(t, want, r, tree[r])
		writeFile(t, got, r, tree[r])
	}

	diffs, err := parity.Diff(want, got, parity.DefaultEquivalence())
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if len(diffs) != 0 {
		t.Fatalf("expected no diffs, got %v", diffs)
	}
}

func TestDiff_FilePresence(t *testing.T) {
	want := t.TempDir()
	got := t.TempDir()
	writeFile(t, want, "only-in-want.md", "x")
	writeFile(t, got, "only-in-got.md", "y")

	diffs, err := parity.Diff(want, got, parity.DefaultEquivalence())
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if len(diffs) != 2 {
		t.Fatalf("expected 2 presence diffs, got %d: %v", len(diffs), diffs)
	}
	for _, d := range diffs {
		if d.Axis != "presence" {
			t.Fatalf("unexpected axis %q in %+v", d.Axis, d)
		}
	}
}

func TestDiff_FrontmatterKeysAndValues(t *testing.T) {
	want := t.TempDir()
	got := t.TempDir()
	writeFile(t, want, "n.md", "---\ntitle: same\nstatus: active\n---\nbody\n")
	writeFile(t, got, "n.md", "---\ntitle: same\nstatus: archived\n---\nbody\n")

	diffs, err := parity.Diff(want, got, parity.DefaultEquivalence())
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 frontmatter diff, got %d: %v", len(diffs), diffs)
	}
	if diffs[0].Axis != "frontmatter" {
		t.Fatalf("axis: got %q", diffs[0].Axis)
	}
}

func TestDiff_WikilinkSetIsOrderInsensitive(t *testing.T) {
	want := t.TempDir()
	got := t.TempDir()
	writeFile(t, want, "n.md", "links: [[A]] [[B]] [[A]]\n")
	writeFile(t, got, "n.md", "links: [[B]] [[A]]\n")

	diffs, err := parity.Diff(want, got, parity.DefaultEquivalence())
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if len(diffs) != 0 {
		t.Fatalf("expected no diff (same set), got %v", diffs)
	}
}

func TestDiff_WikilinkMismatchIsReported(t *testing.T) {
	want := t.TempDir()
	got := t.TempDir()
	writeFile(t, want, "n.md", "see [[A]] and [[B]]\n")
	writeFile(t, got, "n.md", "see [[A]] and [[C]]\n")

	diffs, err := parity.Diff(want, got, parity.DefaultEquivalence())
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if len(diffs) != 1 || diffs[0].Axis != "wikilinks" {
		t.Fatalf("expected one wikilinks diff, got %v", diffs)
	}
}

func TestDiff_BodyNotComparedByDefault(t *testing.T) {
	want := t.TempDir()
	got := t.TempDir()
	writeFile(t, want, "n.md", "---\ntitle: t\n---\nWANT body\n")
	writeFile(t, got, "n.md", "---\ntitle: t\n---\nGOT body\n")

	diffs, err := parity.Diff(want, got, parity.DefaultEquivalence())
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if len(diffs) != 0 {
		t.Fatalf("default equivalence should ignore body; got %v", diffs)
	}
}

func TestDiff_BodyComparedUnderStrict(t *testing.T) {
	want := t.TempDir()
	got := t.TempDir()
	writeFile(t, want, "n.md", "---\ntitle: t\n---\nWANT body\n")
	writeFile(t, got, "n.md", "---\ntitle: t\n---\nGOT body\n")

	diffs, err := parity.Diff(want, got, parity.StrictEquivalence())
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if len(diffs) != 1 || diffs[0].Axis != "body" {
		t.Fatalf("expected one body diff under strict, got %v", diffs)
	}
}

func TestDiff_IgnorePathsSuppressAllAxes(t *testing.T) {
	want := t.TempDir()
	got := t.TempDir()
	writeFile(t, want, "_meta/log.md", "---\ntitle: log\n---\nA\n")
	writeFile(t, got, "_meta/log.md", "---\ntitle: differs\n---\nB [[C]]\n")
	writeFile(t, want, "real.md", "real\n")
	writeFile(t, got, "real.md", "real\n")

	eq := parity.DefaultEquivalence()
	eq.IgnorePaths = []string{"_meta/log.md"}

	diffs, err := parity.Diff(want, got, eq)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if len(diffs) != 0 {
		t.Fatalf("ignored path should produce no diffs, got %v", diffs)
	}
}

func TestDiff_NonMarkdownFilesAreIgnored(t *testing.T) {
	want := t.TempDir()
	got := t.TempDir()
	writeFile(t, want, "attachments/a.png", "binary-A")
	writeFile(t, got, "attachments/a.png", "binary-B")

	diffs, err := parity.Diff(want, got, parity.StrictEquivalence())
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if len(diffs) != 0 {
		t.Fatalf("non-md files should not gate parity, got %v", diffs)
	}
}

// writeFile creates root/rel (incl. parent dirs) with content. Local
// helper per rule 7; promote when a second consumer needs it.
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", full, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}
