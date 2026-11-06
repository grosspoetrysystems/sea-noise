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

func samePath(got, want string) bool {
	gotInfo, gotErr := os.Stat(got)
	wantInfo, wantErr := os.Stat(want)
	return gotErr == nil && wantErr == nil && os.SameFile(gotInfo, wantInfo)
}

func TestExternalSourceEventsAndReloadKeepWorkflowBoundaries(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(external, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(external, "original.md")
	nestedNote := filepath.Join(nested, "note.md")
	if err := os.WriteFile(original, []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nestedNote, []byte("nested\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(external, "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}

	ordinary := filepath.Join(root, "topics", "ordinary.md")
	if err := os.MkdirAll(filepath.Dir(ordinary), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ordinary, []byte("ordinary seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	modified := make(chan string, 16)
	inbox := make(chan string, 16)
	w, err := watcher.New(watcher.Config{
		VaultRoot:   root,
		Debounce:    20 * time.Millisecond,
		InboxSettle: 20 * time.Millisecond,
		Sources:     []vault.Source{{LogicalRoot: "vernacular/alpha/apparatus", PhysicalRoot: external}},
		Actions: watcher.Actions{
			OnNoteModify:  func(_ context.Context, path string) { modified <- path },
			OnInboxCreate: func(_ context.Context, path string) { inbox <- path },
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

	if err := os.WriteFile(original, []byte("original edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := awaitModify(t, modified); !samePath(got, original) {
		t.Fatalf("original callback path = %q, want %q", got, original)
	}
	if logical, ok := w.LogicalPath(original); !ok || logical != "vernacular/alpha/apparatus/original.md" {
		t.Fatalf("original logical path = %q, %v", logical, ok)
	}

	if err := os.WriteFile(nestedNote, []byte("nested edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := awaitModify(t, modified); !samePath(got, nestedNote) {
		t.Fatalf("nested callback path = %q, want %q", got, nestedNote)
	}
	if logical, ok := w.LogicalPath(nestedNote); !ok || logical != "vernacular/alpha/apparatus/nested/note.md" {
		t.Fatalf("nested logical path = %q, %v", logical, ok)
	}

	projectInboxNote := filepath.Join(external, "inbox", "project.md")
	if err := os.WriteFile(projectInboxNote, []byte("project inbox\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := awaitModify(t, modified); !samePath(got, projectInboxNote) {
		t.Fatalf("project inbox callback path = %q, want %q", got, projectInboxNote)
	}
	select {
	case got := <-inbox:
		t.Fatalf("external project inbox fired workflow callback for %q", got)
	case <-time.After(100 * time.Millisecond):
	}

	if err := w.ReloadSources(nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(original, []byte("stale target edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-modified:
		t.Fatalf("stale external target remained watched: %q", got)
	case <-time.After(100 * time.Millisecond):
	}
	if err := os.WriteFile(ordinary, []byte("ordinary edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := awaitModify(t, modified); !samePath(got, ordinary) {
		t.Fatalf("ordinary vault callback path = %q, want %q", got, ordinary)
	}
}

func TestExternalDirectoryCreateRefreshesPopulatedMoveIn(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	modified := make(chan string, 8)
	w, err := watcher.New(watcher.Config{
		VaultRoot: root,
		Debounce:  20 * time.Millisecond,
		Sources:   []vault.Source{{LogicalRoot: "vernacular/alpha/working", PhysicalRoot: external}},
		Actions: watcher.Actions{
			OnNoteModify: func(_ context.Context, path string) { modified <- path },
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

	moved := filepath.Join(external, "moved")
	if err := os.MkdirAll(moved, 0o755); err != nil {
		t.Fatal(err)
	}
	movedNote := filepath.Join(moved, "first.md")
	if err := os.WriteFile(movedNote, []byte("first write raced directory registration\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := awaitModify(t, modified)
	if !samePath(got, moved) {
		t.Fatalf("populated directory refresh path = %q, want %q", got, moved)
	}
	if logical, ok := w.LogicalPath(got); !ok || logical != "vernacular/alpha/working/moved" {
		t.Fatalf("populated directory logical path = %q, %v", logical, ok)
	}
}

func awaitModify(t *testing.T, modified <-chan string) string {
	t.Helper()
	select {
	case path := <-modified:
		return path
	case <-time.After(2 * time.Second):
		t.Fatal("OnNoteModify did not fire")
		return ""
	}
}
