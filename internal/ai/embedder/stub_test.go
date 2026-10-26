package embedder

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestStub_Embed_Shape(t *testing.T) {
	s := NewStub(384)
	got, err := s.Embed(context.Background(), []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}
	for i, v := range got {
		if len(v) != 384 {
			t.Errorf("vector %d has dim %d, want 384", i, len(v))
		}
		for j, f := range v {
			if f != 0 {
				t.Errorf("vector %d[%d] = %v, want 0", i, j, f)
			}
		}
	}
}

func TestStub_Embed_Empty(t *testing.T) {
	s := NewStub(128)
	_, err := s.Embed(context.Background(), nil)
	if !errors.Is(err, ErrEmptyInput) {
		t.Errorf("err = %v, want errors.Is(err, ErrEmptyInput)", err)
	}
}

func TestStub_DimensionAndProviderID(t *testing.T) {
	s := NewStub(768)
	if got := s.Dimension(); got != 768 {
		t.Errorf("Dimension = %d, want 768", got)
	}
	if got := s.ProviderID(); !strings.Contains(got, "768") {
		t.Errorf("ProviderID = %q, want it to mention 768", got)
	}
}

// Confirm the contract via an interface variable (compile-time check
// that *Stub satisfies Embedder).
var _ Embedder = (*Stub)(nil)
