package retrieval

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sea-noise/internal/markdown"
	"sea-noise/internal/vault"
)

// ErrIndex is returned when indexing fails after migration has succeeded.
// Wrap with %w; check with errors.Is.
var ErrIndex = errors.New("retrieval: index failed")

// IndexStats summarises an indexing pass.
type IndexStats struct {
	Notes      int
	Chunks     int
	Tags       int
	Links      int
	Timestamps int
}

// IndexVault populates db from the markdown files under root. The schema
// must already be applied (call Migrate first). This function fully
// rebuilds the index from scratch in a single transaction: existing rows
// are wiped before insert. The vault is the source of truth — incremental
// indexing is deferred until a later wave once parity is established.
//
// excludeDirs defaults to vault.IndexerExcludeDirs when nil; pass an
// explicit slice to override (e.g., test scenarios that include /signal).
func IndexVault(ctx context.Context, db *sql.DB, root string, excludeDirs []string) (IndexStats, error) {
	if excludeDirs == nil {
		excludeDirs = vault.IndexerExcludeDirs
	}
	notes, err := vault.Walk(root, excludeDirs)
	if err != nil {
		return IndexStats{}, fmt.Errorf("%w: walk: %v", ErrIndex, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return IndexStats{}, fmt.Errorf("%w: begin: %v", ErrIndex, err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := wipeIndex(ctx, tx); err != nil {
		return IndexStats{}, err
	}

	stats, err := insertAll(ctx, tx, root, notes)
	if err != nil {
		return IndexStats{}, err
	}

	if err := resolveLinks(ctx, tx); err != nil {
		return IndexStats{}, err
	}

	if err := tx.Commit(); err != nil {
		return IndexStats{}, fmt.Errorf("%w: commit: %v", ErrIndex, err)
	}
	return stats, nil
}

// wipeIndex clears all retrieval tables, including the FTS5 virtuals.
// Called inside a transaction at the start of every IndexVault pass.
func wipeIndex(ctx context.Context, tx *sql.Tx) error {
	for _, stmt := range []string{
		`DELETE FROM notes_fts`,
		`DELETE FROM chunks_fts`,
		`DELETE FROM timestamps`,
		`DELETE FROM assertions`,
		`DELETE FROM entities`,
		`DELETE FROM links`,
		`DELETE FROM tags`,
		`DELETE FROM chunks`,
		`DELETE FROM notes`,
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("%w: wipe %q: %v", ErrIndex, firstLine(stmt), err)
		}
	}
	return nil
}

func insertAll(ctx context.Context, tx *sql.Tx, root string, notes []vault.Note) (IndexStats, error) {
	var stats IndexStats

	noteStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO notes (id, path, type, title, tags, mtime, frontmatter_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return stats, fmt.Errorf("%w: prepare notes: %v", ErrIndex, err)
	}
	defer noteStmt.Close()

	notesFTSStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO notes_fts (rowid, title, tags, frontmatter_json)
		VALUES ((SELECT rowid FROM notes WHERE id = ?), ?, ?, ?)
	`)
	if err != nil {
		return stats, fmt.Errorf("%w: prepare notes_fts: %v", ErrIndex, err)
	}
	defer notesFTSStmt.Close()

	chunkStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO chunks (note_id, ordinal, body) VALUES (?, ?, ?)
	`)
	if err != nil {
		return stats, fmt.Errorf("%w: prepare chunks: %v", ErrIndex, err)
	}
	defer chunkStmt.Close()

	chunksFTSStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO chunks_fts (rowid, body) VALUES (?, ?)
	`)
	if err != nil {
		return stats, fmt.Errorf("%w: prepare chunks_fts: %v", ErrIndex, err)
	}
	defer chunksFTSStmt.Close()

	tagStmt, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO tags (note_id, tag) VALUES (?, ?)`)
	if err != nil {
		return stats, fmt.Errorf("%w: prepare tags: %v", ErrIndex, err)
	}
	defer tagStmt.Close()

	linkStmt, err := tx.PrepareContext(ctx, `INSERT INTO links (from_note_id, to_note_path) VALUES (?, ?)`)
	if err != nil {
		return stats, fmt.Errorf("%w: prepare links: %v", ErrIndex, err)
	}
	defer linkStmt.Close()

	timestampStmt, err := tx.PrepareContext(ctx, `INSERT INTO timestamps (note_id, kind, value) VALUES (?, ?, ?)`)
	if err != nil {
		return stats, fmt.Errorf("%w: prepare timestamps: %v", ErrIndex, err)
	}
	defer timestampStmt.Close()

	for _, n := range notes {
		abs := filepath.Join(root, filepath.FromSlash(n.ID))
		yfm, _ := markdown.ParseFrontmatterYAML(noteBytes(abs))

		typeStr := stringField(yfm.Fields, "type")
		fmJSON, err := json.Marshal(yfm.Fields)
		if err != nil {
			fmJSON = []byte(`{}`)
		}

		var mtimeUnix sql.NullInt64
		if info, err := os.Stat(abs); err == nil {
			mtimeUnix.Valid = true
			mtimeUnix.Int64 = info.ModTime().Unix()
		}

		if _, err := noteStmt.ExecContext(ctx, n.ID, n.Path, typeStr, n.Title, n.Tags, mtimeUnix, string(fmJSON)); err != nil {
			return stats, fmt.Errorf("%w: insert note %s: %v", ErrIndex, n.ID, err)
		}
		stats.Notes++

		if _, err := notesFTSStmt.ExecContext(ctx, n.ID, n.Title, n.Tags, string(fmJSON)); err != nil {
			return stats, fmt.Errorf("%w: insert notes_fts %s: %v", ErrIndex, n.ID, err)
		}

		tagSet := collectTags(n.Tags, yfm.StringTags())
		for tag := range tagSet {
			if _, err := tagStmt.ExecContext(ctx, n.ID, tag); err != nil {
				return stats, fmt.Errorf("%w: insert tag %s/%s: %v", ErrIndex, n.ID, tag, err)
			}
			stats.Tags++
		}

		for _, target := range markdown.ExtractWikilinks(n.Body) {
			if _, err := linkStmt.ExecContext(ctx, n.ID, target); err != nil {
				return stats, fmt.Errorf("%w: insert link %s -> %s: %v", ErrIndex, n.ID, target, err)
			}
			stats.Links++
		}

		if mtimeUnix.Valid {
			if _, err := timestampStmt.ExecContext(ctx, n.ID, "mtime", strconv.FormatInt(mtimeUnix.Int64, 10)); err != nil {
				return stats, fmt.Errorf("%w: insert mtime ts %s: %v", ErrIndex, n.ID, err)
			}
			stats.Timestamps++
		}
		for _, kind := range []string{"created", "updated", "date"} {
			if v := stringField(yfm.Fields, kind); v != "" {
				if _, err := timestampStmt.ExecContext(ctx, n.ID, kind, v); err != nil {
					return stats, fmt.Errorf("%w: insert %s ts %s: %v", ErrIndex, kind, n.ID, err)
				}
				stats.Timestamps++
			}
		}

		chunks := ChunkBody(n.Body)
		if len(chunks) == 0 {
			chunks = []string{strings.TrimSpace(n.Body)}
			if chunks[0] == "" {
				continue
			}
		}
		for i, c := range chunks {
			res, err := chunkStmt.ExecContext(ctx, n.ID, i, c)
			if err != nil {
				return stats, fmt.Errorf("%w: insert chunk %s/%d: %v", ErrIndex, n.ID, i, err)
			}
			rowid, err := res.LastInsertId()
			if err != nil {
				return stats, fmt.Errorf("%w: chunk lastid %s/%d: %v", ErrIndex, n.ID, i, err)
			}
			if _, err := chunksFTSStmt.ExecContext(ctx, rowid, c); err != nil {
				return stats, fmt.Errorf("%w: insert chunks_fts %s/%d: %v", ErrIndex, n.ID, i, err)
			}
			stats.Chunks++
		}
	}

	return stats, nil
}

// resolveLinks runs after all notes are inserted: for each link row whose
// to_note_path matches a known note ID (relative path) or a known note's
// filename stem, populate to_note_id. Multi-match stem cases leave the
// field NULL — broken-link detection / audit-links is responsible for
// reporting those, not the retrieval index.
func resolveLinks(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE links
		SET to_note_id = (
			SELECT id FROM notes WHERE id = links.to_note_path
		)
		WHERE to_note_id IS NULL
	`)
	if err != nil {
		return fmt.Errorf("%w: resolve links by path: %v", ErrIndex, err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE links
		SET to_note_id = (
			SELECT id FROM notes n
			WHERE lower(substr(n.id, instr(n.id, '/') + 1, length(n.id))) = lower(links.to_note_path) || '.md'
			   OR lower(n.id) = lower(links.to_note_path) || '.md'
			LIMIT 1
		)
		WHERE to_note_id IS NULL
		  AND (
		    SELECT count(*) FROM notes n
		    WHERE lower(n.id) LIKE '%/' || lower(links.to_note_path) || '.md'
		       OR lower(n.id) = lower(links.to_note_path) || '.md'
		  ) = 1
	`)
	if err != nil {
		return fmt.Errorf("%w: resolve links by stem: %v", ErrIndex, err)
	}
	return nil
}

// noteBytes reads a file's contents, returning "" on any error. Used for
// yaml-parse of frontmatter inside the indexer — the vault.Walk pass has
// already validated readability and non-emptiness.
func noteBytes(abs string) string {
	b, err := os.ReadFile(abs)
	if err != nil {
		return ""
	}
	return string(b)
}

// stringField returns m[key] coerced to string, or "" if absent / not a
// string-like scalar.
func stringField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	}
	return fmt.Sprint(v)
}

// collectTags merges the walker's space-separated tags string with the
// YAML list form, deduping. Tag values are not lowercased here — that's
// a downstream concern.
func collectTags(spaceSeparated string, yamlTags []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, t := range strings.Fields(spaceSeparated) {
		if t != "" {
			out[t] = struct{}{}
		}
	}
	for _, t := range yamlTags {
		t = strings.TrimSpace(t)
		if t != "" {
			out[t] = struct{}{}
		}
	}
	return out
}
