// Package transport invokes LLM transports — currently just the local
// `claude -p` CLI. AI command wrappers obtain an InferenceDecision from
// the inference router (see ../inferencerouter) and pass it to a
// Transport. The Transport ignores the decision today (LocalRouter
// returns the same default for everything and `claude -p` is invoked
// the same way regardless), but the contract is in place so future
// transports can route by decision.
//
// This package is the only place in the runtime that talks to an LLM.
// Tests of any caller MUST use MockTransport, not ClaudeTransport —
// the real transport spends tokens and is non-deterministic.
package transport

import (
	"context"
	"errors"

	"sea-noise/internal/ai/inferencerouter"
)

// Sentinel errors. Wrap with %w; callers check via errors.Is.
var (
	// ErrTransportUnavailable is returned when the underlying CLI / API
	// is missing on PATH or the transport otherwise can't start.
	ErrTransportUnavailable = errors.New("transport unavailable")

	// ErrTimeout is returned when the inference call exceeds its deadline.
	ErrTimeout = errors.New("inference timed out")

	// ErrNonZeroExit is returned when the underlying CLI exits non-zero.
	ErrNonZeroExit = errors.New("transport exited non-zero")

	// ErrEmptyResponse is returned when the transport succeeded but
	// produced no usable output.
	ErrEmptyResponse = errors.New("empty response")
)

// Transport runs a single LLM prompt and returns the model's response.
// The decision parameter is the InferenceDecision from the router —
// concrete transports may use it to select a model class. Failures
// return one of the sentinel errors above wrapped with %w.
type Transport interface {
	Run(ctx context.Context, decision inferencerouter.InferenceDecision, prompt string) (string, error)
}
