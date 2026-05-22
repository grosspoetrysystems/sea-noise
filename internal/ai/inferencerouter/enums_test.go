package inferencerouter

import (
	"errors"
	"testing"
)

func TestClassificationTier_StringAndParse(t *testing.T) {
	cases := []struct {
		val    ClassificationTier
		str    string
		parses string
	}{
		{TierUnspecified, "", ""},
		{TierSimple, "simple", "Simple"},
		{TierMedium, "medium", "MEDIUM"},
		{TierComplex, "complex", "complex"},
		{TierReasoning, "reasoning", "  reasoning  "},
	}
	for _, c := range cases {
		t.Run(c.str+"_roundtrip", func(t *testing.T) {
			if got := c.val.String(); got != c.str {
				t.Errorf("String() = %q, want %q", got, c.str)
			}
			got, err := ParseTier(c.parses)
			if err != nil {
				t.Errorf("ParseTier(%q): %v", c.parses, err)
			}
			if got != c.val {
				t.Errorf("ParseTier(%q) = %v, want %v", c.parses, got, c.val)
			}
		})
	}
}

func TestClassificationTier_ParseInvalid(t *testing.T) {
	_, err := ParseTier("bogus")
	if !errors.Is(err, ErrInvalidTier) {
		t.Errorf("err = %v, want errors.Is(err, ErrInvalidTier)", err)
	}
}

func TestRoutingProfile_StringAndParse(t *testing.T) {
	cases := []struct {
		val    RoutingProfile
		str    string
		parses string
	}{
		{ProfileUnspecified, "", ""},
		{ProfileAuto, "auto", "Auto"},
		{ProfileEco, "eco", "ECO"},
		{ProfilePremium, "premium", "premium"},
		{ProfileFree, "free", "Free"},
	}
	for _, c := range cases {
		t.Run(c.str+"_roundtrip", func(t *testing.T) {
			if got := c.val.String(); got != c.str {
				t.Errorf("String() = %q, want %q", got, c.str)
			}
			got, err := ParseProfile(c.parses)
			if err != nil {
				t.Errorf("ParseProfile(%q): %v", c.parses, err)
			}
			if got != c.val {
				t.Errorf("ParseProfile(%q) = %v, want %v", c.parses, got, c.val)
			}
		})
	}
}

func TestRoutingProfile_ParseInvalid(t *testing.T) {
	_, err := ParseProfile("turbo")
	if !errors.Is(err, ErrInvalidProfile) {
		t.Errorf("err = %v, want errors.Is(err, ErrInvalidProfile)", err)
	}
}

func TestLocalTarget_StringAndParse(t *testing.T) {
	cases := []struct {
		val    LocalTarget
		str    string
		parses string
	}{
		{TargetUnspecified, "", ""},
		{TargetFast, "fast", "Fast"},
		{TargetSmart, "smart", "smart"},
		{TargetHeavy, "heavy", "HEAVY"},
	}
	for _, c := range cases {
		t.Run(c.str+"_roundtrip", func(t *testing.T) {
			if got := c.val.String(); got != c.str {
				t.Errorf("String() = %q, want %q", got, c.str)
			}
			got, err := ParseTarget(c.parses)
			if err != nil {
				t.Errorf("ParseTarget(%q): %v", c.parses, err)
			}
			if got != c.val {
				t.Errorf("ParseTarget(%q) = %v, want %v", c.parses, got, c.val)
			}
		})
	}
}

func TestLocalTarget_ParseInvalid(t *testing.T) {
	_, err := ParseTarget("ludicrous")
	if !errors.Is(err, ErrInvalidTarget) {
		t.Errorf("err = %v, want errors.Is(err, ErrInvalidTarget)", err)
	}
}
