package retrieval

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"sea-noise/internal/markdown"
	"sea-noise/internal/vault"
	"sea-noise/internal/vernacular"
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
	sources, err := vernacular.Sources(ctx, root)
	if err != nil {
		return IndexStats{}, fmt.Errorf("%w: sources: %v", ErrIndex, err)
	}
	notes, err := vault.WalkSourcesContext(ctx, root, excludeDirs, sources)
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

	stats, err := insertAll(ctx, tx, notes)
	if err != nil {
		return IndexStats{}, err
	}

	if err := resolveLinks(ctx, tx, notes, sources); err != nil {
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

func insertAll(ctx context.Context, tx *sql.Tx, notes []vault.Note) (IndexStats, error) {
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
		abs := n.PhysicalPath
		yfm, _ := markdown.ParseFrontmatterYAML(noteBytes(ctx, abs))

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

// resolveLinks resolves link targets against the notes from this indexing
// pass. Physical mappings are restricted to files already indexed, so an
// overview alias can resolve relative peers without reading arbitrary paths.
func resolveLinks(ctx context.Context, tx *sql.Tx, notes []vault.Note, sources []vault.Source) error {
	resolver := newNoteLinkResolver(notes, sources)
	rows, err := tx.QueryContext(ctx, `SELECT rowid, from_note_id, to_note_path FROM links WHERE to_note_id IS NULL`)
	if err != nil {
		return fmt.Errorf("%w: read links: %v", ErrIndex, err)
	}
	type pending struct {
		rowid  int64
		target string
	}
	var updates []pending
	for rows.Next() {
		var rowid int64
		var fromID, rawTarget string
		if err := rows.Scan(&rowid, &fromID, &rawTarget); err != nil {
			_ = rows.Close()
			return fmt.Errorf("%w: scan link: %v", ErrIndex, err)
		}
		fromPhysical, ok := resolver.notes[fromID]
		if !ok {
			continue
		}
		if target, ok := resolver.resolve(fromID, fromPhysical, rawTarget); ok {
			updates = append(updates, pending{rowid: rowid, target: target})
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("%w: read links: %v", ErrIndex, err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("%w: close links: %v", ErrIndex, err)
	}
	for _, update := range updates {
		if _, err := tx.ExecContext(ctx, `UPDATE links SET to_note_id = ? WHERE rowid = ?`, update.target, update.rowid); err != nil {
			return fmt.Errorf("%w: resolve link: %v", ErrIndex, err)
		}
	}
	return nil
}

type noteLinkResolver struct {
	notes map[string]string
	// Candidate indexes use an empty value for ambiguous keys.
	ids      map[string]string
	names    map[string]string
	physical map[string]string
}

func newNoteLinkResolver(notes []vault.Note, sources []vault.Source) *noteLinkResolver {
	resolver := &noteLinkResolver{
		notes:    make(map[string]string, len(notes)),
		ids:      make(map[string]string, len(notes)),
		names:    make(map[string]string, len(notes)),
		physical: make(map[string]string, len(notes)),
	}
	for _, note := range notes {
		resolver.notes[note.ID] = note.PhysicalPath
		id := filepath.ToSlash(note.ID)
		addLinkCandidate(resolver.ids, strings.ToLower(id), note.ID)
		addLinkCandidate(resolver.names, strings.ToLower(path.Base(id)), note.ID)
		if note.PhysicalPath != "" {
			addLinkCandidate(resolver.physical, note.PhysicalPath, note.ID)
			addSourceAliases(resolver, note.ID, note.PhysicalPath, sources)
		}
	}
	return resolver
}

func addSourceAliases(resolver *noteLinkResolver, noteID, notePath string, sources []vault.Source) {
	// Sources and note paths are already canonical from the validated walk.
	for _, source := range sources {
		logicalRoot := source.LogicalRoot
		if logicalRoot == "" {
			continue
		}
		sourcePath := source.PhysicalRoot
		if notePath == sourcePath {
			addLinkCandidate(resolver.ids, strings.ToLower(logicalRoot), noteID)
			continue
		}
		rel, err := filepath.Rel(sourcePath, notePath)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			continue
		}
		alias := path.Join(logicalRoot, filepath.ToSlash(rel))
		addLinkCandidate(resolver.ids, strings.ToLower(alias), noteID)
	}
}

func (r *noteLinkResolver) resolve(fromID, fromPhysical, raw string) (string, bool) {
	target := normalizeLinkTarget(raw)
	if target == "" {
		return "", false
	}
	hasPath := strings.Contains(target, "/")
	if hasPath {
		if id := r.ids[strings.ToLower(target)]; id != "" {
			return id, true
		}
	}
	if fromPhysical != "" {
		candidate := filepath.Join(filepath.Dir(fromPhysical), filepath.FromSlash(target))
		if id := r.physical[candidate]; id != "" {
			return id, true
		}
		if resolved := physicalKey(candidate); resolved != candidate {
			if id := r.physical[resolved]; id != "" {
				return id, true
			}
		}
	}
	logical := path.Clean(path.Join(path.Dir(filepath.ToSlash(fromID)), target))
	if logical != "." && logical != ".." && !strings.HasPrefix(logical, "../") {
		if id := r.ids[strings.ToLower(logical)]; id != "" {
			return id, true
		}
	}
	if !strings.Contains(target, "/") {
		if id := r.names[strings.ToLower(path.Base(target))]; id != "" {
			return id, true
		}
	}
	return "", false
}

func normalizeLinkTarget(raw string) string {
	target := strings.TrimSpace(raw)
	if target == "" || strings.Contains(target, "\\") || path.IsAbs(target) {
		return ""
	}
	target = path.Clean(target)
	if target == "." || target == ".." {
		return ""
	}
	if !strings.HasSuffix(strings.ToLower(target), ".md") {
		target += ".md"
	}
	return target
}

func physicalKey(name string) string {
	abs, err := filepath.Abs(name)
	if err != nil {
		return filepath.Clean(name)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(abs)
}

func addLinkCandidate(index map[string]string, key, id string) {
	if previous, exists := index[key]; !exists {
		index[key] = id
	} else if previous != id {
		index[key] = ""
	}
}

// noteBytes reads a file's contents, returning "" on any error. Used for
// yaml-parse of frontmatter inside the indexer — the vault.Walk pass has
// already validated readability and non-emptiness.
func noteBytes(ctx context.Context, abs string) string {
	b, err := vault.ReadFileRetryContext(ctx, abs)
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
