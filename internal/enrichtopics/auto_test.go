package enrichtopics_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sea-noise/internal/ai/inferencerouter"
	"sea-noise/internal/ai/skill"
	"sea-noise/internal/ai/transport"
	"sea-noise/internal/crawl"
	"sea-noise/internal/enrichtopics"
)

func TestReadTopicFrequencies_FourColShape(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, vault, "_meta/topic-frequencies.md", `# Topics

| tag | count | last_seen | trend |
|-----|-------|-----------|-------|
| tech/ai | 12 | 2026-05-22 | rising |
| life/cooking | 3 | 2026-05-21 | stable |
| ephemera | 1 | 2026-05-20 |  |
`)
	got, err := enrichtopics.ReadTopicFrequencies(vault)
	if err != nil {
		t.Fatalf("ReadTopicFrequencies: %v", err)
	}
	want := []enrichtopics.TopicEntry{
		{Topic: "tech/ai", Count: 12, LastSeen: "2026-05-22", Trend: "rising"},
		{Topic: "life/cooking", Count: 3, LastSeen: "2026-05-21", Trend: "stable"},
		{Topic: "ephemera", Count: 1, LastSeen: "2026-05-20"},
	}
	if !entriesEqual(got, want) {
		t.Errorf("got = %+v\nwant = %+v", got, want)
	}
}

func TestReadTopicFrequencies_FiveColPrototype(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, vault, "_meta/topic-frequencies.md", `| topic | first_seen | last_seen | count | trend |
| --- | --- | --- | --- | --- |
| reranking | 2026-04-01 | 2026-05-20 | 7 | rising |
`)
	got, err := enrichtopics.ReadTopicFrequencies(vault)
	if err != nil {
		t.Fatalf("ReadTopicFrequencies: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	e := got[0]
	if e.Topic != "reranking" || e.Count != 7 || e.FirstSeen != "2026-04-01" || e.LastSeen != "2026-05-20" || e.Trend != "rising" {
		t.Errorf("entry = %+v, want reranking/7/2026-04-01/2026-05-20/rising", e)
	}
}

func TestReadTopicFrequencies_MissingFileIsEmpty(t *testing.T) {
	got, err := enrichtopics.ReadTopicFrequencies(t.TempDir())
	if err != nil {
		t.Fatalf("ReadTopicFrequencies: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d entries, want 0", len(got))
	}
}

func TestReadEnrichedTopics_ParsesLogLines(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, vault, "_meta/enrichment-log.md", `# Enrichment Log

- tech/ai — 2026-05-20 — notes/references/briefs/2026-05-20-brief-tech-ai.md
- reranking — 2026-05-22 — notes/references/briefs/2026-05-22-brief-reranking.md
random line
`)
	got, err := enrichtopics.ReadEnrichedTopics(vault)
	if err != nil {
		t.Fatalf("ReadEnrichedTopics: %v", err)
	}
	if !got["tech/ai"] || !got["reranking"] {
		t.Errorf("missing expected entries: %v", got)
	}
	if len(got) != 2 {
		t.Errorf("got %v, want exactly 2 entries", got)
	}
}

func TestEnrichTopicsAuto_FiltersAndCaps(t *testing.T) {
	vault := t.TempDir()
	// topic-frequencies with three rising topics — two should pass the
	// threshold; the cap will trim further.
	writeFile(t, vault, "_meta/topic-frequencies.md", `| tag | count | last_seen | trend |
|-----|-------|-----------|-------|
| reranking | 7 | 2026-05-22 | rising |
| llm-evals | 5 | 2026-05-21 | rising |
| garden | 2 | 2026-05-20 | stable |
`)
	// One note per topic so EnrichTopic finds matches.
	writeFile(t, vault, "notes/fleeting/r1.md", "---\ntitle: R\ntags: [reranking]\n---\nbody\n")
	writeFile(t, vault, "notes/fleeting/l1.md", "---\ntitle: L\ntags: [llm-evals]\n---\nbody\n")
	// Mark one as already enriched so it gets skipped.
	writeFile(t, vault, "_meta/enrichment-log.md", "- llm-evals — 2026-05-22 — notes/references/briefs/x.md\n")

	exec := newTestExecutor(t, "ENRICHMENT BODY")
	crawler := crawl.NoopCrawler{}

	res, err := enrichtopics.EnrichTopicsAuto(context.Background(), exec, crawler, enrichtopics.AutoInput{
		VaultRoot: vault,
		Threshold: 3,
		MaxPerRun: 5,
		Today:     time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("EnrichTopicsAuto: %v", err)
	}
	if len(res.Enriched) != 1 {
		t.Fatalf("Enriched = %d, want 1 (one above threshold, the other already in log)", len(res.Enriched))
	}
	if res.Enriched[0].BriefPath == "" {
		t.Errorf("brief path empty: %+v", res.Enriched[0])
	}
}

func TestEnrichTopicsAuto_RespectsMaxPerRun(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, vault, "_meta/topic-frequencies.md", `| tag | count | last_seen | trend |
|-----|-------|-----------|-------|
| a | 10 | 2026-05-22 | rising |
| b | 9 | 2026-05-22 | rising |
| c | 8 | 2026-05-22 | rising |
`)
	writeFile(t, vault, "notes/fleeting/a.md", "---\ntitle: A\ntags: [a]\n---\nbody\n")
	writeFile(t, vault, "notes/fleeting/b.md", "---\ntitle: B\ntags: [b]\n---\nbody\n")
	writeFile(t, vault, "notes/fleeting/c.md", "---\ntitle: C\ntags: [c]\n---\nbody\n")

	exec := newTestExecutor(t, "BRIEF")
	res, err := enrichtopics.EnrichTopicsAuto(context.Background(), exec, crawl.NoopCrawler{}, enrichtopics.AutoInput{
		VaultRoot: vault,
		Threshold: 3,
		MaxPerRun: 2,
		Today:     time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("EnrichTopicsAuto: %v", err)
	}
	if len(res.Enriched) != 2 {
		t.Errorf("Enriched = %d, want 2 (capped at MaxPerRun)", len(res.Enriched))
	}
}

// --- WalkTopicCandidates (parity-discovery surface) ---

func TestWalkTopicCandidates_Happy(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, vault, "_meta/topic-frequencies.md", `| tag | count | last_seen | trend |
|-----|-------|-----------|-------|
| ai | 5 | 2026-05-22 | rising |
`)
	writeFile(t, vault, "notes/references/ai-paper.md", `---
title: AI Paper
tags:
  - tech/ai
---

A note tagged with ai.
`)
	writeFile(t, vault, "notes/references/another-ai.md", `---
title: Another AI Note
tags:
  - ai
---

Body.
`)

	got, err := enrichtopics.WalkTopicCandidates(
		context.Background(),
		enrichtopics.AutoInput{VaultRoot: vault, Threshold: 3},
		enrichtopics.NoopCrawler{},
	)
	if err != nil {
		t.Fatalf("WalkTopicCandidates: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Topic != "ai" {
		t.Errorf("Topic = %q, want %q", got[0].Topic, "ai")
	}
	if got[0].SkillInput.NoteCount != 2 {
		t.Errorf("NoteCount = %d, want 2", got[0].SkillInput.NoteCount)
	}
	if got[0].SkillInput.WebSection != "" {
		t.Errorf("WebSection = %q, want empty (NoopCrawler + no source URLs)", got[0].SkillInput.WebSection)
	}
}

func TestWalkTopicCandidates_RespectsThreshold(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, vault, "_meta/topic-frequencies.md", `| tag | count | last_seen | trend |
|-----|-------|-----------|-------|
| sparse | 1 | 2026-05-20 |  |
`)
	writeFile(t, vault, "notes/sparse.md", "---\ntitle: x\ntags:\n  - sparse\n---\nbody\n")

	got, err := enrichtopics.WalkTopicCandidates(
		context.Background(),
		enrichtopics.AutoInput{VaultRoot: vault, Threshold: 3},
		nil,
	)
	if err != nil {
		t.Fatalf("WalkTopicCandidates: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0 (below threshold)", len(got))
	}
}

func TestWalkTopicCandidates_SkipsAlreadyEnriched(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, vault, "_meta/topic-frequencies.md", `| tag | count | last_seen | trend |
|-----|-------|-----------|-------|
| ai | 5 | 2026-05-22 | rising |
`)
	writeFile(t, vault, "_meta/enrichment-log.md", `# Enrichment Log

- ai — 2026-05-20 — notes/references/briefs/2026-05-20-ai-research-brief.md
`)
	writeFile(t, vault, "notes/ai.md", "---\ntitle: x\ntags:\n  - ai\n---\nbody\n")

	got, err := enrichtopics.WalkTopicCandidates(
		context.Background(),
		enrichtopics.AutoInput{VaultRoot: vault, Threshold: 3},
		nil,
	)
	if err != nil {
		t.Fatalf("WalkTopicCandidates: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0 (already in enrichment-log)", len(got))
	}
}

func TestWalkTopicCandidates_UsesStubCrawler(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, vault, "_meta/topic-frequencies.md", `| tag | count | last_seen | trend |
|-----|-------|-----------|-------|
| ai | 5 | 2026-05-22 | rising |
`)
	writeFile(t, vault, "notes/ai.md", `---
title: AI Note
tags:
  - ai
source: https://example.com/ai-article
---

Body.
`)

	stubsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(stubsDir, "ai.md"), []byte("# Stubbed AI Web Content\n\nLorem ipsum about AI.\n"), 0o644); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	stub, err := crawl.NewStubCrawler(stubsDir)
	if err != nil {
		t.Fatalf("NewStubCrawler: %v", err)
	}

	got, err := enrichtopics.WalkTopicCandidates(
		context.Background(),
		enrichtopics.AutoInput{VaultRoot: vault, Threshold: 3},
		stub,
	)
	if err != nil {
		t.Fatalf("WalkTopicCandidates: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if !strings.Contains(got[0].SkillInput.WebSection, "Stubbed AI Web Content") {
		t.Errorf("WebSection missing stub content: %q", got[0].SkillInput.WebSection)
	}
}

// --- helpers ---

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func entriesEqual(a, b []enrichtopics.TopicEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// newTestExecutor wires the real skill loader against an echo
// transport that returns the given response for any prompt. This
// exercises the full template + execution path without burning
// inference tokens.
func newTestExecutor(t *testing.T, response string) *skill.Executor {
	t.Helper()
	repoRoot := findRepoRoot(t)
	skillsPath := filepath.Join(repoRoot, ".agents/shared/skills/inference")
	skills, err := skill.LoadFromDir(skillsPath)
	if err != nil {
		t.Fatalf("load skills: %v", err)
	}
	mock := &transport.MockTransport{}
	// Enqueue several responses so a multi-topic auto run doesn't run dry.
	mock.EnqueueResponses(response, response, response, response, response)
	return skill.NewExecutor(skills, inferencerouter.NewLocalRouter(), mock)
}
