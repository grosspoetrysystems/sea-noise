package parity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"sea-noise/internal/ai/inferencerouter"
	"sea-noise/internal/ai/transport"
)

// Envelope is one recorded prototype LLM interaction. The corpus on
// disk is a directory of these — one JSON file per envelope, named by
// PromptHash to make collisions self-evident in the filesystem.
//
// SkillSHA pins the skill file's content at capture time per PARITY.md
// decision 4. Tests compare the recorded SkillSHA against the current
// skill file's SHA to surface drift distinct from port regressions.
type Envelope struct {
	SkillName  string `json:"skill_name"`
	SkillSHA   string `json:"skill_sha"`
	Prompt     string `json:"prompt"`
	Response   string `json:"response"`
	CapturedAt string `json:"captured_at"`

	// PromptHash is the SHA256 of Prompt, hex-encoded and truncated
	// to 16 chars. Set by Canonicalize / verified on load.
	PromptHash string `json:"prompt_hash"`
}

// Canonicalize sets PromptHash from Prompt. Call this when constructing
// envelopes programmatically; loaded envelopes have it set already.
func (e *Envelope) Canonicalize() {
	e.PromptHash = HashPrompt(e.Prompt)
}

// HashPrompt is the canonical prompt-hash function — SHA256, hex,
// truncated to 16 chars. Exported so capture scripts can produce the
// same key the runner expects.
func HashPrompt(prompt string) string {
	sum := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(sum[:])[:16]
}

// RecordedTransport implements transport.Transport by replaying
// envelopes loaded from a corpus directory. On each Run, it hashes the
// supplied prompt and returns the matching envelope's Response, or
// ErrEnvelopeNotFound if none exists.
//
// Concurrency-safe: a single RecordedTransport can be shared across
// goroutines / sub-tests; lookups are read-only after construction and
// Calls is mutex-guarded.
type RecordedTransport struct {
	byPromptHash map[string]Envelope

	mu        sync.Mutex
	calls     []RecordedCall
	lastMatch Envelope
}

// RecordedCall records one Run invocation. Tests inspect Calls to
// assert the workflow assembled the right prompt.
type RecordedCall struct {
	Decision inferencerouter.InferenceDecision
	Prompt   string
	Matched  bool
	SkillSHA string // recorded SHA of the matched envelope; empty if Matched is false
}

// NewRecordedTransport loads every *.json file under corpusDir as an
// Envelope and indexes them by PromptHash. Returns ErrCorpusInvalid on
// malformed entries or duplicate prompt hashes.
func NewRecordedTransport(corpusDir string) (*RecordedTransport, error) {
	rt := &RecordedTransport{byPromptHash: map[string]Envelope{}}

	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		return nil, fmt.Errorf("parity: read corpus dir %s: %w", corpusDir, err)
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(corpusDir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("parity: read envelope %s: %w", path, err)
		}
		var env Envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, fmt.Errorf("%w: parse %s: %v", ErrCorpusInvalid, path, err)
		}
		if env.PromptHash == "" {
			return nil, fmt.Errorf("%w: %s missing prompt_hash", ErrCorpusInvalid, path)
		}
		if got := HashPrompt(env.Prompt); got != env.PromptHash {
			return nil, fmt.Errorf("%w: %s prompt_hash mismatch: recorded %s, computed %s",
				ErrCorpusInvalid, path, env.PromptHash, got)
		}
		if _, dup := rt.byPromptHash[env.PromptHash]; dup {
			return nil, fmt.Errorf("%w: duplicate prompt_hash %s in %s",
				ErrCorpusInvalid, env.PromptHash, path)
		}
		rt.byPromptHash[env.PromptHash] = env
	}

	return rt, nil
}

// Run satisfies transport.Transport. Hashes the prompt, returns the
// matching envelope's Response, or ErrEnvelopeNotFound. Records every
// call to Calls and the matched envelope to LastMatch for test
// inspection.
func (rt *RecordedTransport) Run(_ context.Context, decision inferencerouter.InferenceDecision, prompt string) (string, error) {
	hash := HashPrompt(prompt)
	env, ok := rt.byPromptHash[hash]

	rt.mu.Lock()
	call := RecordedCall{Decision: decision, Prompt: prompt, Matched: ok}
	if ok {
		call.SkillSHA = env.SkillSHA
		rt.lastMatch = env
	}
	rt.calls = append(rt.calls, call)
	rt.mu.Unlock()

	if !ok {
		return "", fmt.Errorf("%w: hash %s", ErrEnvelopeNotFound, hash)
	}
	return env.Response, nil
}

// Calls returns a snapshot of recorded calls. Safe to call from tests
// while goroutines may still be invoking Run.
func (rt *RecordedTransport) Calls() []RecordedCall {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	out := make([]RecordedCall, len(rt.calls))
	copy(out, rt.calls)
	return out
}

// LastMatch returns the envelope matched by the most recent successful
// Run. Zero-value Envelope if no successful match has happened.
func (rt *RecordedTransport) LastMatch() Envelope {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.lastMatch
}

// Size returns the number of envelopes in the corpus. Useful for
// asserting "the corpus has the count the test expects."
func (rt *RecordedTransport) Size() int {
	return len(rt.byPromptHash)
}

// Compile-time assertion that RecordedTransport satisfies the
// transport interface.
var _ transport.Transport = (*RecordedTransport)(nil)
