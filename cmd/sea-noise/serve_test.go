package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"sea-noise/internal/daemon"
	"sea-noise/internal/vernacular"
	"sea-noise/internal/watcher"
)

func TestStartWatcherKeepsNewestSourceAfterOverlappingReloads(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}

	control := t.TempDir()
	alpha := writeWatcherRaceProject(t, "alpha", "alpha initial")
	beta := writeWatcherRaceProject(t, "beta", "beta initial")
	installWatcherRaceVern(t, control, alpha)

	if _, err := vernacular.Attach(context.Background(), root, "alpha", alpha); err != nil {
		t.Fatalf("attach alpha: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "index.db")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := daemon.New(daemon.Config{VaultRoot: root}, daemon.Registry{})
	w, err := startWatcher(ctx, srv, daemonStepDeps{VaultRoot: root, IndexDBPath: dbPath}, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("start watcher: %v", err)
	}
	defer w.Stop()

	if err := touchWatcherRaceFile(filepath.Join(control, "block")); err != nil {
		t.Fatal(err)
	}
	// A real source-control event starts the first callback while membership
	// still contains only alpha. The vern fixture holds its first resolution.
	if err := touchWatcherRaceFile(filepath.Join(root, "vernacular", ".race-trigger")); err != nil {
		t.Fatal(err)
	}
	waitWatcherRaceFile(t, filepath.Join(control, "entered"), 2*time.Second)

	if _, err := vernacular.Attach(context.Background(), root, "beta", beta); err != nil {
		t.Fatalf("attach beta: %v", err)
	}

	betaNote := filepath.Join(beta, "working", "beta-note.md")
	betaID := "vernacular/beta/working/beta-note.md"
	// Observe the installed mapping, not a vern process that might belong to
	// an independent index refresh. Only a source reload installs this target.
	newerMapped := watcherRaceLogicalPathAppears(w, betaNote, betaID, time.Second)
	if newerMapped {
		waitWatcherRaceChunk(t, dbPath, betaID, "beta initial", 3*time.Second)
	}

	if err := touchWatcherRaceFile(filepath.Join(control, "release")); err != nil {
		t.Fatal(err)
	}
	waitWatcherRaceFile(t, filepath.Join(control, "old-working"), 2*time.Second)
	if err := touchWatcherRaceFile(filepath.Join(control, "final-release")); err != nil {
		t.Fatal(err)
	}

	if newerMapped {
		// The uncorrected callback order is now observable as the newest
		// target becoming unmapped after the older callback reloads alpha.
		waitWatcherRaceLogicalPathAbsent(t, w, betaNote, 3*time.Second)
		t.Fatal("older source resolution replaced the newer beta membership")
	}

	// With serialization, the queued newer callback starts only after the old
	// callback has completed; wait for its positive indexing evidence before
	// checking future target edits.
	waitWatcherRaceChunk(t, dbPath, betaID, "beta initial", 3*time.Second)
	waitWatcherRaceLogicalPath(t, w, betaNote, "vernacular/beta/working/beta-note.md", 3*time.Second)
	if err := os.WriteFile(betaNote, []byte("beta updated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitWatcherRaceChunk(t, dbPath, betaID, "beta updated", 3*time.Second)
}

func writeWatcherRaceProject(t *testing.T, key, body string) string {
	t.Helper()
	checkout := t.TempDir()
	for _, dir := range []string{"canon", "apparatus", "working"} {
		if err := os.MkdirAll(filepath.Join(checkout, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(checkout, "working", key+"-note.md"), []byte(body+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return checkout
}

func installWatcherRaceVern(t *testing.T, control, alpha string) {
	t.Helper()
	alpha, err := filepath.EvalSymlinks(alpha)
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	stub := filepath.Join(bin, "vern")
	const script = `#!/bin/sh
set -eu
cwd=$(pwd -P)
control=${SEA_NOISE_TEST_VERN_CONTROL:?}
if [ -f "$control/block" ] && [ ! -f "$control/entered" ]; then
  : > "$control/entered"
  while [ ! -f "$control/release" ]; do sleep 0.01; done
fi
if [ "$cwd" = "${SEA_NOISE_TEST_VERN_ALPHA:-}" ] && [ "$2" = "working" ] && [ -f "$control/release" ] && [ ! -f "$control/old-working" ]; then
  : > "$control/old-working"
  while [ ! -f "$control/final-release" ]; do sleep 0.01; done
fi
case "$2" in
  canon|apparatus|working) printf '%s/%s\n' "$cwd" "$2" ;;
  *) exit 1 ;;
esac
`
	// Keep fixture paths in the environment so the shell stays independent of
	// temporary-directory quoting and the test exercises exec.CommandContext.
	t.Setenv("SEA_NOISE_TEST_VERN_CONTROL", control)
	t.Setenv("SEA_NOISE_TEST_VERN_ALPHA", alpha)
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func touchWatcherRaceFile(path string) error {
	return os.WriteFile(path, []byte("signal\n"), 0o644)
}

func waitWatcherRaceFile(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	if !watcherRaceFileAppears(path, timeout) {
		t.Fatalf("timed out waiting for %s", path)
	}
}

func watcherRaceFileAppears(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func waitWatcherRaceChunk(t *testing.T, dbPath, noteID, text string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if body, ok := watcherRaceChunk(dbPath, noteID); ok && strings.Contains(body, text) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s to contain %q", noteID, text)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
func watcherRaceChunk(dbPath, noteID string) (string, bool) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return "", false
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	var body string
	err = db.QueryRowContext(ctx, `SELECT body FROM chunks WHERE note_id = ? ORDER BY ordinal LIMIT 1`, noteID).Scan(&body)
	return body, err == nil
}

func waitWatcherRaceLogicalPathAbsent(t *testing.T, w *watcher.Watcher, physical string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if _, ok := w.LogicalPath(physical); !ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for logical path %q to become unmapped", physical)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func waitWatcherRaceLogicalPath(t *testing.T, w *watcher.Watcher, physical, logical string, timeout time.Duration) {
	t.Helper()
	if !watcherRaceLogicalPathAppears(w, physical, logical, timeout) {
		t.Fatalf("timed out waiting for logical path %q -> %q", physical, logical)
	}
}

func watcherRaceLogicalPathAppears(w *watcher.Watcher, physical, logical string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if got, ok := w.LogicalPath(physical); ok && got == logical {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(2 * time.Millisecond)
	}
}
