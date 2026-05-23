package retrieval

import (
	"context"
	"database/sql"
	"fmt"
)

// IndexDump is a deterministic snapshot of the retrieval index. Used by
// the golden-file regression test: the same vault should yield the same
// dump byte-for-byte across runs.
//
// Mtime values and "mtime" timestamps are intentionally excluded from
// the dump because file modification times are not hermetic across
// machines or fresh checkouts. Frontmatter-derived timestamps (created,
// updated, date) ARE included since those are content-derived.
type IndexDump struct {
	Notes      []NoteRow      `json:"notes"`
	Chunks     []ChunkRow     `json:"chunks"`
	Tags       []TagRow       `json:"tags"`
	Links      []LinkRow      `json:"links"`
	Entities   []EntityRow    `json:"entities"`
	Assertions []AssertionRow `json:"assertions"`
	Timestamps []TimestampRow `json:"timestamps"`
}

// NoteRow is the dump form of a notes row. The mtime column is excluded
// (non-hermetic); frontmatter_json is included as a string verbatim so
// the dump is faithful to what the indexer wrote.
type NoteRow struct {
	ID              string `json:"id"`
	Path            string `json:"path"`
	Type            string `json:"type"`
	Title           string `json:"title"`
	Tags            string `json:"tags"`
	FrontmatterJSON string `json:"frontmatter_json"`
}

// ChunkRow is the dump form of a chunks row. The auto-increment id is
// excluded; ordinal + body is what matters for regression.
type ChunkRow struct {
	NoteID  string `json:"note_id"`
	Ordinal int    `json:"ordinal"`
	Body    string `json:"body"`
}

// TagRow is one (note_id, tag) pair.
type TagRow struct {
	NoteID string `json:"note_id"`
	Tag    string `json:"tag"`
}

// LinkRow is one wikilink with its resolution status. The auto-increment
// id is excluded.
type LinkRow struct {
	FromNoteID string `json:"from_note_id"`
	ToNotePath string `json:"to_note_path"`
	ToNoteID   string `json:"to_note_id,omitempty"`
}

// EntityRow / AssertionRow / TimestampRow follow the same pattern.
type EntityRow struct {
	NoteID        string `json:"note_id"`
	Surface       string `json:"surface"`
	CanonicalForm string `json:"canonical_form"`
}

type AssertionRow struct {
	NoteID     string   `json:"note_id"`
	Statement  string   `json:"statement"`
	Confidence *float64 `json:"confidence,omitempty"`
}

type TimestampRow struct {
	NoteID string `json:"note_id"`
	Kind   string `json:"kind"`
	Value  string `json:"value"`
}

// DumpIndex reads the entire retrieval index and returns a deterministic
// snapshot suitable for golden-file comparison. Rows are sorted by their
// natural keys; "mtime" timestamps are excluded.
func DumpIndex(ctx context.Context, db *sql.DB) (IndexDump, error) {
	d := IndexDump{}

	if err := dumpNotes(ctx, db, &d); err != nil {
		return IndexDump{}, err
	}
	if err := dumpChunks(ctx, db, &d); err != nil {
		return IndexDump{}, err
	}
	if err := dumpTags(ctx, db, &d); err != nil {
		return IndexDump{}, err
	}
	if err := dumpLinks(ctx, db, &d); err != nil {
		return IndexDump{}, err
	}
	if err := dumpEntities(ctx, db, &d); err != nil {
		return IndexDump{}, err
	}
	if err := dumpAssertions(ctx, db, &d); err != nil {
		return IndexDump{}, err
	}
	if err := dumpTimestamps(ctx, db, &d); err != nil {
		return IndexDump{}, err
	}
	return d, nil
}

func dumpNotes(ctx context.Context, db *sql.DB, d *IndexDump) error {
	rows, err := db.QueryContext(ctx, `
		SELECT id, path, COALESCE(type, ''), title, tags, frontmatter_json
		FROM notes ORDER BY id
	`)
	if err != nil {
		return fmt.Errorf("dump notes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r NoteRow
		if err := rows.Scan(&r.ID, &r.Path, &r.Type, &r.Title, &r.Tags, &r.FrontmatterJSON); err != nil {
			return fmt.Errorf("dump notes scan: %w", err)
		}
		d.Notes = append(d.Notes, r)
	}
	return rows.Err()
}

func dumpChunks(ctx context.Context, db *sql.DB, d *IndexDump) error {
	rows, err := db.QueryContext(ctx, `
		SELECT note_id, ordinal, body
		FROM chunks ORDER BY note_id, ordinal
	`)
	if err != nil {
		return fmt.Errorf("dump chunks: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r ChunkRow
		if err := rows.Scan(&r.NoteID, &r.Ordinal, &r.Body); err != nil {
			return fmt.Errorf("dump chunks scan: %w", err)
		}
		d.Chunks = append(d.Chunks, r)
	}
	return rows.Err()
}

func dumpTags(ctx context.Context, db *sql.DB, d *IndexDump) error {
	rows, err := db.QueryContext(ctx, `SELECT note_id, tag FROM tags ORDER BY note_id, tag`)
	if err != nil {
		return fmt.Errorf("dump tags: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r TagRow
		if err := rows.Scan(&r.NoteID, &r.Tag); err != nil {
			return fmt.Errorf("dump tags scan: %w", err)
		}
		d.Tags = append(d.Tags, r)
	}
	return rows.Err()
}

func dumpLinks(ctx context.Context, db *sql.DB, d *IndexDump) error {
	rows, err := db.QueryContext(ctx, `
		SELECT from_note_id, to_note_path, COALESCE(to_note_id, '')
		FROM links ORDER BY from_note_id, to_note_path, COALESCE(to_note_id, '')
	`)
	if err != nil {
		return fmt.Errorf("dump links: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r LinkRow
		if err := rows.Scan(&r.FromNoteID, &r.ToNotePath, &r.ToNoteID); err != nil {
			return fmt.Errorf("dump links scan: %w", err)
		}
		d.Links = append(d.Links, r)
	}
	return rows.Err()
}

func dumpEntities(ctx context.Context, db *sql.DB, d *IndexDump) error {
	rows, err := db.QueryContext(ctx, `
		SELECT note_id, surface, canonical_form
		FROM entities ORDER BY note_id, surface, canonical_form
	`)
	if err != nil {
		return fmt.Errorf("dump entities: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r EntityRow
		if err := rows.Scan(&r.NoteID, &r.Surface, &r.CanonicalForm); err != nil {
			return fmt.Errorf("dump entities scan: %w", err)
		}
		d.Entities = append(d.Entities, r)
	}
	return rows.Err()
}

func dumpAssertions(ctx context.Context, db *sql.DB, d *IndexDump) error {
	rows, err := db.QueryContext(ctx, `
		SELECT note_id, statement, confidence
		FROM assertions ORDER BY note_id, statement
	`)
	if err != nil {
		return fmt.Errorf("dump assertions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r AssertionRow
		var conf sql.NullFloat64
		if err := rows.Scan(&r.NoteID, &r.Statement, &conf); err != nil {
			return fmt.Errorf("dump assertions scan: %w", err)
		}
		if conf.Valid {
			c := conf.Float64
			r.Confidence = &c
		}
		d.Assertions = append(d.Assertions, r)
	}
	return rows.Err()
}

func dumpTimestamps(ctx context.Context, db *sql.DB, d *IndexDump) error {
	// Exclude kind='mtime': file modification times are not hermetic
	// across machines / fresh checkouts / Git checkouts.
	rows, err := db.QueryContext(ctx, `
		SELECT note_id, kind, value
		FROM timestamps WHERE kind != 'mtime' ORDER BY note_id, kind, value
	`)
	if err != nil {
		return fmt.Errorf("dump timestamps: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r TimestampRow
		if err := rows.Scan(&r.NoteID, &r.Kind, &r.Value); err != nil {
			return fmt.Errorf("dump timestamps scan: %w", err)
		}
		d.Timestamps = append(d.Timestamps, r)
	}
	return rows.Err()
}
