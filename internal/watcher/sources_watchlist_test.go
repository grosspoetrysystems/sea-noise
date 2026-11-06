package watcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sea-noise/internal/vault"
)

func TestSuspendedSourceNeverWatchesThroughReplacedIntermediateSymlink(t *testing.T) {
	root, parent, outside := t.TempDir(), t.TempDir(), t.TempDir()
	outer := filepath.Join(parent, "outer")
	space := filepath.Join(outer, "project-space")
	target := filepath.Join(space, "apparatus", "overview.md")
	for _, file := range []string{target, filepath.Join(outside, "apparatus", "overview.md")} {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("# Overview\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w, err := New(Config{
		VaultRoot: root,
		Sources:   []vault.Source{{LogicalRoot: "vernacular/alpha/overview.md", PhysicalRoot: target}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := w.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	if err := os.RemoveAll(space); err != nil {
		t.Fatal(err)
	}
	if err := w.SuspendSources(); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, space); err != nil {
		t.Fatal(err)
	}
	if err := w.SuspendSources(); err != nil {
		t.Fatal(err)
	}
	ancestor, err := filepath.EvalSymlinks(outer)
	if err != nil {
		t.Fatal(err)
	}
	unselected, err := filepath.EvalSymlinks(outside)
	if err != nil {
		t.Fatal(err)
	}
	watches := w.fsw.WatchList()
	observesRestoration := false
	for _, watch := range watches {
		resolved, err := filepath.EvalSymlinks(watch)
		if err != nil {
			t.Fatal(err)
		}
		if resolved == ancestor {
			observesRestoration = true
		}
		rel, err := filepath.Rel(unselected, resolved)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("unselected subtree is watched through %q: %v", watch, watches)
		}
	}
	if !observesRestoration {
		t.Fatalf("safe restoration ancestor %q is not watched: %v", ancestor, watches)
	}
}
