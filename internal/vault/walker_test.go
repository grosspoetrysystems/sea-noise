package vault

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
)

func TestWalk_VaultNotFound(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist")

	_, err := Walk(missing, nil)
	if !errors.Is(err, ErrVaultNotFound) {
		t.Fatalf("Walk(%q) = %v, want errors.Is(err, ErrVaultNotFound)", missing, err)
	}
}

func TestWalk_PathIsFile(t *testing.T) {
	dir := t.TempDir()
	notADir := filepath.Join(dir, "file.md")
	if err := os.WriteFile(notADir, []byte("plain"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Walk(notADir, nil)
	if !errors.Is(err, ErrVaultNotFound) {
		t.Fatalf("Walk(%q) = %v, want errors.Is(err, ErrVaultNotFound)", notADir, err)
	}
}

func TestWalk_SkipsExcludedDirsAndEmptyFiles(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, "note.md", "# Real Note\nbody\n")
	mustWrite(t, dir, "templates/skel.md", "# template\n") // excluded
	mustWrite(t, dir, "empty.md", "   \n  \n")             // skipped (whitespace-only)
	mustWrite(t, dir, "subdir/nested.md", "# Nested\n")
	mustWrite(t, dir, "not-markdown.txt", "ignored\n")

	notes, err := Walk(dir, []string{"templates"})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	gotIDs := make([]string, len(notes))
	for i, n := range notes {
		gotIDs[i] = n.ID
	}
	wantIDs := []string{"note.md", "subdir/nested.md"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Errorf("walked IDs = %v, want %v", gotIDs, wantIDs)
	}
}

func TestWalk_TitleFallbacks(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, "frontmatter-title.md", "---\ntitle: From FM\n---\nbody\n")
	mustWrite(t, dir, "heading-title.md", "# From Heading\n\nbody\n")
	mustWrite(t, dir, "stem-fallback-name.md", "plain body, no title source\n")

	notes, err := Walk(dir, nil)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	want := map[string]string{
		"frontmatter-title.md":  "From FM",
		"heading-title.md":      "From Heading",
		"stem-fallback-name.md": "stem fallback name",
	}
	for _, n := range notes {
		if got := n.Title; got != want[n.ID] {
			t.Errorf("Note %q title = %q, want %q", n.ID, got, want[n.ID])
		}
	}
}

// TestWalk_ParityIndexer is the parity gate: walking the T09 fixture vault
// with the indexer's exclude set must produce records that match the
// golden file captured from the Python prototype.
func TestWalk_ParityIndexer(t *testing.T) {
	root := repoRoot(t)
	fixture := filepath.Join(root, "testdata", "fixture-vault")
	goldenPath := filepath.Join(root, "testdata", "expected", "frontmatter", "index.json")

	notes, err := Walk(fixture, IndexerExcludeDirs)
	if err != nil {
		t.Fatalf("Walk(fixture): %v", err)
	}

	type record struct {
		BodyPreview string `json:"body_preview"`
		ID          string `json:"id"`
		Path        string `json:"path"`
		Tags        string `json:"tags"`
		Title       string `json:"title"`
	}

	got := make([]record, 0, len(notes))
	for _, n := range notes {
		got = append(got, record{
			BodyPreview: runeSlice(n.Body, 4000),
			ID:          n.ID,
			Path:        n.Path,
			Tags:        n.Tags,
			Title:       n.Title,
		})
	}

	var want []record
	goldenBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %q: %v", goldenPath, err)
	}
	if err := json.Unmarshal(goldenBytes, &want); err != nil {
		t.Fatalf("parse golden: %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("record count mismatch: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("record %d (%q) mismatch:\n  got:  %+v\n  want: %+v", i, want[i].ID, got[i], want[i])
		}
	}
}

// TestExtractWikilinks_ParityAuditLinks: walking the fixture vault with
// the audit-links exclude set and extracting wikilinks per note must
// match the golden per-note.json captured from the Python prototype.
func TestExtractWikilinks_ParityAuditLinks(t *testing.T) {
	// Pulled into this package to share the repoRoot/walker plumbing.
	// When a second cross-package consumer needs these helpers, lift to
	// internal/testhelper per test-discipline rule 7.
	root := repoRoot(t)
	fixture := filepath.Join(root, "testdata", "fixture-vault")
	goldenPath := filepath.Join(root, "testdata", "expected", "links", "per-note.json")

	notes, err := Walk(fixture, LinksExcludeDirs)
	if err != nil {
		t.Fatalf("Walk(fixture): %v", err)
	}

	type linkRecord struct {
		Links  []string `json:"links"`
		Source string   `json:"source"`
	}

	// The walker reads body; for link extraction we want the *original*
	// content, not the post-frontmatter body. Re-read each file.
	got := make([]linkRecord, 0, len(notes))
	for _, n := range notes {
		full, err := os.ReadFile(filepath.Join(fixture, n.Path))
		if err != nil {
			t.Fatalf("read %q: %v", n.Path, err)
		}
		got = append(got, linkRecord{
			Source: n.ID,
			Links:  extractWikilinksForTest(string(full)),
		})
	}

	var want []linkRecord
	goldenBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %q: %v", goldenPath, err)
	}
	if err := json.Unmarshal(goldenBytes, &want); err != nil {
		t.Fatalf("parse golden: %v", err)
	}

	// Golden is sorted by JSON key (source) after sort_keys=True; our
	// records come out in walker order. Sort both consistently.
	sort.Slice(got, func(i, j int) bool { return got[i].Source < got[j].Source })
	sort.Slice(want, func(i, j int) bool { return want[i].Source < want[j].Source })

	if len(got) != len(want) {
		t.Fatalf("record count mismatch: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Source != want[i].Source {
			t.Errorf("record %d source mismatch: got %q, want %q", i, got[i].Source, want[i].Source)
			continue
		}
		if !equalStringSlices(got[i].Links, want[i].Links) {
			t.Errorf("record %q links mismatch:\n  got:  %#v\n  want: %#v", got[i].Source, got[i].Links, want[i].Links)
		}
	}
}

// === helpers (file-local; lift to testhelper when a second consumer needs them) ===

func mustWrite(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// This file is at <repo>/internal/vault/walker_test.go.
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func runeSlice(s string, max int) string {
	r := []rune(s)
	if len(r) > max {
		r = r[:max]
	}
	return string(r)
}

func equalStringSlices(a, b []string) bool {
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
