package retrieval

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrMigrate is returned when applying the schema fails. Wrap with %w;
// check with errors.Is.
var ErrMigrate = errors.New("retrieval: migrate failed")

// schemaStatements is the full DDL for the retrieval index. The set of
// tables mirrors ARCHITECTURE.md §3. Statements are independently
// executed and idempotent (`IF NOT EXISTS`). Foreign keys reference
// notes.id (the relative vault path); rebuilds wipe rows via the index
// drop-rebuild flow, not via cascading deletes here.
//
// FTS5 virtual tables use external content tied to the source tables.
// The indexer is responsible for syncing FTS5 rows after inserts/updates.
// Triggers are intentionally not used in this wave — explicit sync is
// easier to reason about while parity is still being established.
var schemaStatements = []string{
	`PRAGMA foreign_keys = ON`,

	`CREATE TABLE IF NOT EXISTS notes (
		id TEXT PRIMARY KEY,
		path TEXT NOT NULL,
		type TEXT,
		title TEXT NOT NULL,
		tags TEXT NOT NULL DEFAULT '',
		mtime INTEGER,
		frontmatter_json TEXT NOT NULL DEFAULT '{}'
	)`,

	`CREATE TABLE IF NOT EXISTS chunks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		note_id TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
		ordinal INTEGER NOT NULL,
		body TEXT NOT NULL,
		UNIQUE (note_id, ordinal)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_chunks_note ON chunks(note_id)`,

	`CREATE TABLE IF NOT EXISTS tags (
		note_id TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
		tag TEXT NOT NULL,
		PRIMARY KEY (note_id, tag)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_tags_tag ON tags(tag)`,

	`CREATE TABLE IF NOT EXISTS links (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		from_note_id TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
		to_note_path TEXT NOT NULL,
		to_note_id TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_links_from ON links(from_note_id)`,
	`CREATE INDEX IF NOT EXISTS idx_links_to_path ON links(to_note_path)`,
	`CREATE INDEX IF NOT EXISTS idx_links_to_id ON links(to_note_id)`,

	`CREATE TABLE IF NOT EXISTS entities (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		note_id TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
		surface TEXT NOT NULL,
		canonical_form TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_entities_note ON entities(note_id)`,
	`CREATE INDEX IF NOT EXISTS idx_entities_canonical ON entities(canonical_form)`,

	`CREATE TABLE IF NOT EXISTS assertions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		note_id TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
		statement TEXT NOT NULL,
		confidence REAL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_assertions_note ON assertions(note_id)`,

	`CREATE TABLE IF NOT EXISTS timestamps (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		note_id TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
		kind TEXT NOT NULL,
		value TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_timestamps_note ON timestamps(note_id)`,
	`CREATE INDEX IF NOT EXISTS idx_timestamps_kind ON timestamps(kind)`,

	`CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(
		title, tags, frontmatter_json,
		content='notes', content_rowid='rowid',
		tokenize='porter unicode61'
	)`,

	`CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
		body,
		content='chunks', content_rowid='id',
		tokenize='porter unicode61'
	)`,
}

// Migrate applies the retrieval schema to db. Idempotent — safe to call
// against a fresh database or one that has already been migrated.
func Migrate(ctx context.Context, db *sql.DB) error {
	for _, stmt := range schemaStatements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("%w: %q: %v", ErrMigrate, firstLine(stmt), err)
		}
	}
	return nil
}

// firstLine extracts the first non-empty line of a SQL statement for
// error messages. Avoids dumping the full multi-line DDL.
func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	if len(s) > 80 {
		return s[:80]
	}
	return s
}
