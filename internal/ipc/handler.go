package ipc

import (
	"context"
	"encoding/json"
	"errors"
)

// Handler is the verb-implementation seam. Callers (typically
// cmd/sea-noise) build a concrete Handler that closes over the
// daemon.Server, retrieval router, skill executor, etc. The IPC
// Server depends only on this interface, so the daemon package and
// workflow packages stay free of ipc imports.
type Handler interface {
	SkillRun(ctx context.Context, params SkillRunParams) (SkillRunResult, error)
	PipelineTrigger(ctx context.Context, params PipelineTriggerParams) (PipelineTriggerResult, error)
	VaultQuery(ctx context.Context, params VaultQueryParams) (VaultQueryResult, error)
	IndexRefresh(ctx context.Context) (IndexRefreshResult, error)
	Status(ctx context.Context) (StatusResult, error)
}

// Sentinel errors a Handler implementation may return; the dispatcher
// maps these to stable wire ErrorBody.Code values. Anything else is
// surfaced as ErrCodeInternal with the error's message.
var (
	// ErrBadRequest signals malformed verb params.
	ErrBadRequest = errors.New("ipc: bad request")
	// ErrUnknownPipeline signals an unrecognized pipeline name.
	ErrUnknownPipeline = errors.New("ipc: unknown pipeline")
	// ErrAlreadyRan signals a marker-gated skip on pipeline.trigger.
	ErrAlreadyRan = errors.New("ipc: pipeline already ran")
	// ErrUnavailable signals a dependency the handler needs is not
	// configured (e.g. no skill executor when --cli-command is unset).
	ErrUnavailable = errors.New("ipc: unavailable")
)

// errorCode maps a handler-returned error to its stable wire code.
func errorCode(err error) string {
	switch {
	case errors.Is(err, ErrBadRequest):
		return ErrCodeBadRequest
	case errors.Is(err, ErrUnknownPipeline):
		return ErrCodeUnknownPipeline
	case errors.Is(err, ErrAlreadyRan):
		return ErrCodeAlreadyRan
	case errors.Is(err, ErrUnavailable):
		return ErrCodeUnavailable
	default:
		return ErrCodeInternal
	}
}

// unmarshalParams decodes a Request.Params blob into v. Empty params
// is treated as the zero value of v. Decode failures map to
// ErrBadRequest.
func unmarshalParams(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return errors.Join(ErrBadRequest, err)
	}
	return nil
}
