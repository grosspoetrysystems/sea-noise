// Package inferencerouter defines the inference eval router seam (see
// ARCHITECTURE.md §6).
//
// The seam is harness-agnostic: it is not designed around any specific
// external routing product, present or anticipated. AI command wrappers
// obtain an InferenceDecision from this package before invoking any LLM
// transport.
//
// Constraints on this package, enforced by CI grep gates:
//   - zero exec.Command calls
//   - zero outbound net/http
//   - zero provider SDK imports
//   - zero string literals for model/provider IDs
package inferencerouter

// ClassificationTier names the rough computational shape an inference
// call needs. Callers may leave it Unspecified; the router fills it in
// from heuristics or defaults.
type ClassificationTier int

const (
	TierUnspecified ClassificationTier = iota
	TierSimple                         // small lookups, classifications, format-mapping
	TierMedium                         // single-turn synthesis, summarization
	TierComplex                        // multi-step reasoning, longer context assembly
	TierReasoning                      // deep step-by-step inference
)

// RoutingProfile expresses the user's cost/quality preference for a
// given session. The router maps Profile + Tier to a concrete Target.
type RoutingProfile int

const (
	ProfileUnspecified RoutingProfile = iota
	ProfileAuto                       // router decides
	ProfileEco                        // prefer cheaper/faster models
	ProfilePremium                    // prefer higher-quality models
	ProfileFree                       // use only no-cost models
)

// LocalTarget is the abstract model class the router resolves to. The
// transport layer maps Target to a concrete provider/model via config —
// no provider/model strings appear in this package.
type LocalTarget int

const (
	TargetUnspecified LocalTarget = iota
	TargetFast                    // small, fast model
	TargetSmart                   // mid-tier, balanced
	TargetHeavy                   // largest available
)

// InferenceRequest is what a caller passes to Router.Route. Fields are
// optional; the router fills in defaults where unset.
type InferenceRequest struct {
	// Tier is the caller's hint about computational shape. Unspecified
	// means "router decides."
	Tier ClassificationTier

	// Profile is the user's cost/quality preference for this call.
	// Unspecified means "use the session default" (which the router
	// or its caller resolves elsewhere).
	Profile RoutingProfile

	// InputSize is an approximate token count of the inference input.
	// Used by size-aware routing heuristics. Zero means "unknown."
	InputSize int

	// Purpose is a freeform tag for telemetry and logging (e.g.,
	// "enrich-topic", "profile-entity"). Not used for routing
	// decisions; carried through for observability.
	Purpose string
}

// InferenceDecision is what the router returns. AI command wrappers
// pass the decision to the transport layer, which resolves Target into
// a concrete provider/model via config.
type InferenceDecision struct {
	Tier    ClassificationTier
	Profile RoutingProfile
	Target  LocalTarget
}
