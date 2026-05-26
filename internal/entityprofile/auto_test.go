package entityprofile

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sea-noise/internal/ai/inferencerouter"
	"sea-noise/internal/ai/skill"
	"sea-noise/internal/ai/transport"
)

func TestGenerateAuto_Happy(t *testing.T) {
	vault := t.TempDir()
	// Three notes, each mentioning a different repeat-able entity.
	// Counts: Sam Kriss x5, Dario Amodei x5, Margaret Atwood x4.
	writeNote(t, vault, "notes/fleeting/a.md", `---
title: First Note
---

Sam Kriss writes essays. Sam Kriss has a substack. Sam Kriss again. Sam Kriss
again. Sam Kriss.
`)
	writeNote(t, vault, "notes/fleeting/b.md", `---
title: Anthropic Talk
---

Dario Amodei spoke yesterday. Dario Amodei runs Anthropic. Dario Amodei.
Dario Amodei. Dario Amodei.
`)
	writeNote(t, vault, "notes/references/c.md", `---
title: Atwood Essay
---

Margaret Atwood wrote it. Margaret Atwood is canadian. Margaret Atwood.
Margaret Atwood.
`)

	exec := newAutoExecutor(t,
		// First execute call: filter-entity-candidates.
		`[{"name":"Sam Kriss","keep":true,"type":"person"},{"name":"Dario Amodei","keep":true,"type":"person"},{"name":"Margaret Atwood","keep":false,"type":"person"}]`,
		// Two follow-on calls: generate-entity-profile per kept entity,
		// emitted in count-desc / name-asc order (Dario before Sam at
		// equal counts; see the sort.Slice in GenerateAuto).
		"DARIO PROFILE BODY",
		"SAM PROFILE BODY",
	)

	res, err := GenerateAuto(context.Background(), exec, AutoInput{
		VaultRoot: vault,
		Threshold: 4,
		MaxPerRun: 5,
		Today:     time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("GenerateAuto: %v", err)
	}
	if len(res.Profiled) != 2 {
		t.Fatalf("Profiled = %d, want 2 (Margaret filtered out by skill)", len(res.Profiled))
	}
	// Result entities, in deterministic order, are Sam and Dario.
	got := []string{
		filepath.Base(res.Profiled[0].ProfilePath),
		filepath.Base(res.Profiled[1].ProfilePath),
	}
	want := []string{"2026-05-26-profile-dario-amodei.md", "2026-05-26-profile-sam-kriss.md"}
	if got[0] != want[0] || got[1] != want[1] {
		t.Errorf("profile filenames = %v, want %v", got, want)
	}
	// Verify entity-type was threaded through to the frontmatter.
	contents := mustRead(t, filepath.Join(vault, res.Profiled[0].ProfilePath))
	if !strings.Contains(contents, "entity-type: person") {
		t.Errorf("profile missing entity-type: person\n%s", contents)
	}
	if !strings.Contains(contents, "  - entity/person") {
		t.Errorf("profile missing entity/person tag\n%s", contents)
	}
}

func TestGenerateAuto_RespectsThreshold(t *testing.T) {
	vault := t.TempDir()
	// Only 2 mentions — below threshold of 5.
	writeNote(t, vault, "notes/fleeting/a.md", `---
title: One Sighting
---

Cheap Concept exists. Cheap Concept again.
`)
	exec := newAutoExecutor(t, "[]")
	res, err := GenerateAuto(context.Background(), exec, AutoInput{
		VaultRoot: vault,
		Threshold: 5,
	})
	if err != nil {
		t.Fatalf("GenerateAuto: %v", err)
	}
	if len(res.Profiled) != 0 {
		t.Errorf("Profiled = %v, want 0 (below threshold)", res.Profiled)
	}
}

func TestGenerateAuto_RespectsCap(t *testing.T) {
	vault := t.TempDir()
	// Three eligible candidates, cap at 1.
	writeNote(t, vault, "notes/fleeting/a.md", `---
title: A
---

Alpha Bravo. Alpha Bravo. Alpha Bravo. Alpha Bravo. Alpha Bravo.
Charlie Delta. Charlie Delta. Charlie Delta. Charlie Delta. Charlie Delta.
Echo Foxtrot. Echo Foxtrot. Echo Foxtrot. Echo Foxtrot. Echo Foxtrot.
`)
	exec := newAutoExecutor(t,
		`[{"name":"Alpha Bravo","keep":true,"type":"concept"},{"name":"Charlie Delta","keep":true,"type":"concept"},{"name":"Echo Foxtrot","keep":true,"type":"concept"}]`,
		"PROFILE",
	)
	res, err := GenerateAuto(context.Background(), exec, AutoInput{
		VaultRoot: vault,
		Threshold: 3,
		MaxPerRun: 1,
	})
	if err != nil {
		t.Fatalf("GenerateAuto: %v", err)
	}
	if len(res.Profiled) != 1 {
		t.Errorf("Profiled = %d, want 1 (capped)", len(res.Profiled))
	}
}

func TestGenerateAuto_SkipsAlreadyProfiledFromLog(t *testing.T) {
	vault := t.TempDir()
	writeNote(t, vault, "notes/fleeting/a.md", `---
title: Many Mentions
---

Sam Kriss. Sam Kriss. Sam Kriss. Sam Kriss. Sam Kriss. Sam Kriss.
`)
	writeNote(t, vault, "_meta/entity-profiles-log.md", `# Entity Profiles Log

- Sam Kriss — 2026-05-20 — notes/entities/2026-05-20-profile-sam-kriss.md
`)
	exec := newAutoExecutor(t, "[]" /* not reached */)
	res, err := GenerateAuto(context.Background(), exec, AutoInput{
		VaultRoot: vault,
		Threshold: 3,
	})
	if err != nil {
		t.Fatalf("GenerateAuto: %v", err)
	}
	if len(res.Profiled) != 0 {
		t.Errorf("Profiled = %v, want 0 (already in log)", res.Profiled)
	}
	if !contains(res.Skipped, "Sam Kriss") {
		t.Errorf("Skipped = %v, want to include Sam Kriss", res.Skipped)
	}
}

func TestGenerateAuto_SkipsExcludedDirs(t *testing.T) {
	vault := t.TempDir()
	// Note inside _meta/ should not contribute to the entity counts.
	writeNote(t, vault, "_meta/log.md", strings.Repeat("Excluded Phrase. ", 10))
	exec := newAutoExecutor(t, "[]")
	res, err := GenerateAuto(context.Background(), exec, AutoInput{
		VaultRoot: vault,
		Threshold: 3,
	})
	if err != nil {
		t.Fatalf("GenerateAuto: %v", err)
	}
	if res.Considered != 0 {
		t.Errorf("Considered = %d, want 0", res.Considered)
	}
}

// --- helpers ---
//
// findRepoRoot, mustWriteFile, mustRead, loadRealSkills already
// live in entityprofile_test.go and are reused here. Only the
// multi-response executor builder is new.

func writeNote(t *testing.T, vault, rel, content string) {
	t.Helper()
	mustWriteFile(t, vault, rel, content)
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// newAutoExecutor wires the real skill loader with a MockTransport
// that returns the given responses in order. The first response is
// the filter-entity-candidates output; subsequent responses are
// generate-entity-profile bodies.
func newAutoExecutor(t *testing.T, responses ...string) *skill.Executor {
	t.Helper()
	mock := &transport.MockTransport{}
	mock.EnqueueResponses(responses...)
	return skill.NewExecutor(loadRealSkills(t), inferencerouter.NewLocalRouter(), mock)
}
