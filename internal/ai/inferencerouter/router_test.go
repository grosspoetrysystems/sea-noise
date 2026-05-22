package inferencerouter

import (
	"context"
	"testing"
)

func TestLocalRouter_ReturnsDefaults(t *testing.T) {
	r := NewLocalRouter()

	cases := []struct {
		name string
		req  InferenceRequest
	}{
		{"empty request", InferenceRequest{}},
		{"caller hints ignored at stub stage", InferenceRequest{
			Tier:    TierSimple,
			Profile: ProfileEco,
		}},
		{"large input still hits defaults", InferenceRequest{
			InputSize: 1_000_000,
			Purpose:   "synthesize",
		}},
	}

	want := InferenceDecision{
		Tier:    TierMedium,
		Profile: ProfileAuto,
		Target:  TargetSmart,
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := r.Route(context.Background(), c.req)
			if err != nil {
				t.Fatalf("Route: %v", err)
			}
			if got != want {
				t.Errorf("Route = %+v, want %+v", got, want)
			}
		})
	}
}

// Compile-time confirmation that NewLocalRouter returns a Router.
func TestLocalRouter_SatisfiesInterface(t *testing.T) {
	var _ Router = NewLocalRouter()
}
