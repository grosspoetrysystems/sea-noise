package entityprofile

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
)

const sampleProfile = `### Overview
Sample person.

### Key Ideas & Contributions
- Founded a thing.

### Related Entities
- Other Person.

### External Links
- Unknown.

### Vault Presence
3 mentions across the vault.`

func TestGenerate_HappyPath(t *testing.T) {
	vault := t.TempDir()
	mustWriteFile(t, vault, "notes/fleeting/note1.md", `---
title: First Mention
---

Met Sam Kriss at a thing. He writes well.
`)
	mustWriteFile(t, vault, "notes/references/note2.md", `---
title: Second Note
---

Sam Kriss has an essay on the topic.
`)
	mustWriteFile(t, vault, "notes/fleeting/unrelated.md", `---
title: Unrelated
---

Nothing relevant here.
`)

	exec := newTestExecutor(t, sampleProfile)

	res, err := Generate(context.Background(), exec, Input{
		VaultRoot: vault,
		Entity:    "Sam Kriss",
		Today:     time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if res.Mentions != 2 {
		t.Errorf("Mentions = %d, want 2", res.Mentions)
	}
	wantProfileRel := "notes/entities/2026-05-23-profile-sam-kriss.md"
	if res.ProfilePath != wantProfileRel {
		t.Errorf("ProfilePath = %q, want %q", res.ProfilePath, wantProfileRel)
	}

	// Verify profile content
	profile := mustRead(t, filepath.Join(vault, wantProfileRel))
	if !strings.Contains(profile, `title: "Sam Kriss"`) {
		t.Errorf("profile missing title:\n%s", profile)
	}
	if !strings.Contains(profile, "mention-count: 2") {
		t.Errorf("profile missing mention-count:\n%s", profile)
	}
	if !strings.Contains(profile, "### Overview\nSample person.") {
		t.Errorf("profile body missing skill output:\n%s", profile)
	}
	if !strings.Contains(profile, "## Vault Backlinks\n- [[note1]]") {
		t.Errorf("profile missing backlinks:\n%s", profile)
	}

	// Verify log entry
	log := mustRead(t, filepath.Join(vault, "_meta/entity-profiles-log.md"))
	if !strings.Contains(log, "- Sam Kriss — 2026-05-23 — notes/entities/2026-05-23-profile-sam-kriss.md") {
		t.Errorf("log missing entry:\n%s", log)
	}
}

func TestGenerate_NoMentions(t *testing.T) {
	vault := t.TempDir()
	mustWriteFile(t, vault, "notes/fleeting/note1.md", "nothing here\n")

	exec := newTestExecutor(t, "should not be called")
	_, err := Generate(context.Background(), exec, Input{
		VaultRoot: vault,
		Entity:    "NonExistent Entity",
		Today:     time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC),
	})
	if !errors.Is(err, ErrEntityNotMentioned) {
		t.Errorf("err = %v, want errors.Is(err, ErrEntityNotMentioned)", err)
	}
}

func TestGenerate_PromptShape(t *testing.T) {
	vault := t.TempDir()
	mustWriteFile(t, vault, "notes/fleeting/note1.md", `---
title: Note One
---

Sam Kriss is mentioned here. He writes essays.
`)

	mock := &transport.MockTransport{}
	mock.EnqueueResponses(sampleProfile)
	exec := skill.NewExecutor(loadRealSkills(t), inferencerouter.NewLocalRouter(), mock)

	_, err := Generate(context.Background(), exec, Input{
		VaultRoot: vault,
		Entity:    "Sam Kriss",
		Today:     time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(mock.Calls) != 1 {
		t.Fatalf("Calls = %d, want 1", len(mock.Calls))
	}
	prompt := mock.Calls[0].Prompt
	for _, want := range []string{
		"profile note for 'Sam Kriss'",
		"## Vault Mentions (1 notes)",
		"### Overview",
		"### Related Entities",
		"Note One",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q\nfull prompt:\n%s", want, prompt)
		}
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Sam Kriss", "sam-kriss"},
		{"OpenAI", "openai"},
		{"AT&T Labs", "at-t-labs"},
		{"  spaces  around  ", "spaces-around"},
		{"already-slug", "already-slug"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := slugify(c.in); got != c.want {
				t.Errorf("slugify(%q) = %q, want %q", c.in, got, c.want)
			}
		})
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
