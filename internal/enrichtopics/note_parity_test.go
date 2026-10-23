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

// TestEnrichNote_ParityWithPrototype proves the `enrich-topics --note`
// (per-note enrichment) path against a captured prototype run. Added
// at W4 close to lift the `--note` matrix row from `pinned` to
// `parity-proven`, tightening the phase-exit ratio (criterion #3).
//
// Single LLM call per invocation; mirrors the W2 summarize-transcripts
// shape. The target note has no source URL, so the crawl path doesn't
// fire — StubCrawler is wired for completeness but unused on this
// fixture.
//
// Capture: testdata/scripts/capture-enrich-note.py. Re-run when the
// fixture note, the prototype, or the enrich-single-note skill
// template changes.
func TestEnrichNote_ParityWithPrototype(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	fixtureSrc := filepath.Join(repoRoot, "testdata", "fixture-vault")
	envelopeCorpus := filepath.Join(repoRoot, "testdata", "envelopes", "enrich-note")
	expectedVault := filepath.Join(repoRoot, "testdata", "expected", "enrich-note", "post-state")
	skillsDir := filepath.Join(repoRoot, ".agents", "shared", "skills", "inference")
	crawlStubsDir := filepath.Join(repoRoot, "testdata", "crawl-stubs", "enrich-topics")

	vault := parity.LoadFixture(t, fixtureSrc)

	rt, err := parity.NewRecordedTransport(envelopeCorpus)
	if err != nil {
		t.Fatalf("NewRecordedTransport: %v", err)
	}
	if rt.Size() == 0 {
		t.Fatal("envelope corpus is empty — run testdata/scripts/capture-enrich-note.py")
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

	const targetNote = "notes/references/2026-03-09-cauliflower-lentils-chorizo.md"
	res, err := enrichtopics.EnrichNote(context.Background(), exec, stub, enrichtopics.NoteInput{
		VaultRoot: vault,
		NotePath:  targetNote,
		Today:     time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("EnrichNote: %v", err)
	}
	if res.NotePath != targetNote {
		t.Errorf("res.NotePath = %q, want %q", res.NotePath, targetNote)
	}
	if res.Crawled != 0 {
		t.Errorf("res.Crawled = %d, want 0 (no source URL → no crawl)", res.Crawled)
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
