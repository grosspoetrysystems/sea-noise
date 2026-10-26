package enrichtopics_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sea-noise/internal/ai/inferencerouter"
	"sea-noise/internal/ai/skill"
	"sea-noise/internal/crawl"
	"sea-noise/internal/enrichtopics"
	"sea-noise/internal/parity"
)

// TestEnrichTopicsAuto_ParityWithPrototype is the third rung of the
// verification ladder per .tempor/exec/P1-W3-kickoff.md (T-SCALE-TOPIC).
// It exercises internal/parity primitives on the enrich-topics
// workflow against a recorded prototype LLM payload.
//
// Single-topic fixture: testdata/fixture-vault/_meta/topic-frequencies.md
// names go-workflows above threshold; the fixture provides tag-matched
// synthetic notes. One LLM call. The notes have no source URL, so the
// crawl path is a no-op — the StubCrawler seam is covered by unit tests
// in this package and in internal/crawl/.
//
// Capture pipeline:
//   - testdata/scripts/capture-enrich-topics.py runs the prototype
//     against a copy of testdata/fixture-vault with the PATH-shim;
//     pairs the captured envelope to the Go prompt by topic name;
//     re-keys under Go prompt hash; snapshots post-state.
//   - Re-run that script when fixture content, the prototype, or the
//     generate-research-brief skill template changes. See
//     testdata/CAPTURING-FIXTURES.md.
//
// Today is pinned to 2026-10-26 because the synthetic snapshot's brief
// filename is 2026-10-26-go-workflows-research-brief.md. Re-capturing
// on a different date requires re-pinning here in lockstep.
func TestEnrichTopicsAuto_ParityWithPrototype(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	fixtureSrc := filepath.Join(repoRoot, "testdata", "fixture-vault")
	envelopeCorpus := filepath.Join(repoRoot, "testdata", "envelopes", "enrich-topics")
	expectedVault := filepath.Join(repoRoot, "testdata", "expected", "enrich-topics", "post-state")
	skillsDir := filepath.Join(repoRoot, ".agents", "shared", "skills", "inference")
	crawlStubsDir := filepath.Join(repoRoot, "testdata", "crawl-stubs", "enrich-topics")

	vault := parity.LoadFixture(t, fixtureSrc)

	rt, err := parity.NewRecordedTransport(envelopeCorpus)
	if err != nil {
		t.Fatalf("NewRecordedTransport: %v", err)
	}
	if rt.Size() == 0 {
		t.Fatal("envelope corpus is empty — run testdata/scripts/capture-enrich-topics.py")
	}

	skills, err := skill.LoadFromDir(skillsDir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	exec := skill.NewExecutor(skills, inferencerouter.NewLocalRouter(), rt)

	stub, err := crawl.NewStubCrawler(crawlStubsDir)
	if err != nil {
		t.Fatalf("NewStubCrawler: %v", err)
	}

	res, err := enrichtopics.EnrichTopicsAuto(context.Background(), exec, stub, enrichtopics.AutoInput{
		VaultRoot: vault,
		Today:     time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("EnrichTopicsAuto: %v", err)
	}

	wantBrief := "notes/references/briefs/2026-10-26-go-workflows-research-brief.md"
	if len(res.Enriched) != 1 || res.Enriched[0].BriefPath != wantBrief {
		t.Fatalf("Enriched = %v, want one entry with BriefPath %q", res.Enriched, wantBrief)
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
