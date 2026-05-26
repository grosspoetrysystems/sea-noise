package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"sea-noise/internal/ai/inferencerouter"
	"sea-noise/internal/ai/skill"
	"sea-noise/internal/ai/transport"
	"sea-noise/internal/config"
	"sea-noise/internal/inboxprocess"
)

// runProcessInbox implements `sea-noise process-inbox`. Loads the
// inbox-process skill and invokes it via the configured inference
// CLI. The skill itself instructs the LLM to walk the inbox, triage,
// link, write _meta/topic-frequencies.md + rejected-connections.md +
// atlas updates, and promote notes — see
// .agents/shared/skills/inference/inbox-process.md.
func runProcessInbox(args []string) error {
	fs := flag.NewFlagSet("process-inbox", flag.ExitOnError)
	skillsDir := fs.String("skills-dir", "", "directory containing inference skill files")
	cliCommand := fs.String("cli-command", "", "executable for the inference CLI (e.g., claude, codex)")
	cliArgsList := fs.String("cli-args", "-p,{PROMPT},--dangerously-skip-permissions", "comma-separated argv template; {PROMPT} is substituted")
	timeoutSec := fs.Int("timeout", 1800, "timeout for the inbox-process invocation in seconds (default 30 minutes)")
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
	if _, ok := skills[inboxprocess.SkillName]; !ok {
		return errors.New("inbox-process skill not found; check --skills-dir")
	}
	if *cliCommand == "" {
		return errors.New("--cli-command required (e.g., claude, codex)")
	}

	tx := &transport.CLITransport{
		Command:      *cliCommand,
		Args:         splitCSV(*cliArgsList),
		Timeout:      time.Duration(*timeoutSec) * time.Second,
		EnvOverrides: map[string]string{"CLAUDECODE": ""},
	}
	exec := skill.NewExecutor(skills, inferencerouter.NewLocalRouter(), tx)

	res, err := inboxprocess.Run(context.Background(), exec, inboxprocess.Input{VaultRoot: vaultRoot})
	if err != nil {
		return err
	}
	fmt.Println(res.Report)
	return nil
}
