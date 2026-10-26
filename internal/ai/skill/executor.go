package skill

import (
	"context"
	"fmt"

	"sea-noise/internal/ai/inferencerouter"
	"sea-noise/internal/ai/transport"
)

// Executor ties a skill catalog, an inference router, and a transport
// together. Callers obtain an Executor and call Execute(skillName, data)
// to render the skill's prompt and dispatch it for inference.
type Executor struct {
	Skills    map[string]*Skill
	Router    inferencerouter.Router
	Transport transport.Transport
}

// NewExecutor returns an Executor wired with the given catalog, router,
// and transport. Callers typically load skills via LoadFromDir.
func NewExecutor(skills map[string]*Skill, router inferencerouter.Router, t transport.Transport) *Executor {
	return &Executor{Skills: skills, Router: router, Transport: t}
}

// Execute renders the named skill's prompt with the given data, asks
// the router for an InferenceDecision, and dispatches via the
// transport. The transport's stripped response is returned verbatim;
// callers parse it per the skill's declared output format.
func (e *Executor) Execute(ctx context.Context, skillName string, data any) (string, error) {
	s, ok := e.Skills[skillName]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrSkillNotFound, skillName)
	}

	prompt, err := s.Render(data)
	if err != nil {
		return "", err
	}

	// Build the inference request from the skill's metadata. Purpose
	// is the skill name so telemetry / future routing can see which
	// skill drove the call.
	req := inferencerouter.InferenceRequest{
		Purpose:   s.Name,
		InputSize: len(prompt),
	}
	decision, err := e.Router.Route(ctx, req)
	if err != nil {
		return "", fmt.Errorf("route skill %s: %w", s.Name, err)
	}

	return e.Transport.Run(ctx, decision, prompt)
}
