// Package embedder defines the Embedder interface for the optional
// semantic retrieval layer (see ARCHITECTURE.md §5).
//
// The interface is harness-agnostic — implementations may call a remote
// HTTP API, run a local model, or return deterministic stubs for tests.
// The runtime ships with a semantic layer only if it materially
// improves retrieval quality against the golden fixture vault; that
// decision is made in Phase 4 (T10).
//
// Dimension is fixed at construction time (per provider/model choice);
// it does not vary per call. Callers that need a different dimension
// construct a different Embedder.
package embedder

import (
	"context"
	"errors"
)

// Sentinel errors. Wrap with %w; callers check via errors.Is.
var (
	// ErrEmptyInput is returned when Embed is called with no texts.
	ErrEmptyInput = errors.New("embedder: empty input")

	// ErrUnavailable is returned by implementations when the underlying
	// provider is configured but currently unreachable (network, quota,
	// transient API error). Callers may fall back per the retrieval
	// fallback chain (ARCHITECTURE.md §8).
	ErrUnavailable = errors.New("embedder: provider unavailable")
)

// Embedder produces embedding vectors for the given texts.
//
// Contract:
//   - Embed returns one vector per input, in the same order.
//   - All returned vectors have length Dimension().
//   - ProviderID() is a stable string identifying the provider+model
//     combination (e.g., "stub/zero-768" or some other implementation-
//     specific token). Used as a cache key and in KNOWN-DELTA.md
//     entries describing the semantic-layer decision. Concrete model
//     identifiers are config-only — they never appear as literals in
//     this package.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
	ProviderID() string
}
