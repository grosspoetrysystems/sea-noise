package inferencerouter

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors for enum parsing. Callers check via errors.Is.
var (
	ErrInvalidTier    = errors.New("invalid classification tier")
	ErrInvalidProfile = errors.New("invalid routing profile")
	ErrInvalidTarget  = errors.New("invalid local target")
)

func (t ClassificationTier) String() string {
	switch t {
	case TierUnspecified:
		return ""
	case TierSimple:
		return "simple"
	case TierMedium:
		return "medium"
	case TierComplex:
		return "complex"
	case TierReasoning:
		return "reasoning"
	default:
		return fmt.Sprintf("tier(%d)", int(t))
	}
}

func ParseTier(s string) (ClassificationTier, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return TierUnspecified, nil
	case "simple":
		return TierSimple, nil
	case "medium":
		return TierMedium, nil
	case "complex":
		return TierComplex, nil
	case "reasoning":
		return TierReasoning, nil
	}
	return TierUnspecified, fmt.Errorf("%w: %q", ErrInvalidTier, s)
}

func (p RoutingProfile) String() string {
	switch p {
	case ProfileUnspecified:
		return ""
	case ProfileAuto:
		return "auto"
	case ProfileEco:
		return "eco"
	case ProfilePremium:
		return "premium"
	case ProfileFree:
		return "free"
	default:
		return fmt.Sprintf("profile(%d)", int(p))
	}
}

func ParseProfile(s string) (RoutingProfile, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return ProfileUnspecified, nil
	case "auto":
		return ProfileAuto, nil
	case "eco":
		return ProfileEco, nil
	case "premium":
		return ProfilePremium, nil
	case "free":
		return ProfileFree, nil
	}
	return ProfileUnspecified, fmt.Errorf("%w: %q", ErrInvalidProfile, s)
}

func (t LocalTarget) String() string {
	switch t {
	case TargetUnspecified:
		return ""
	case TargetFast:
		return "fast"
	case TargetSmart:
		return "smart"
	case TargetHeavy:
		return "heavy"
	default:
		return fmt.Sprintf("target(%d)", int(t))
	}
}

func ParseTarget(s string) (LocalTarget, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return TargetUnspecified, nil
	case "fast":
		return TargetFast, nil
	case "smart":
		return TargetSmart, nil
	case "heavy":
		return TargetHeavy, nil
	}
	return TargetUnspecified, fmt.Errorf("%w: %q", ErrInvalidTarget, s)
}
