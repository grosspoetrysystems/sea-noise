package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"sea-noise/internal/ai/inferencerouter"
	"sea-noise/internal/ai/skill"
	"sea-noise/internal/ai/transport"
	"sea-noise/internal/config"
	"sea-noise/internal/summarizer"
)

// runSummarizeTranscripts implements `sea-noise summarize-transcripts`.
// Walks the vault for notes containing `## Transcript`, enriches each
// with a `## Research Context` section via the summarize-content skill.
//
// Mirrors the transcript path of references/scripts/summarize-transcripts.py.
// Webclip and tag-enrichment paths are not yet ported.
func runSummarizeTranscripts(args []string) error {
	fs := flag.NewFlagSet("summarize-transcripts", flag.ExitOnError)
	skillsDir := fs.String("skills-dir", "", "directory containing inference skill files (defaults to .agents/shared/skills/inference relative to repo root or binary)")
	cliCommand := fs.String("cli-command", "", "executable for the inference CLI (e.g., claude, codex)")
	cliArgsList := fs.String("cli-args", "-p,{PROMPT},--dangerously-skip-permissions", "comma-separated argv template; {PROMPT} is substituted with the prompt at call time")
	timeoutSec := fs.Int("timeout", 300, "per-inference-call timeout in seconds")
	if err := fs.Parse(args); err != nil {
		return err
	}

	vaultRoot, err := config.LoadVaultPath()
	if err != nil {
		return err
	}

	skills, err := loadSkills(*skillsDir)
	if err != nil {
		return fmt.Errorf("load skills: %w", err)
	}
	if _, ok := skills["summarize-content"]; !ok {
		return errors.New("summarize-content skill not found; pass --skills-dir to override embedded skills")
	}

	if *cliCommand == "" {
		return errors.New("--cli-command required (e.g., claude, codex). Inference transport is user-configured per the agent-agnostic stance")
	}

	tx := &transport.CLITransport{
		Command: *cliCommand,
		Args:    splitCSV(*cliArgsList),
		Timeout: time.Duration(*timeoutSec) * time.Second,
		EnvOverrides: map[string]string{
			// Parity with the prototype: when the host process is itself
			// running under a claude harness, this env var allows
			// nested invocation. Harmless for non-claude transports.
			"CLAUDECODE": "",
		},
	}

	exec := skill.NewExecutor(skills, inferencerouter.NewLocalRouter(), tx)

	res, err := summarizer.EnrichTranscripts(context.Background(), exec, summarizer.EnrichTranscriptsInput{
		VaultRoot: vaultRoot,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Updated:  %d\n", len(res.Updated))
	for _, p := range res.Updated {
		fmt.Printf("  ✓ %s\n", p)
	}
	fmt.Printf("Skipped:  %d\n", len(res.Skipped))
	fmt.Printf("Failed:   %d\n", len(res.Failed))
	for _, p := range res.Failed {
		fmt.Printf("  ✗ %s\n", p)
	}
	return nil
}

// loadSkills returns the inference skill set: if skillsDir is set
// explicitly (--skills-dir flag), reads from that on-disk path; else
// uses the skills baked into the binary at build time via
// skill.LoadEmbedded. The embedded path is what makes the shipped
// binary self-contained — `sea-noise serve` works from anywhere
// without a repo checkout. Dev-time skill iteration: pass
// --skills-dir <repo>/.agents/shared/skills/inference to pick up
// live edits.
func loadSkills(skillsDir string) (map[string]*skill.Skill, error) {
	if skillsDir != "" {
		return skill.LoadFromDir(skillsDir)
	}
	return skill.LoadEmbedded()
}

// guessSkillsDir resolves the default skills directory by walking up
// from the binary's location (and CWD) until it finds .agents/shared/
// skills/inference. Returns the empty string if it can't find one;
// the caller surfaces a clear error.
//
// Retained for parity-discover's --skills-dir flag where dev-tree
// auto-detection is genuinely useful (capture scripts otherwise
// would need absolute paths). Not used by the daemon or one-shot
// subcommands — those default to embedded via loadSkills.
func guessSkillsDir() string {
	candidates := []string{}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(exe))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, wd)
	}
	for _, c := range candidates {
		for d := c; d != "/" && d != "."; d = filepath.Dir(d) {
			candidate := filepath.Join(d, ".agents", "shared", "skills", "inference")
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate
			}
		}
	}
	return ".agents/shared/skills/inference"
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	out := []string{}
	cur := ""
	for _, c := range s {
		if c == ',' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(c)
	}
	out = append(out, cur)
	return out
}
