package inferencerouter

import "context"

// Router decides how to dispatch an LLM inference. Implementations are
// deterministic Go decision logic — they do not call out to LLMs to
// make routing decisions, and they do not invoke transports themselves.
// AI command wrappers obtain an InferenceDecision and then pass it to
// the transport layer.
type Router interface {
	Route(ctx context.Context, req InferenceRequest) (InferenceDecision, error)
}

// LocalRouter is the first concrete Router implementation. It returns
// sensible defaults regardless of input. Real heuristics (size-aware
// routing, profile-conditional model selection) are post-parity work;
// this stub exists to make AI command wrappers compile and run.
type LocalRouter struct {
	defaults InferenceDecision
}

// NewLocalRouter returns a LocalRouter with the project's default
// decision (Tier: Medium, Profile: Auto, Target: Smart). The defaults
// are not configurable here; callers that need different ones build
// their own Router.
func NewLocalRouter() *LocalRouter {
	return &LocalRouter{
		defaults: InferenceDecision{
			Tier:    TierMedium,
			Profile: ProfileAuto,
			Target:  TargetSmart,
		},
	}
}

// Route returns the LocalRouter's default decision. The InferenceRequest
// fields are ignored at this stub stage; future routers consume them.
func (r *LocalRouter) Route(_ context.Context, _ InferenceRequest) (InferenceDecision, error) {
	return r.defaults, nil
}

// Compile-time assertion that LocalRouter satisfies Router.
var _ Router = (*LocalRouter)(nil)
