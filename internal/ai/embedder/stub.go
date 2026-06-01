package embedder

import (
	"context"
	"fmt"
)

// Stub is a deterministic Embedder that returns zero-vectors of the
// configured dimension. Used by tests and by downstream consumers
// (e.g., T05 pipeline commands) that need an Embedder wired in before
// Phase 4 ships the real implementation.
//
// Stub is intentionally trivial; it does not call out to any network
// service, and its zero vectors mean cosine similarity is undefined.
// Tests that exercise retrieval ranking should not use Stub for the
// retrieval-quality assertion — they use real vectors or skip the
// semantic surface entirely.
type Stub struct {
	dim int
	id  string
}

// NewStub returns a Stub with the given dimension. ID format mirrors
// "stub/zero-<dim>" so it's recognizable in logs and .tempor/KNOWN-DELTA.md.
func NewStub(dim int) *Stub {
	return &Stub{dim: dim, id: fmt.Sprintf("stub/zero-%d", dim)}
}

func (s *Stub) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, ErrEmptyInput
	}
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = make([]float32, s.dim) // all zeros by default
	}
	return out, nil
}

func (s *Stub) Dimension() int     { return s.dim }
func (s *Stub) ProviderID() string { return s.id }
