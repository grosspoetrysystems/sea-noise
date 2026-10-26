package parity_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"sea-noise/internal/ai/inferencerouter"
	"sea-noise/internal/parity"
)

func TestRecordedTransport_LookupByPromptHash(t *testing.T) {
	dir := t.TempDir()
	writeEnvelope(t, dir, "e1.json", parity.Envelope{
		SkillName: "summarize-content",
		SkillSHA:  "sha-a",
		Prompt:    "Summarize this transcript: hello",
		Response:  "A short summary.",
	})
	writeEnvelope(t, dir, "e2.json", parity.Envelope{
		SkillName: "summarize-content",
		SkillSHA:  "sha-a",
		Prompt:    "Summarize this transcript: world",
		Response:  "Another summary.",
	})

	rt, err := parity.NewRecordedTransport(dir)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if rt.Size() != 2 {
		t.Fatalf("size: got %d, want 2", rt.Size())
	}

	resp, err := rt.Run(context.Background(), inferencerouter.InferenceDecision{}, "Summarize this transcript: hello")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp != "A short summary." {
		t.Fatalf("resp: got %q", resp)
	}

	calls := rt.Calls()
	if len(calls) != 1 || !calls[0].Matched || calls[0].SkillSHA != "sha-a" {
		t.Fatalf("calls: %+v", calls)
	}
	if got := rt.LastMatch().Response; got != "A short summary." {
		t.Fatalf("last match response: %q", got)
	}
}

func TestRecordedTransport_MissingEnvelopeReturnsSentinel(t *testing.T) {
	dir := t.TempDir()
	writeEnvelope(t, dir, "e1.json", parity.Envelope{
		Prompt:   "known",
		Response: "ok",
	})

	rt, err := parity.NewRecordedTransport(dir)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	_, err = rt.Run(context.Background(), inferencerouter.InferenceDecision{}, "unknown prompt")
	if !errors.Is(err, parity.ErrEnvelopeNotFound) {
		t.Fatalf("err: got %v, want wrap of ErrEnvelopeNotFound", err)
	}

	calls := rt.Calls()
	if len(calls) != 1 || calls[0].Matched {
		t.Fatalf("unmatched call should be recorded as not matched: %+v", calls)
	}
}

func TestRecordedTransport_CorpusInvalid(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, dir string)
	}{
		{
			name: "malformed json",
			setup: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{not json"), 0o644); err != nil {
					t.Fatalf("write: %v", err)
				}
			},
		},
		{
			name: "missing prompt_hash",
			setup: func(t *testing.T, dir string) {
				env := parity.Envelope{Prompt: "x", Response: "y"} // PromptHash unset
				b, _ := json.Marshal(env)
				if err := os.WriteFile(filepath.Join(dir, "e.json"), b, 0o644); err != nil {
					t.Fatalf("write: %v", err)
				}
			},
		},
		{
			name: "prompt_hash mismatch",
			setup: func(t *testing.T, dir string) {
				env := parity.Envelope{Prompt: "x", Response: "y", PromptHash: "deadbeefdeadbeef"}
				b, _ := json.Marshal(env)
				if err := os.WriteFile(filepath.Join(dir, "e.json"), b, 0o644); err != nil {
					t.Fatalf("write: %v", err)
				}
			},
		},
		{
			name: "duplicate prompt_hash",
			setup: func(t *testing.T, dir string) {
				writeEnvelope(t, dir, "a.json", parity.Envelope{Prompt: "dup", Response: "r1"})
				writeEnvelope(t, dir, "b.json", parity.Envelope{Prompt: "dup", Response: "r2"})
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tc.setup(t, dir)
			_, err := parity.NewRecordedTransport(dir)
			if !errors.Is(err, parity.ErrCorpusInvalid) {
				t.Fatalf("err: got %v, want wrap of ErrCorpusInvalid", err)
			}
		})
	}
}

func TestRecordedTransport_EmptyDirIsValid(t *testing.T) {
	dir := t.TempDir()
	rt, err := parity.NewRecordedTransport(dir)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if rt.Size() != 0 {
		t.Fatalf("size: got %d, want 0", rt.Size())
	}
}

func TestHashPrompt_DeterministicAndLength(t *testing.T) {
	a := parity.HashPrompt("hello")
	b := parity.HashPrompt("hello")
	c := parity.HashPrompt("world")
	if a != b {
		t.Fatalf("hash not deterministic: %q vs %q", a, b)
	}
	if a == c {
		t.Fatalf("collision between distinct prompts: %q", a)
	}
	if len(a) != 16 {
		t.Fatalf("hash length: got %d, want 16", len(a))
	}
}

// writeEnvelope serializes env with PromptHash canonicalized and
// writes it under dir/name. Test helper local to this file (rule 7);
// promote when a second consumer needs it.
func writeEnvelope(t *testing.T, dir, name string, env parity.Envelope) {
	t.Helper()
	env.Canonicalize()
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
