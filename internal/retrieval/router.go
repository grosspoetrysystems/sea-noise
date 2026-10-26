package retrieval

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Mode names the retrieval strategy the router should use. ModeAuto asks
// the router to pick one from the request shape (see ARCHITECTURE.md §4).
type Mode int

const (
	ModeAuto Mode = iota
	ModeDirectLookup
	ModeFTSSearch
	ModeGraphExpansion
	ModeDeterministicContextBuild
	ModeOptionalSemanticExpansion
)

// String returns the canonical mode name.
func (m Mode) String() string {
	switch m {
	case ModeAuto:
		return "Auto"
	case ModeDirectLookup:
		return "DirectLookup"
	case ModeFTSSearch:
		return "FTSSearch"
	case ModeGraphExpansion:
		return "GraphExpansion"
	case ModeDeterministicContextBuild:
		return "DeterministicContextBuild"
	case ModeOptionalSemanticExpansion:
		return "OptionalSemanticExpansion"
	}
	return fmt.Sprintf("Mode(%d)", int(m))
}

// RetrieveRequest carries query parameters and filters.
type RetrieveRequest struct {
	// Mode selects the retrieval strategy. ModeAuto lets the router pick.
	Mode Mode

	// Query is the keyword / phrase for FTSSearch (and the input to
	// OptionalSemanticExpansion reranking). Empty for path-style modes.
	Query string

	// Path or ID directly resolves a note for ModeDirectLookup. Either
	// field works — Path is intended as the human-typed form, ID as the
	// internal relative-path identifier (today they coincide).
	Path string
	ID   string

	// Seeds is the starting note set for ModeGraphExpansion.
	Seeds []string

	// WorkflowID names a deterministic context builder for
	// ModeDeterministicContextBuild.
	WorkflowID string

	// Depth is the BFS depth cap for ModeGraphExpansion. Zero means the
	// router default (1 — direct neighbors only). GraphExpansion follows
	// both outgoing wikilinks and backlinks per ARCHITECTURE §4.
	Depth int

	// Filters: a result must satisfy all non-empty filters.
	Tags      []string   // any-of
	NoteTypes []string   // any-of
	Since     *time.Time // mtime >= Since
	Until     *time.Time // mtime <= Until

	// Limit is the max number of results to return. Zero is treated as
	// the router's default (currently 20).
	Limit int
}

// RetrieveResponse carries result identifiers and minimal provenance.
type RetrieveResponse struct {
	// Mode is the strategy that actually ran. Set even when the request
	// was ModeAuto.
	Mode Mode

	// Hits are the result notes/chunks in ranked order (best first).
	Hits []Hit
}

// Hit is a single retrieval result.
type Hit struct {
	// NoteID is the note's relative-path identifier.
	NoteID string

	// ChunkID is the chunk ID when retrieval was sub-note; 0 when the
	// hit is the whole note.
	ChunkID int64

	// Score is the mode-specific ranking score. For FTSSearch this is
	// BM25 (lower = better). Zero when not applicable.
	Score float64

	// Snippet is an optional preview, populated for FTSSearch.
	Snippet string
}

// Router is the single entry point for any caller that needs notes or
// chunks. The router does not orchestrate — it picks a mode and
// dispatches. It never invokes an LLM.
type Router interface {
	Retrieve(ctx context.Context, req RetrieveRequest) (RetrieveResponse, error)
}

// Sentinel errors. Wrap with %w; check with errors.Is.
var (
	ErrUnsupportedMode = errors.New("retrieval: unsupported mode")
	ErrEmptyRequest    = errors.New("retrieval: request has no usable inputs")
	ErrRetrieve        = errors.New("retrieval: query failed")
)

// SQLiteRouter is the SQLite-backed Router. It implements ModeDirectLookup
// and ModeFTSSearch in this wave. Other modes return ErrUnsupportedMode
// until they land in subsequent waves.
type SQLiteRouter struct {
	DB *sql.DB

	// DefaultLimit is the limit applied when RetrieveRequest.Limit == 0.
	// Defaults to 20 when zero.
	DefaultLimit int
}

// NewSQLiteRouter is a convenience constructor.
func NewSQLiteRouter(db *sql.DB) *SQLiteRouter {
	return &SQLiteRouter{DB: db, DefaultLimit: 20}
}

// Retrieve dispatches the request to the chosen mode.
func (r *SQLiteRouter) Retrieve(ctx context.Context, req RetrieveRequest) (RetrieveResponse, error) {
	mode := req.Mode
	if mode == ModeAuto {
		var err error
		mode, err = r.resolveAuto(req)
		if err != nil {
			return RetrieveResponse{}, err
		}
	}
	limit := req.Limit
	if limit <= 0 {
		if r.DefaultLimit <= 0 {
			limit = 20
		} else {
			limit = r.DefaultLimit
		}
	}

	switch mode {
	case ModeDirectLookup:
		return r.directLookup(ctx, req)
	case ModeFTSSearch:
		return r.ftsSearch(ctx, req, limit)
	case ModeGraphExpansion:
		return r.graphExpansion(ctx, req, limit)
	case ModeDeterministicContextBuild, ModeOptionalSemanticExpansion:
		return RetrieveResponse{Mode: mode}, fmt.Errorf("%w: %s", ErrUnsupportedMode, mode)
	}
	return RetrieveResponse{Mode: mode}, fmt.Errorf("%w: %s", ErrUnsupportedMode, mode)
}

// resolveAuto applies the precedence rules from ARCHITECTURE.md §4 for
// ModeAuto: ID/Path > WorkflowID > Seeds > Query.
func (r *SQLiteRouter) resolveAuto(req RetrieveRequest) (Mode, error) {
	switch {
	case req.ID != "" || req.Path != "":
		return ModeDirectLookup, nil
	case req.WorkflowID != "":
		return ModeDeterministicContextBuild, nil
	case len(req.Seeds) > 0 && req.Query == "":
		return ModeGraphExpansion, nil
	case req.Query != "":
		return ModeFTSSearch, nil
	}
	return ModeAuto, ErrEmptyRequest
}

func (r *SQLiteRouter) directLookup(ctx context.Context, req RetrieveRequest) (RetrieveResponse, error) {
	key := req.ID
	if key == "" {
		key = req.Path
	}
	if key == "" {
		return RetrieveResponse{Mode: ModeDirectLookup}, ErrEmptyRequest
	}
	row := r.DB.QueryRowContext(ctx, `SELECT id FROM notes WHERE id = ?`, key)
	var id string
	if err := row.Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RetrieveResponse{Mode: ModeDirectLookup}, nil
		}
		return RetrieveResponse{Mode: ModeDirectLookup}, fmt.Errorf("%w: %v", ErrRetrieve, err)
	}
	return RetrieveResponse{
		Mode: ModeDirectLookup,
		Hits: []Hit{{NoteID: id}},
	}, nil
}

func (r *SQLiteRouter) ftsSearch(ctx context.Context, req RetrieveRequest, limit int) (RetrieveResponse, error) {
	if strings.TrimSpace(req.Query) == "" {
		return RetrieveResponse{Mode: ModeFTSSearch}, ErrEmptyRequest
	}

	// Build a filterable query. The chunk-level FTS5 match is always
	// applied; tag/type/time filters narrow the candidate notes via
	// correlated EXISTS clauses and a notes JOIN.
	var sb strings.Builder
	sb.WriteString(`
		SELECT c.note_id, c.id, bm25(chunks_fts) AS score,
		       snippet(chunks_fts, 0, '«', '»', '…', 16) AS snip
		FROM chunks_fts
		JOIN chunks c ON c.id = chunks_fts.rowid
		JOIN notes  n ON n.id = c.note_id
		WHERE chunks_fts MATCH ?`)
	args := []any{req.Query}

	if len(req.Tags) > 0 {
		sb.WriteString(` AND EXISTS (SELECT 1 FROM tags t WHERE t.note_id = n.id AND t.tag IN (`)
		sb.WriteString(placeholders(len(req.Tags)))
		sb.WriteString(`))`)
		for _, t := range req.Tags {
			args = append(args, t)
		}
	}
	if len(req.NoteTypes) > 0 {
		sb.WriteString(` AND n.type IN (`)
		sb.WriteString(placeholders(len(req.NoteTypes)))
		sb.WriteString(`)`)
		for _, t := range req.NoteTypes {
			args = append(args, t)
		}
	}
	if req.Since != nil {
		sb.WriteString(` AND n.mtime IS NOT NULL AND n.mtime >= ?`)
		args = append(args, req.Since.Unix())
	}
	if req.Until != nil {
		sb.WriteString(` AND n.mtime IS NOT NULL AND n.mtime <= ?`)
		args = append(args, req.Until.Unix())
	}

	sb.WriteString(` ORDER BY score ASC LIMIT ?`)
	args = append(args, limit)

	rows, err := r.DB.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return RetrieveResponse{Mode: ModeFTSSearch}, fmt.Errorf("%w: %v", ErrRetrieve, err)
	}
	defer rows.Close()

	resp := RetrieveResponse{Mode: ModeFTSSearch}
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.NoteID, &h.ChunkID, &h.Score, &h.Snippet); err != nil {
			return RetrieveResponse{Mode: ModeFTSSearch}, fmt.Errorf("%w: scan: %v", ErrRetrieve, err)
		}
		resp.Hits = append(resp.Hits, h)
	}
	if err := rows.Err(); err != nil {
		return RetrieveResponse{Mode: ModeFTSSearch}, fmt.Errorf("%w: rows: %v", ErrRetrieve, err)
	}
	return resp, nil
}

// graphExpansion performs a BFS from req.Seeds over the links table,
// following both outgoing wikilinks (links.from_note_id → links.to_note_id)
// and backlinks (links.to_note_id → links.from_note_id). Depth defaults
// to 1 (direct neighbors only). Seeds themselves are not returned; only
// reachable notes.
//
// Note filtering (Tags, NoteTypes, Since, Until) is applied to the
// expanded set before truncating to Limit.
func (r *SQLiteRouter) graphExpansion(ctx context.Context, req RetrieveRequest, limit int) (RetrieveResponse, error) {
	if len(req.Seeds) == 0 {
		return RetrieveResponse{Mode: ModeGraphExpansion}, ErrEmptyRequest
	}
	depth := req.Depth
	if depth <= 0 {
		depth = 1
	}

	visited := map[string]struct{}{}
	for _, s := range req.Seeds {
		visited[s] = struct{}{}
	}
	frontier := append([]string(nil), req.Seeds...)
	var ordered []string // newly-discovered notes in BFS order

	for d := 0; d < depth && len(frontier) > 0; d++ {
		next, err := r.neighborsBidirectional(ctx, frontier)
		if err != nil {
			return RetrieveResponse{Mode: ModeGraphExpansion}, err
		}
		var newFrontier []string
		for _, n := range next {
			if _, seen := visited[n]; seen {
				continue
			}
			visited[n] = struct{}{}
			ordered = append(ordered, n)
			newFrontier = append(newFrontier, n)
		}
		frontier = newFrontier
	}

	if len(ordered) == 0 {
		return RetrieveResponse{Mode: ModeGraphExpansion}, nil
	}

	filtered, err := r.applyNoteFilters(ctx, ordered, req)
	if err != nil {
		return RetrieveResponse{Mode: ModeGraphExpansion}, err
	}
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	resp := RetrieveResponse{Mode: ModeGraphExpansion, Hits: make([]Hit, 0, len(filtered))}
	for _, id := range filtered {
		resp.Hits = append(resp.Hits, Hit{NoteID: id})
	}
	return resp, nil
}

// neighborsBidirectional returns the set of note IDs reachable from any
// of the given seed IDs in one hop, via outgoing wikilinks OR backlinks.
// Order within the returned slice is by note ID for determinism.
func (r *SQLiteRouter) neighborsBidirectional(ctx context.Context, seeds []string) ([]string, error) {
	if len(seeds) == 0 {
		return nil, nil
	}
	ph := placeholders(len(seeds))
	q := `
		SELECT DISTINCT to_note_id
		FROM links
		WHERE to_note_id IS NOT NULL AND from_note_id IN (` + ph + `)
		UNION
		SELECT DISTINCT from_note_id
		FROM links
		WHERE to_note_id IN (` + ph + `)
		ORDER BY 1`
	args := make([]any, 0, len(seeds)*2)
	for _, s := range seeds {
		args = append(args, s)
	}
	for _, s := range seeds {
		args = append(args, s)
	}
	rows, err := r.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("%w: neighbors: %v", ErrRetrieve, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("%w: neighbors scan: %v", ErrRetrieve, err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: neighbors rows: %v", ErrRetrieve, err)
	}
	return out, nil
}

// applyNoteFilters returns the subset of ids whose notes satisfy the
// request's Tags / NoteTypes / Since / Until filters. Input order is
// preserved (BFS order from the caller).
func (r *SQLiteRouter) applyNoteFilters(ctx context.Context, ids []string, req RetrieveRequest) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if len(req.Tags) == 0 && len(req.NoteTypes) == 0 && req.Since == nil && req.Until == nil {
		return ids, nil
	}

	var sb strings.Builder
	sb.WriteString(`SELECT n.id FROM notes n WHERE n.id IN (`)
	sb.WriteString(placeholders(len(ids)))
	sb.WriteString(`)`)
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	if len(req.Tags) > 0 {
		sb.WriteString(` AND EXISTS (SELECT 1 FROM tags t WHERE t.note_id = n.id AND t.tag IN (`)
		sb.WriteString(placeholders(len(req.Tags)))
		sb.WriteString(`))`)
		for _, t := range req.Tags {
			args = append(args, t)
		}
	}
	if len(req.NoteTypes) > 0 {
		sb.WriteString(` AND n.type IN (`)
		sb.WriteString(placeholders(len(req.NoteTypes)))
		sb.WriteString(`)`)
		for _, t := range req.NoteTypes {
			args = append(args, t)
		}
	}
	if req.Since != nil {
		sb.WriteString(` AND n.mtime IS NOT NULL AND n.mtime >= ?`)
		args = append(args, req.Since.Unix())
	}
	if req.Until != nil {
		sb.WriteString(` AND n.mtime IS NOT NULL AND n.mtime <= ?`)
		args = append(args, req.Until.Unix())
	}

	rows, err := r.DB.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("%w: filter: %v", ErrRetrieve, err)
	}
	defer rows.Close()
	keep := map[string]struct{}{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("%w: filter scan: %v", ErrRetrieve, err)
		}
		keep[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: filter rows: %v", ErrRetrieve, err)
	}

	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := keep[id]; ok {
			out = append(out, id)
		}
	}
	return out, nil
}

// placeholders returns "?, ?, ..." with n placeholders.
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	var sb strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("?")
	}
	return sb.String()
}
