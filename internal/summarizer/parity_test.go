package summarizer_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"sea-noise/internal/ai/inferencerouter"
	"sea-noise/internal/ai/skill"
	"sea-noise/internal/parity"
	"sea-noise/internal/summarizer"
)

// TestEnrichTranscripts_ParityWithPrototype is the first end-to-end
// parity proof per .tempor/exec/P1-W2-kickoff.md (T-PROVE on
// summarize-transcripts). It exercises internal/parity primitives on
// the live workflow against a recorded prototype LLM payload and
// asserts post-state structural equivalence.
//
// Capture pipeline that produced the inputs:
//   - testdata/scripts/capture-llm-envelopes.py runs the Python
//     prototype against a copy of testdata/fixture-vault with a
//     PATH-shim ahead of the real claude binary; pairs each captured
//     response with the Go-rendered prompt from
//     `sea-noise parity-discover`; writes envelopes keyed by Go
//     prompt hash; snapshots the post-Python vault state.
//   - Re-run that script when the fixture, prototype, or Go skill
//     template changes. See testdata/CAPTURING-FIXTURES.md.
//
// Fixture coverage rationale:
//   - example-workflow: synthetic qualifying transcript note; processed
//     identically by both Python and Go → the parity-asserted case.
//   - roasted-vegetables: a non-transcript reference note that both
//     implementations skip.
//   - notes under other directories are outside the workflow's search
//     paths and remain untouched.
//
// The fixture cleanly produces a single LLM-touching mutation that both
// implementations must reproduce.
func TestEnrichTranscripts_ParityWithPrototype(t *testing.T) {
	// Package CWD = internal/summarizer at test time; "../.." is the repo root.
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	fixtureSrc := filepath.Join(repoRoot, "testdata", "fixture-vault")
	envelopeCorpus := filepath.Join(repoRoot, "testdata", "envelopes", "summarize-transcripts")
	expectedVault := filepath.Join(repoRoot, "testdata", "expected", "summarize-transcripts", "post-state")
	skillsDir := filepath.Join(repoRoot, ".agents", "shared", "skills", "inference")

	vault := parity.LoadFixture(t, fixtureSrc)

	rt, err := parity.NewRecordedTransport(envelopeCorpus)
	if err != nil {
		t.Fatalf("NewRecordedTransport: %v", err)
	}
	if rt.Size() == 0 {
		t.Fatal("envelope corpus is empty — run testdata/scripts/capture-llm-envelopes.py")
	}

	skills, err := skill.LoadFromDir(skillsDir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	exec := skill.NewExecutor(skills, inferencerouter.NewLocalRouter(), rt)

	res, err := summarizer.EnrichTranscripts(context.Background(), exec, summarizer.EnrichTranscriptsInput{
		VaultRoot: vault,
	})
	if err != nil {
		t.Fatalf("EnrichTranscripts: %v", err)
	}

	wantUpdated := "notes/references/2026-04-12-example-workflow.md"
	if len(res.Updated) != 1 || res.Updated[0] != wantUpdated {
		t.Fatalf("Updated = %v, want [%s]", res.Updated, wantUpdated)
	}
	if len(res.Failed) != 0 {
		t.Fatalf("Failed = %v, want none", res.Failed)
	}

	calls := rt.Calls()
	if len(calls) != 1 {
		t.Fatalf("transport calls = %d, want 1", len(calls))
	}
	if !calls[0].Matched {
		t.Errorf("transport call did not match a recorded envelope; "+
			"prompt hash = %s, prompt prefix = %q",
			parity.HashPrompt(calls[0].Prompt), truncate(calls[0].Prompt, 100))
	}

	diffs, err := parity.Diff(expectedVault, vault, parity.DefaultEquivalence())
	if err != nil {
		t.Fatalf("parity.Diff: %v", err)
	}
	if len(diffs) > 0 {
		var b strings.Builder
		for _, d := range diffs {
			b.WriteString("  ")
			b.WriteString(d.String())
			b.WriteString("\n")
		}
		t.Errorf("Go post-state diverges from recorded prototype post-state:\n%s", b.String())
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
