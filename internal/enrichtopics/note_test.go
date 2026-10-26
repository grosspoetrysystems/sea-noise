package enrichtopics_test

import (
	"context"
	"errors"
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

func TestEnrichNote_HappyPath(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, vault, "notes/fleeting/post.md", `---
title: A Thought
tags: [tech/ai]
source: https://example.com/post
---

A short note about the topic.
`)

	exec := newNoteExecutor(t, "### Follow-up Questions\n- Q1\n\n### Related Materials\n- M1\n\n### Book & Podcast Recommendations\n- B1\n")
	res, err := enrichtopics.EnrichNote(context.Background(), exec, crawl.NoopCrawler{}, enrichtopics.NoteInput{
		VaultRoot: vault,
		NotePath:  "notes/fleeting/post.md",
		Today:     time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("EnrichNote: %v", err)
	}
	if res.NotePath != "notes/fleeting/post.md" {
		t.Errorf("NotePath = %q", res.NotePath)
	}
	got := readFile(t, vault, "notes/fleeting/post.md")
	if !strings.Contains(got, "### Follow-up Questions") {
		t.Errorf("enrichment not appended:\n%s", got)
	}
	if !strings.Contains(got, "enriched: '2026-05-26'") {
		t.Errorf("frontmatter missing enriched key:\n%s", got)
	}
	// Pre-existing keys + body survive.
	if !strings.HasPrefix(got, "---\ntitle: A Thought\ntags: [tech/ai]\nsource: https://example.com/post") {
		t.Errorf("frontmatter prefix mutated:\n%s", got[:200])
	}
	if !strings.Contains(got, "A short note about the topic.") {
		t.Errorf("body removed:\n%s", got)
	}
	// Log line written.
	logContent := readFile(t, vault, "_meta/enrichment-log.md")
	if !strings.Contains(logContent, "post — 2026-05-26") {
		t.Errorf("log missing entry:\n%s", logContent)
	}
}

func TestEnrichNote_NotFound(t *testing.T) {
	vault := t.TempDir()
	exec := newNoteExecutor(t, "")
	_, err := enrichtopics.EnrichNote(context.Background(), exec, crawl.NoopCrawler{}, enrichtopics.NoteInput{
		VaultRoot: vault,
		NotePath:  "notes/fleeting/missing.md",
	})
	if !errors.Is(err, enrichtopics.ErrNoteNotFound) {
		t.Fatalf("err = %v, want ErrNoteNotFound", err)
	}
}

func TestEnrichNote_AlreadyEnriched(t *testing.T) {
	cases := []struct {
		name, marker string
	}{
		{"research_context", "## Research Context"},
		{"followup_questions", "## Follow-up Questions"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vault := t.TempDir()
			writeFile(t, vault, "notes/fleeting/done.md", "---\ntitle: X\n---\nbody\n\n"+tc.marker+"\n\nx\n")
			exec := newNoteExecutor(t, "SHOULD NOT BE CALLED")
			_, err := enrichtopics.EnrichNote(context.Background(), exec, crawl.NoopCrawler{}, enrichtopics.NoteInput{
				VaultRoot: vault,
				NotePath:  "notes/fleeting/done.md",
			})
			if !errors.Is(err, enrichtopics.ErrAlreadyEnriched) {
				t.Errorf("err = %v, want ErrAlreadyEnriched", err)
			}
		})
	}
}

func TestEnrichNote_DoesNotOverwriteExistingEnrichedKey(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, vault, "notes/fleeting/p.md", `---
title: Stable
enriched: '2026-05-01'
---

body
`)
	exec := newNoteExecutor(t, "### Follow-up Questions\n- Q\n")
	_, err := enrichtopics.EnrichNote(context.Background(), exec, crawl.NoopCrawler{}, enrichtopics.NoteInput{
		VaultRoot: vault,
		NotePath:  "notes/fleeting/p.md",
		Today:     time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("EnrichNote: %v", err)
	}
	got := readFile(t, vault, "notes/fleeting/p.md")
	if !strings.Contains(got, "enriched: '2026-05-01'") {
		t.Errorf("existing enriched date overwritten:\n%s", got)
	}
	if strings.Contains(got, "enriched: '2026-05-26'") {
		t.Errorf("new enriched date inserted on top of existing:\n%s", got)
	}
}

// --- helpers ---

func readFile(t *testing.T, vault, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(vault, rel))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(b)
}

func newNoteExecutor(t *testing.T, response string) *skill.Executor {
	t.Helper()
	root := findRepoRoot(t)
	skills, err := skill.LoadFromDir(filepath.Join(root, ".agents/shared/skills/inference"))
	if err != nil {
		t.Fatalf("load skills: %v", err)
	}
	mock := &transport.MockTransport{}
	mock.EnqueueResponses(response)
	return skill.NewExecutor(skills, inferencerouter.NewLocalRouter(), mock)
}
