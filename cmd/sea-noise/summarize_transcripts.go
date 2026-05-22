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

	skillsPath := *skillsDir
	if skillsPath == "" {
		skillsPath = guessSkillsDir()
	}
	skills, err := skill.LoadFromDir(skillsPath)
	if err != nil {
		return fmt.Errorf("load skills from %s: %w", skillsPath, err)
	}
	if _, ok := skills["summarize-content"]; !ok {
		return errors.New("summarize-content skill not found; check --skills-dir")
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

// guessSkillsDir resolves the default skills directory by walking up
// from the binary's location (and CWD) until it finds .agents/shared/
// skills/inference. Returns the empty string if it can't find one;
// the caller surfaces a clear error.
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
