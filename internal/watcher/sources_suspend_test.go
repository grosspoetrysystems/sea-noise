package watcher_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sea-noise/internal/vault"
	"sea-noise/internal/watcher"
)

func TestSuspendSourcesObservesRestoreButEmptyReloadClearsSentinel(t *testing.T) {
	root := t.TempDir()
	parent := t.TempDir()
	space := filepath.Join(parent, "project-space")
	target := filepath.Join(space, "apparatus")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	sourcesChanged := make(chan struct{}, 8)
	modified := make(chan string, 8)
	w, err := watcher.New(watcher.Config{
		VaultRoot: root,
		Sources:   []vault.Source{{LogicalRoot: "vernacular/alpha/apparatus", PhysicalRoot: target}},
		Actions: watcher.Actions{
			OnNoteModify:    func(_ context.Context, path string) { modified <- path },
			OnSourcesChange: func(_ context.Context) { sourcesChanged <- struct{}{} },
		},
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
	select {
	case <-sourcesChanged:
	case <-time.After(100 * time.Millisecond):
	}
	sibling := filepath.Join(parent, "unrelated")
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sibling, "note.md"), []byte("unrelated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-modified:
		t.Fatalf("unrelated sentinel sibling was watched: %q", got)
	case <-time.After(100 * time.Millisecond):
	}
	if err := os.MkdirAll(space, 0o755); err != nil {
		t.Fatal(err)
	}
	select {
	case <-sourcesChanged:
	case <-time.After(2 * time.Second):
		t.Fatal("restored enclosing space did not trigger OnSourcesChange")
	}
	if err := w.SuspendSources(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	select {
	case <-sourcesChanged:
	case <-time.After(2 * time.Second):
		t.Fatal("restored source did not trigger OnSourcesChange")
	}

	if err := w.ReloadSources(nil); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(space); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	select {
	case <-sourcesChanged:
		t.Fatal("empty source reload left a stale restoration sentinel")
	case <-time.After(100 * time.Millisecond):
	}
}
