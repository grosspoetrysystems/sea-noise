package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"

	"sea-noise/internal/config"
	"sea-noise/internal/retrieval"
)

// runIndex implements `sea-noise index [--db PATH]`.
//
// Rebuilds the SQLite retrieval index for the configured vault. The index
// is wiped and repopulated in a single transaction; readers see the prior
// state until commit.
func runIndex(args []string) error {
	fs := flag.NewFlagSet("index", flag.ContinueOnError)
	dbPath := fs.String("db", "", "path to the SQLite index file (defaults to ~/.sea-noise/retrieval.db)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage: sea-noise index [--db PATH]

Rebuild the SQLite retrieval index for the configured vault.

Flags:
  --db PATH    where to write the SQLite index file (defaults to ~/.sea-noise/retrieval.db)`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	vaultRoot, err := config.LoadVaultPath()
	if err != nil {
		return err
	}

	resolvedDBPath, err := resolveIndexDBPath(vaultRoot, *dbPath)
	if err != nil {
		return err
	}

	stats, err := rebuildIndex(context.Background(), vaultRoot, resolvedDBPath)
	if err != nil {
		return err
	}

	fmt.Printf("Indexed %d notes, %d chunks, %d tags, %d links, %d timestamps → %s\n",
		stats.Notes, stats.Chunks, stats.Tags, stats.Links, stats.Timestamps, resolvedDBPath)
	return nil
}

// ponytail: one process indexes one vault; use per-database locks if a
// multi-vault daemon is introduced. Watcher, IPC and scheduled rebuilds
// must not race their write transactions.
var indexMu sync.Mutex

func rebuildIndex(ctx context.Context, vaultRoot, dbPath string) (retrieval.IndexStats, error) {
	indexMu.Lock()
	defer indexMu.Unlock()
	if err := ctx.Err(); err != nil {
		return retrieval.IndexStats{}, err
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return retrieval.IndexStats{}, fmt.Errorf("mkdir %s: %w", filepath.Dir(dbPath), err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return retrieval.IndexStats{}, fmt.Errorf("open db %s: %w", dbPath, err)
	}
	defer db.Close()
	if err := retrieval.Migrate(ctx, db); err != nil {
		return retrieval.IndexStats{}, err
	}
	return retrieval.IndexVault(ctx, db, vaultRoot, nil)
}
