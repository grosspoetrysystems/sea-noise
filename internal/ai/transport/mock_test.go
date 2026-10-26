package transport

import (
	"context"
	"errors"
	"testing"

	"sea-noise/internal/ai/inferencerouter"
)

func TestMockTransport_QueuesResponses(t *testing.T) {
	m := &MockTransport{}
	m.EnqueueResponses("first", "second")

	d := inferencerouter.InferenceDecision{Tier: inferencerouter.TierMedium}

	r1, err := m.Run(context.Background(), d, "prompt 1")
	if err != nil || r1 != "first" {
		t.Fatalf("Run 1 = (%q, %v), want (\"first\", nil)", r1, err)
	}
	r2, err := m.Run(context.Background(), d, "prompt 2")
	if err != nil || r2 != "second" {
		t.Fatalf("Run 2 = (%q, %v), want (\"second\", nil)", r2, err)
	}

	if len(m.Calls) != 2 {
		t.Fatalf("Calls length = %d, want 2", len(m.Calls))
	}
	if m.Calls[0].Prompt != "prompt 1" || m.Calls[1].Prompt != "prompt 2" {
		t.Errorf("Calls = %#v, prompts mis-recorded", m.Calls)
	}
	if m.Calls[0].Decision != d {
		t.Errorf("Calls[0].Decision = %+v, want %+v", m.Calls[0].Decision, d)
	}
}

func TestMockTransport_EmptyQueueReturnsSentinel(t *testing.T) {
	m := &MockTransport{}
	_, err := m.Run(context.Background(), inferencerouter.InferenceDecision{}, "prompt")
	if !errors.Is(err, ErrEmptyResponse) {
		t.Errorf("err = %v, want errors.Is(err, ErrEmptyResponse)", err)
	}
}

func TestMockTransport_CustomEmptyError(t *testing.T) {
	custom := errors.New("custom test error")
	m := &MockTransport{ErrOnEmpty: custom}
	_, err := m.Run(context.Background(), inferencerouter.InferenceDecision{}, "prompt")
	if !errors.Is(err, custom) {
		t.Errorf("err = %v, want %v", err, custom)
	}
}

func TestMockTransport_EnqueueError(t *testing.T) {
	m := &MockTransport{}
	m.EnqueueResponses("ok1")
	m.EnqueueError(ErrTransportUnavailable)
	m.EnqueueResponses("ok2")

	r1, err := m.Run(context.Background(), inferencerouter.InferenceDecision{}, "p1")
	if err != nil || r1 != "ok1" {
		t.Fatalf("Run 1 = (%q, %v), want (\"ok1\", nil)", r1, err)
	}
	_, err = m.Run(context.Background(), inferencerouter.InferenceDecision{}, "p2")
	if !errors.Is(err, ErrTransportUnavailable) {
		t.Fatalf("Run 2 err = %v, want ErrTransportUnavailable", err)
	}
	r3, err := m.Run(context.Background(), inferencerouter.InferenceDecision{}, "p3")
	if err != nil || r3 != "ok2" {
		t.Fatalf("Run 3 = (%q, %v), want (\"ok2\", nil)", r3, err)
	}
}
