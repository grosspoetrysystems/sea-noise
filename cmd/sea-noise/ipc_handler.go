package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"sea-noise/internal/ai/skill"
	"sea-noise/internal/daemon"
	"sea-noise/internal/ipc"
	"sea-noise/internal/retrieval"
)

// ipcHandler bridges the agent-agnostic ipc.Handler interface to the
// daemon.Server, the skill executor, and the retrieval router. It
// holds no state of its own — the Server, executor, vault root, and
// index DB path are wired in at construction.
//
// The handler stays in cmd/sea-noise rather than internal/ipc so the
// ipc package keeps no imports of workflow or daemon code. That
// preserves the "daemon does not know who is calling" property —
// internal/ipc handles transport and dispatch only.
type ipcHandler struct {
	srv         *daemon.Server
	exec        *skill.Executor // may be nil — handler returns ipc.ErrUnavailable
	vaultRoot   string
	indexDBPath string
	startedAt   time.Time
	dryRun      bool
}

func newIPCHandler(srv *daemon.Server, exec *skill.Executor, vaultRoot, indexDBPath string, dryRun bool) *ipcHandler {
	return &ipcHandler{
		srv:         srv,
		exec:        exec,
		vaultRoot:   vaultRoot,
		indexDBPath: indexDBPath,
		startedAt:   time.Now(),
		dryRun:      dryRun,
	}
}

// SkillRun renders + dispatches a skill via the executor. The verb is
// agent-agnostic: the caller names a skill, the executor handles
// routing + transport.
func (h *ipcHandler) SkillRun(ctx context.Context, params ipc.SkillRunParams) (ipc.SkillRunResult, error) {
	if h.exec == nil {
		return ipc.SkillRunResult{}, fmt.Errorf("%w: skill executor not configured (run with --cli-command)", ipc.ErrUnavailable)
	}
	if params.Name == "" {
		return ipc.SkillRunResult{}, fmt.Errorf("%w: skill name required", ipc.ErrBadRequest)
	}
	out, err := h.exec.Execute(ctx, params.Name, params.Data)
	if err != nil {
		return ipc.SkillRunResult{}, err
	}
	return ipc.SkillRunResult{Output: out}, nil
}

// PipelineTrigger fires one named pipeline through the daemon. Marker
// gating applies exactly as for the scheduled tick. ErrSkippedAlreadyRan
// from the daemon maps to ipc.ErrAlreadyRan on the wire.
func (h *ipcHandler) PipelineTrigger(ctx context.Context, params ipc.PipelineTriggerParams) (ipc.PipelineTriggerResult, error) {
	if params.Name == "" {
		return ipc.PipelineTriggerResult{}, fmt.Errorf("%w: pipeline name required", ipc.ErrBadRequest)
	}
	err := h.srv.RunPipeline(ctx, params.Name, time.Now().UTC())
	switch {
	case err == nil:
		return ipc.PipelineTriggerResult{Status: "ran"}, nil
	case errors.Is(err, daemon.ErrSkippedAlreadyRan):
		return ipc.PipelineTriggerResult{Status: "skipped", Skipped: true}, fmt.Errorf("%w: %s", ipc.ErrAlreadyRan, params.Name)
	case errors.Is(err, daemon.ErrUnknownPipeline):
		return ipc.PipelineTriggerResult{}, fmt.Errorf("%w: %s", ipc.ErrUnknownPipeline, params.Name)
	default:
		return ipc.PipelineTriggerResult{}, err
	}
}

// VaultQuery opens the retrieval DB on demand and dispatches the
// request to the SQLite router. The DB connection is short-lived per
// call so a long-running daemon does not pin sqlite handles when
// callers are idle.
func (h *ipcHandler) VaultQuery(ctx context.Context, params ipc.VaultQueryParams) (ipc.VaultQueryResult, error) {
	mode, err := parseQueryMode(params.Mode)
	if err != nil {
		return ipc.VaultQueryResult{}, err
	}
	db, err := sql.Open("sqlite", h.indexDBPath)
	if err != nil {
		return ipc.VaultQueryResult{}, fmt.Errorf("open index: %w", err)
	}
	defer db.Close()

	router := retrieval.NewSQLiteRouter(db)
	resp, err := router.Retrieve(ctx, retrieval.RetrieveRequest{
		Mode:       mode,
		Query:      params.Query,
		Path:       params.Path,
		ID:         params.ID,
		Seeds:      params.Seeds,
		WorkflowID: params.WorkflowID,
		Depth:      params.Depth,
		Tags:       params.Tags,
		NoteTypes:  params.NoteTypes,
		Limit:      params.Limit,
	})
	if err != nil {
		return ipc.VaultQueryResult{}, err
	}
	hits := make([]ipc.VaultQueryHit, 0, len(resp.Hits))
	for _, h := range resp.Hits {
		hits = append(hits, ipc.VaultQueryHit{
			NoteID:  h.NoteID,
			ChunkID: h.ChunkID,
			Score:   h.Score,
			Snippet: h.Snippet,
		})
	}
	return ipc.VaultQueryResult{Mode: resp.Mode.String(), Hits: hits}, nil
}

// IndexRefresh re-indexes the vault. Migrate is idempotent; IndexVault
// returns counts the wire surfaces as IndexRefreshResult.
func (h *ipcHandler) IndexRefresh(ctx context.Context) (ipc.IndexRefreshResult, error) {
	db, err := sql.Open("sqlite", h.indexDBPath)
	if err != nil {
		return ipc.IndexRefreshResult{}, fmt.Errorf("open index: %w", err)
	}
	defer db.Close()
	if err := retrieval.Migrate(ctx, db); err != nil {
		return ipc.IndexRefreshResult{}, err
	}
	stats, err := retrieval.IndexVault(ctx, db, h.vaultRoot, nil)
	if err != nil {
		return ipc.IndexRefreshResult{}, err
	}
	return ipc.IndexRefreshResult{
		Notes:  stats.Notes,
		Chunks: stats.Chunks,
		Links:  stats.Links,
	}, nil
}

// Status reports a small heartbeat envelope for clients to verify
// the daemon is alive and which vault it owns.
func (h *ipcHandler) Status(_ context.Context) (ipc.StatusResult, error) {
	return ipc.StatusResult{
		Version:   ipc.Version,
		VaultRoot: h.vaultRoot,
		UptimeSec: int64(time.Since(h.startedAt).Seconds()),
		DryRun:    h.dryRun,
	}, nil
}

// parseQueryMode maps the wire string to retrieval.Mode. Empty falls
// back to ModeAuto so callers get router-picked behavior.
func parseQueryMode(s string) (retrieval.Mode, error) {
	switch s {
	case "", "auto":
		return retrieval.ModeAuto, nil
	case "direct":
		return retrieval.ModeDirectLookup, nil
	case "fts":
		return retrieval.ModeFTSSearch, nil
	case "graph":
		return retrieval.ModeGraphExpansion, nil
	case "context":
		return retrieval.ModeDeterministicContextBuild, nil
	case "semantic":
		return retrieval.ModeOptionalSemanticExpansion, nil
	default:
		return retrieval.ModeAuto, fmt.Errorf("%w: unknown mode %q", ipc.ErrBadRequest, s)
	}
}
