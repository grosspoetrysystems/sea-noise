package retrieval_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"sea-noise/internal/retrieval"
)

// TestIndexDump_FixtureVaultGolden is a regression test against the
// committed retrieval-index dump for testdata/fixture-vault. If the
// indexer changes shape (new fields, different chunking, link
// resolution rules, etc.), this test diffs the actual dump against
// testdata/expected/retrieval/index.json and reports the file path
// to update.
//
// Regenerate the golden ONLY as a deliberate, reviewable change:
//
//	REGEN_RETRIEVAL_GOLDEN=1 go test ./internal/retrieval/ -run TestIndexDump_FixtureVaultGolden
//
// This is a regression test, not a parity test against the Python
// prototype — the prototype's index shape (LanceDB, one row per note)
// is incompatible with the Go index by design (ARCHITECTURE.md §2).
func TestIndexDump_FixtureVaultGolden(t *testing.T) {
	vaultRoot, repoRoot := findFixtureVault(t)

	db := openDB(t)
	if _, err := retrieval.IndexVault(context.Background(), db, vaultRoot, nil); err != nil {
		t.Fatalf("IndexVault: %v", err)
	}
	dump, err := retrieval.DumpIndex(context.Background(), db)
	if err != nil {
		t.Fatalf("DumpIndex: %v", err)
	}
	got, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got = append(got, '\n')

	goldenPath := filepath.Join(repoRoot, "testdata", "expected", "retrieval", "index.json")

	if os.Getenv("REGEN_RETRIEVAL_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir golden parent: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", goldenPath, err)
		}
		t.Logf("regenerated %s (%d bytes)", goldenPath, len(got))
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v\nTo create it the first time, run:\n  REGEN_RETRIEVAL_GOLDEN=1 go test ./internal/retrieval/ -run TestIndexDump_FixtureVaultGolden",
			goldenPath, err)
	}
	if string(got) != string(want) {
		// Write the actual output next to the golden so the diff is
		// easy to inspect in a failing CI / local run.
		actualPath := goldenPath + ".actual"
		_ = os.WriteFile(actualPath, got, 0o644)
		t.Fatalf("retrieval index dump diverges from golden.\n  golden: %s\n  actual: %s\nTo accept the new state, run:\n  REGEN_RETRIEVAL_GOLDEN=1 go test ./internal/retrieval/ -run TestIndexDump_FixtureVaultGolden",
			goldenPath, actualPath)
	}
}

// findFixtureVault walks upward from the package's working directory to
// locate testdata/fixture-vault and returns (vaultPath, repoRoot).
// Test is skipped if the fixture cannot be found.
func findFixtureVault(t *testing.T) (string, string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := wd
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(root, "testdata", "fixture-vault")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, root
		}
		root = filepath.Dir(root)
	}
	t.Skipf("testdata/fixture-vault not found from %s", wd)
	return "", ""
}
