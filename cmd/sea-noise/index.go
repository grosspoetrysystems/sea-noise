package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"sea-noise/internal/config"
	"sea-noise/internal/retrieval"
)

// runIndex implements `sea-noise index --db PATH`.
//
// Rebuilds the SQLite retrieval index for the configured vault. The index
// is wiped and repopulated in a single transaction; existing readers
// (none today) would see the prior state until commit.
func runIndex(args []string) error {
	fs := flag.NewFlagSet("index", flag.ContinueOnError)
	dbPath := fs.String("db", "", "path to the SQLite index file (will be created if absent)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage: sea-noise index --db PATH

Rebuild the SQLite retrieval index for the configured vault.

Flags:
  --db PATH    where to write the SQLite index file (required)`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dbPath == "" {
		fs.Usage()
		return fmt.Errorf("--db is required")
	}

	vaultRoot, err := config.LoadVaultPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(*dbPath), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(*dbPath), err)
	}

	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		return fmt.Errorf("open db %s: %w", *dbPath, err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := retrieval.Migrate(ctx, db); err != nil {
		return err
	}

	stats, err := retrieval.IndexVault(ctx, db, vaultRoot, nil)
	if err != nil {
		return err
	}

	fmt.Printf("Indexed %d notes, %d chunks, %d tags, %d links, %d timestamps → %s\n",
		stats.Notes, stats.Chunks, stats.Tags, stats.Links, stats.Timestamps, *dbPath)
	return nil
}
