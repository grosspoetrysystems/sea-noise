package transport

import (
	"context"

	"sea-noise/internal/ai/inferencerouter"
)

// MockTransport is a deterministic Transport for tests. It dispenses
// queued results (success responses OR errors, interleaved) in FIFO
// order and records every prompt + decision the transport received.
//
// Tests of any AI command wrapper SHOULD use MockTransport via the
// Transport interface so they never make real LLM calls.
type MockTransport struct {
	// Results is the FIFO queue of outcomes. Each entry is either a
	// success response or an error. Use EnqueueResponses and
	// EnqueueError to populate.
	Results []MockResult

	// ErrOnEmpty is the error returned when the result queue is empty.
	// Zero value -> ErrEmptyResponse, so a missing test setup fails loudly.
	ErrOnEmpty error

	// Calls records every (decision, prompt) the transport received,
	// in order. Tests assert against this to verify the wrapper sent
	// the right prompt.
	Calls []MockCall
}

// MockResult is one queued outcome: a response string (for success) or
// an error (for failure). Exactly one is set.
type MockResult struct {
	Response string
	Err      error
}

// MockCall is one recorded invocation of MockTransport.Run.
type MockCall struct {
	Decision inferencerouter.InferenceDecision
	Prompt   string
}

// Run pops the next queued result, records the call, and returns
// either the queued response or the queued error. When the queue is
// empty, returns ErrOnEmpty (defaulting to ErrEmptyResponse).
func (m *MockTransport) Run(_ context.Context, decision inferencerouter.InferenceDecision, prompt string) (string, error) {
	m.Calls = append(m.Calls, MockCall{Decision: decision, Prompt: prompt})

	if len(m.Results) == 0 {
		if m.ErrOnEmpty != nil {
			return "", m.ErrOnEmpty
		}
		return "", ErrEmptyResponse
	}

	r := m.Results[0]
	m.Results = m.Results[1:]
	if r.Err != nil {
		return "", r.Err
	}
	return r.Response, nil
}

// EnqueueResponses appends success responses to the FIFO queue.
func (m *MockTransport) EnqueueResponses(responses ...string) {
	for _, r := range responses {
		m.Results = append(m.Results, MockResult{Response: r})
	}
}

// EnqueueError appends an error to the FIFO queue. The next Run() call
// after the existing queue entries will return this error.
func (m *MockTransport) EnqueueError(err error) {
	m.Results = append(m.Results, MockResult{Err: err})
}

// Compile-time assertion.
var _ Transport = (*MockTransport)(nil)
