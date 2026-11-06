package watcher_test

import (
	"os"
	"path/filepath"
	"testing"

	"sea-noise/internal/vault"
	"sea-noise/internal/watcher"
)

func TestLogicalPathAndReloadSources(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	if err := os.MkdirAll(filepath.Join(external, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	w, err := watcher.New(watcher.Config{
		VaultRoot: root,
		Sources: []vault.Source{{
			LogicalRoot:  "vernacular/alpha/apparatus",
			PhysicalRoot: external,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	logical, ok := w.LogicalPath(filepath.Join(external, "nested", "note.md"))
	if !ok || logical != "vernacular/alpha/apparatus/nested/note.md" {
		t.Fatalf("LogicalPath = %q, %v", logical, ok)
	}
	if logical, ok := w.LogicalPath(filepath.Join(root, "inbox", "note.md")); !ok || logical != "inbox/note.md" {
		t.Fatalf("vault LogicalPath = %q, %v", logical, ok)
	}
	if err := w.ReloadSources(nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := w.LogicalPath(filepath.Join(external, "nested", "note.md")); ok {
		t.Fatal("external mapping survived ReloadSources(nil)")
	}
}
func TestNewRejectsSymmetricVaultOverlap(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "external")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, physical := range []string{inside, filepath.Dir(root)} {
		_, err := watcher.New(watcher.Config{
			VaultRoot: root,
			Sources: []vault.Source{{
				LogicalRoot:  "vernacular/alpha/apparatus",
				PhysicalRoot: physical,
			}},
		})
		if err == nil {
			t.Fatalf("source %q overlap was accepted", physical)
		}
	}
}
