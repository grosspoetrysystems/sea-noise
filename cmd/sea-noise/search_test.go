package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"sea-noise/internal/retrieval"
)

func TestRunSearchUsesDefaultIndexPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vault := t.TempDir()
	t.Setenv("MIND_PALACE_VAULT", vault)

	dbPath := filepath.Join(home, ".sea-noise", indexDBFileName)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := retrieval.Migrate(context.Background(), db); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if err := runSearch([]string{"needle"}); err != nil {
		t.Fatalf("runSearch: %v", err)
	}
}
