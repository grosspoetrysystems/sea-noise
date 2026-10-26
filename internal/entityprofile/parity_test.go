package entityprofile_test

import (
	"context"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"sea-noise/internal/ai/inferencerouter"
	"sea-noise/internal/ai/skill"
	"sea-noise/internal/entityprofile"
	"sea-noise/internal/parity"
)

// TestGenerateAuto_ParityWithPrototype is the second rung of the
// verification ladder per .tempor/exec/P1-W3-kickoff.md (T-SCALE-PROFILE).
// It exercises internal/parity primitives on the two-phase
// profile-entities workflow against recorded prototype LLM payloads
// and asserts post-state structural equivalence.
//
// Capture pipeline that produced the inputs:
//   - testdata/scripts/capture-profile-entities.py runs the Python
//     prototype against a copy of testdata/fixture-vault with a
//     PATH-shim ahead of the real claude binary; classifies each
//     captured response (filter vs. per-entity profile) and re-keys
//     under the Go prompt hash via two-pass parity-discover; snapshots
//     the post-Python vault state.
//   - Re-run that script when the fixture, prototype, or either skill
//     template changes. See testdata/CAPTURING-FIXTURES.md.
//
// Today is pinned to 2026-10-26 because the captured snapshot's
// profile filename is 2026-10-26-profile-amber-engine.md. Re-capturing on
// a different date requires re-pinning here in lockstep.
func TestGenerateAuto_ParityWithPrototype(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	fixtureSrc := filepath.Join(repoRoot, "testdata", "fixture-vault")
	envelopeCorpus := filepath.Join(repoRoot, "testdata", "envelopes", "profile-entities")
	expectedVault := filepath.Join(repoRoot, "testdata", "expected", "profile-entities", "post-state")
	skillsDir := filepath.Join(repoRoot, ".agents", "shared", "skills", "inference")

	vault := parity.LoadFixture(t, fixtureSrc)

	rt, err := parity.NewRecordedTransport(envelopeCorpus)
	if err != nil {
		t.Fatalf("NewRecordedTransport: %v", err)
	}
	if rt.Size() == 0 {
		t.Fatal("envelope corpus is empty — run testdata/scripts/capture-profile-entities.py")
	}

	skills, err := skill.LoadFromDir(skillsDir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	exec := skill.NewExecutor(skills, inferencerouter.NewLocalRouter(), rt)

	res, err := entityprofile.GenerateAuto(context.Background(), exec, entityprofile.AutoInput{
		VaultRoot: vault,
		Today:     time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("GenerateAuto: %v", err)
	}

	// The synthetic fixture has one qualifying concept. The filter and
	// profile passes are both replayed from the public envelope corpus.
	wantProfileBasenames := []string{
		"2026-10-26-profile-amber-engine.md",
	}
	gotProfileBasenames := make([]string, 0, len(res.Profiled))
	for _, p := range res.Profiled {
		gotProfileBasenames = append(gotProfileBasenames, filepath.Base(p.ProfilePath))
	}
	sort.Strings(gotProfileBasenames)
	if !reflect.DeepEqual(gotProfileBasenames, wantProfileBasenames) {
		t.Fatalf("Profiled basenames = %v, want %v", gotProfileBasenames, wantProfileBasenames)
	}
	if len(res.Failed) != 0 {
		t.Fatalf("Failed = %v, want none", res.Failed)
	}

	// 2 calls: 1 filter + 1 profile. Every call must match a recorded
	// envelope; an unmatched call is a parity break.
	calls := rt.Calls()
	if len(calls) != 2 {
		t.Fatalf("transport calls = %d, want 2 (1 filter + 1 profile)", len(calls))
	}
	for i, c := range calls {
		if !c.Matched {
			t.Errorf("transport call %d did not match a recorded envelope; "+
				"prompt hash = %s, prompt prefix = %q",
				i, parity.HashPrompt(c.Prompt), truncate(c.Prompt, 100))
		}
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
