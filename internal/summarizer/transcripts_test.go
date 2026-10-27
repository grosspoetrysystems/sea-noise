package summarizer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sea-noise/internal/ai/inferencerouter"
	"sea-noise/internal/ai/skill"
	"sea-noise/internal/ai/transport"
)

func TestEnrichTranscripts_HappyPath(t *testing.T) {
	vault := t.TempDir()
	mustWriteFile(t, vault, "notes/references/podcast-note.md", longTranscriptNote("Demo Title"))

	exec := newTestExecutor(t, "MOCK SUMMARY")

	res, err := EnrichTranscripts(context.Background(), exec, EnrichTranscriptsInput{VaultRoot: vault})
	if err != nil {
		t.Fatalf("EnrichTranscripts: %v", err)
	}
	if len(res.Updated) != 1 || res.Updated[0] != "notes/references/podcast-note.md" {
		t.Fatalf("Updated = %v, want [notes/references/podcast-note.md]", res.Updated)
	}

	updated := mustRead(t, filepath.Join(vault, "notes/references/podcast-note.md"))
	if !strings.Contains(updated, "## Research Context\n\nMOCK SUMMARY") {
		t.Errorf("updated note missing Research Context block:\n%s", updated)
	}
	// Frontmatter preserved.
	if !strings.HasPrefix(updated, "---\ntitle: Demo Title") {
		t.Errorf("frontmatter not preserved at top of note:\n%s", updated[:200])
	}
	// Research Context appears BEFORE Transcript section.
	rcIdx := strings.Index(updated, "## Research Context")
	tIdx := strings.Index(updated, "## Transcript")
	if rcIdx < 0 || tIdx < 0 || rcIdx >= tIdx {
		t.Errorf("Research Context not before Transcript (rcIdx=%d, tIdx=%d)", rcIdx, tIdx)
	}
}

func TestEnrichTranscripts_AlreadyEnrichedSkipped(t *testing.T) {
	vault := t.TempDir()
	mustWriteFile(t, vault, "notes/references/already.md", `---
title: Already Enriched
---

## Research Context

old summary

## Transcript

`+longText(1000))

	exec := newTestExecutor(t, "SHOULD NOT BE CALLED")
	res, err := EnrichTranscripts(context.Background(), exec, EnrichTranscriptsInput{VaultRoot: vault})
	if err != nil {
		t.Fatalf("EnrichTranscripts: %v", err)
	}
	if len(res.Updated) != 0 {
		t.Errorf("Updated = %v, want empty (already enriched)", res.Updated)
	}
	if len(res.Skipped) != 1 {
		t.Errorf("Skipped = %v, want 1 entry", res.Skipped)
	}
}

func TestEnrichTranscripts_ShortTranscriptSkipped(t *testing.T) {
	vault := t.TempDir()
	mustWriteFile(t, vault, "notes/references/short.md", `---
title: Short
---

## Transcript

too short
`)

	exec := newTestExecutor(t, "unused")
	res, err := EnrichTranscripts(context.Background(), exec, EnrichTranscriptsInput{VaultRoot: vault})
	if err != nil {
		t.Fatalf("EnrichTranscripts: %v", err)
	}
	if len(res.Updated) != 0 {
		t.Errorf("Updated = %v, want empty", res.Updated)
	}
	if len(res.Skipped) != 1 {
		t.Errorf("Skipped = %v, want 1 entry", res.Skipped)
	}
}

func TestEnrichTranscripts_NoTranscriptSkipped(t *testing.T) {
	vault := t.TempDir()
	mustWriteFile(t, vault, "notes/references/plain.md", `---
title: Plain
---

just a regular note, no transcript section
`)

	exec := newTestExecutor(t, "unused")
	res, err := EnrichTranscripts(context.Background(), exec, EnrichTranscriptsInput{VaultRoot: vault})
	if err != nil {
		t.Fatalf("EnrichTranscripts: %v", err)
	}
	if len(res.Skipped) != 1 {
		t.Errorf("Skipped = %v, want 1 entry", res.Skipped)
	}
}

func TestEnrichTranscripts_ReadFailureDoesNotAbortLaterNotes(t *testing.T) {
	vault := t.TempDir()
	unreadable := filepath.Join(vault, "notes/references/a-unreadable.md")
	if err := os.MkdirAll(filepath.Dir(unreadable), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(vault, "missing.md"), unreadable); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, vault, "notes/references/z-good.md", longTranscriptNote("Good"))

	exec := newTestExecutor(t, "MOCK SUMMARY")
	res, err := EnrichTranscripts(context.Background(), exec, EnrichTranscriptsInput{VaultRoot: vault})
	if err != nil {
		t.Fatalf("EnrichTranscripts: %v", err)
	}
	if len(res.Failed) != 1 || res.Failed[0] != "notes/references/a-unreadable.md" {
		t.Fatalf("Failed = %v, want [notes/references/a-unreadable.md]", res.Failed)
	}
	if len(res.Updated) != 1 || res.Updated[0] != "notes/references/z-good.md" {
		t.Fatalf("Updated = %v, want [notes/references/z-good.md]", res.Updated)
	}
}

func TestEnrichTranscripts_PromptSentMatchesSkill(t *testing.T) {
	vault := t.TempDir()
	mustWriteFile(t, vault, "notes/references/podcast.md", longTranscriptNote("My Title"))

	mock := &transport.MockTransport{}
	mock.EnqueueResponses("summary")
	exec := skill.NewExecutor(loadRealSkills(t), inferencerouter.NewLocalRouter(), mock)

	_, err := EnrichTranscripts(context.Background(), exec, EnrichTranscriptsInput{VaultRoot: vault})
	if err != nil {
		t.Fatalf("EnrichTranscripts: %v", err)
	}
	if len(mock.Calls) != 1 {
		t.Fatalf("Calls length = %d, want 1", len(mock.Calls))
	}
	prompt := mock.Calls[0].Prompt
	// The skill template has these phrases when rendered with our input.
	wants := []string{
		"a YouTube video transcript",
		"about 'My Title'",
		"### Key Takeaways",
		"### Follow-up Questions",
	}
	for _, w := range wants {
		if !strings.Contains(prompt, w) {
			t.Errorf("prompt missing %q. Prompt was:\n%s", w, prompt)
		}
	}
}

// === helpers ===

func newTestExecutor(t *testing.T, response string) *skill.Executor {
	t.Helper()
	mock := &transport.MockTransport{}
	mock.EnqueueResponses(response)
	return skill.NewExecutor(loadRealSkills(t), inferencerouter.NewLocalRouter(), mock)
}

func loadRealSkills(t *testing.T) map[string]*skill.Skill {
	t.Helper()
	root := findRepoRoot(t)
	skills, err := skill.LoadFromDir(filepath.Join(root, ".agents/shared/skills/inference"))
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	return skills
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for d := wd; d != "/" && d != "."; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
	}
	t.Fatal("could not locate repo root from " + wd)
	return ""
}

func mustWriteFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestWalkTranscriptCandidates(t *testing.T) {
	vault := t.TempDir()
	mustWriteFile(t, vault, "notes/references/qualifying.md", longTranscriptNote("Qualifying"))
	mustWriteFile(t, vault, "notes/references/already.md", `---
title: Already Enriched
---

## Research Context

prior summary

## Transcript

`+longText(1000))
	mustWriteFile(t, vault, "notes/references/short.md", `---
title: Short
---

## Transcript

too short
`)
	mustWriteFile(t, vault, "notes/references/no-transcript.md", `---
title: Plain
---

just a note, no transcript header
`)

	cands, err := WalkTranscriptCandidates(EnrichTranscriptsInput{VaultRoot: vault})
	if err != nil {
		t.Fatalf("WalkTranscriptCandidates: %v", err)
	}
	if len(cands) != 1 {
		t.Fatalf("len(cands) = %d, want 1; got %+v", len(cands), cands)
	}
	got := cands[0]
	if got.Rel != "notes/references/qualifying.md" {
		t.Errorf("Rel = %q, want notes/references/qualifying.md", got.Rel)
	}
	if got.SkillInput.SourceDescription != "a YouTube video transcript" {
		t.Errorf("SourceDescription = %q", got.SkillInput.SourceDescription)
	}
	if got.SkillInput.TitleContext != " about 'Qualifying'" {
		t.Errorf("TitleContext = %q, want \" about 'Qualifying'\"", got.SkillInput.TitleContext)
	}
	if !strings.Contains(got.SkillInput.Content, "the quick brown fox") {
		t.Errorf("Content missing transcript body (got first 80 chars: %q)", got.SkillInput.Content[:min(80, len(got.SkillInput.Content))])
	}
}

func longTranscriptNote(title string) string {
	return `---
title: ` + title + `
source: https://youtube.com/watch?v=abc
---

# ` + title + `

Some intro.

## Transcript

` + longText(MinContentChars+100) + `
`
}

func longText(n int) string {
	const lorem = "the quick brown fox jumps over the lazy dog. "
	var b strings.Builder
	for b.Len() < n {
		b.WriteString(lorem)
	}
	return b.String()[:n]
}
