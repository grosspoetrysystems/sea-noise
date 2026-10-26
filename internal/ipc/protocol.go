// Package ipc is the Unix-socket JSON-RPC surface that lets local
// agents (and humans) ask the sea-noise daemon to do work. The
// protocol is agent-agnostic: the daemon does not know who is calling
// and the verbs name only what they do, not who needs them.
//
// Wire format is newline-delimited JSON over a Unix domain socket.
// Each request and response is one JSON object terminated by '\n'.
// A single connection may carry many request/response pairs in
// sequence; the server handles them one at a time per connection.
//
// Default socket path is $SEA_NOISE_SOCKET, falling back to
// ~/.sea-noise/daemon.sock. Tests typically inject an ephemeral
// socket under t.TempDir.
package ipc

import "encoding/json"

// Protocol version. Bumped only on breaking wire changes.
const Version = "1"

// Verb names. Keep stable — clients pin against these strings.
const (
	VerbSkillRun        = "skill.run"
	VerbPipelineTrigger = "pipeline.trigger"
	VerbVaultQuery      = "vault.query"
	VerbIndexRefresh    = "index.refresh"
	VerbStatus          = "status"
)

// Request is one client→server call.
type Request struct {
	// ID is echoed back in the matching Response. Clients use it to
	// pair responses on a long-lived connection. Empty is allowed.
	ID string `json:"id,omitempty"`
	// Method is one of the Verb* constants.
	Method string `json:"method"`
	// Params is the verb-specific payload. The dispatcher unmarshals
	// it into the verb's params type.
	Params json.RawMessage `json:"params,omitempty"`
}

// Response is one server→client reply.
type Response struct {
	ID     string          `json:"id,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *ErrorBody      `json:"error,omitempty"`
}

// ErrorBody is the structured error envelope.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error codes. Stable strings — clients may match on these.
const (
	ErrCodeBadRequest      = "bad_request"
	ErrCodeUnknownMethod   = "unknown_method"
	ErrCodeUnknownPipeline = "unknown_pipeline"
	ErrCodeAlreadyRan      = "already_ran"
	ErrCodeUnavailable     = "unavailable"
	ErrCodeInternal        = "internal"
)

// --- Verb params + results ---

// SkillRunParams names a skill and supplies its template data.
type SkillRunParams struct {
	Name string          `json:"name"`
	Data json.RawMessage `json:"data,omitempty"`
}

// SkillRunResult carries the raw, stripped inference output.
type SkillRunResult struct {
	Output string `json:"output"`
}

// PipelineTriggerParams names a pipeline. Marker gating applies
// exactly as for the in-process scheduler — callers wanting to bypass
// can pass Force=true (not yet wired; reserved).
type PipelineTriggerParams struct {
	Name  string `json:"name"`
	Force bool   `json:"force,omitempty"`
}

// PipelineTriggerResult reports whether the pipeline ran or was
// skipped by the marker gate.
type PipelineTriggerResult struct {
	Status  string `json:"status"`            // "ran" | "skipped"
	Skipped bool   `json:"skipped,omitempty"` // true when marker-gated out
}

// VaultQueryParams maps onto retrieval.RetrieveRequest. The router
// resolves Mode=auto when unset.
type VaultQueryParams struct {
	Mode       string   `json:"mode,omitempty"` // "" | "direct" | "fts" | "graph" | "context" | "semantic"
	Query      string   `json:"query,omitempty"`
	Path       string   `json:"path,omitempty"`
	ID         string   `json:"id,omitempty"`
	Seeds      []string `json:"seeds,omitempty"`
	WorkflowID string   `json:"workflow_id,omitempty"`
	Depth      int      `json:"depth,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	NoteTypes  []string `json:"note_types,omitempty"`
	Limit      int      `json:"limit,omitempty"`
}

// VaultQueryHit is one result; mirrors retrieval.Hit.
type VaultQueryHit struct {
	NoteID  string  `json:"note_id"`
	ChunkID int64   `json:"chunk_id,omitempty"`
	Score   float64 `json:"score,omitempty"`
	Snippet string  `json:"snippet,omitempty"`
}

// VaultQueryResult is the response envelope for vault.query.
type VaultQueryResult struct {
	Mode string          `json:"mode"`
	Hits []VaultQueryHit `json:"hits"`
}

// IndexRefreshResult reports the IndexVault stats.
type IndexRefreshResult struct {
	Notes  int `json:"notes"`
	Chunks int `json:"chunks"`
	Links  int `json:"links"`
}

// StatusResult is the response for the status verb.
type StatusResult struct {
	Version   string `json:"version"`
	VaultRoot string `json:"vault_root"`
	UptimeSec int64  `json:"uptime_sec"`
	DryRun    bool   `json:"dry_run,omitempty"`
}
