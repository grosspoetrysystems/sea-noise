package inboxprocess_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sea-noise/internal/ai/inferencerouter"
	"sea-noise/internal/ai/skill"
	"sea-noise/internal/ai/transport"
	"sea-noise/internal/inboxprocess"
)

func TestRun_InvokesSkillWithVaultRoot(t *testing.T) {
	vault := t.TempDir()
	mock := &transport.MockTransport{}
	mock.EnqueueResponses("Done. 3 notes promoted, 0 discarded, 2 new tags.")
	exec := skill.NewExecutor(loadRealSkills(t), inferencerouter.NewLocalRouter(), mock)

	res, err := inboxprocess.Run(context.Background(), exec, inboxprocess.Input{VaultRoot: vault, SkipPaths: "inbox/protected.md"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(res.Report, "3 notes promoted") {
		t.Errorf("Report did not propagate skill output: %q", res.Report)
	}

	// The mock saw exactly one invocation whose rendered prompt
	// includes the vault root — proves the template binding works
	// end-to-end without us needing to assert against the entire prompt.
	if len(mock.Calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(mock.Calls))
	}
	if !strings.Contains(mock.Calls[0].Prompt, vault) {
		t.Errorf("rendered prompt did not contain vault root %q:\n%s", vault, mock.Calls[0].Prompt)
	}
	if !strings.Contains(mock.Calls[0].Prompt, "inbox/protected.md") {
		t.Errorf("rendered prompt omitted hydration-protected path:\n%s", mock.Calls[0].Prompt)
	}
}

func loadRealSkills(t *testing.T) map[string]*skill.Skill {
	t.Helper()
	root := findRepoRoot(t)
	skills, err := skill.LoadFromDir(filepath.Join(root, ".agents/shared/skills/inference"))
	if err != nil {
		t.Fatalf("load skills: %v", err)
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
	t.Fatalf("repo root not found")
	return ""
}
