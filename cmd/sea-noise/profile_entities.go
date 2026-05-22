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
	"sea-noise/internal/entityprofile"
)

// runProfileEntities implements `sea-noise profile-entities <entity>`.
// Manual mode only — auto-mode (regex NER + Claude-driven candidate
// filtering) is a follow-up port.
func runProfileEntities(args []string) error {
	fs := flag.NewFlagSet("profile-entities", flag.ExitOnError)
	skillsDir := fs.String("skills-dir", "", "directory containing inference skill files")
	cliCommand := fs.String("cli-command", "", "executable for the inference CLI (e.g., claude, codex)")
	cliArgsList := fs.String("cli-args", "-p,{PROMPT},--dangerously-skip-permissions", "comma-separated argv template; {PROMPT} is substituted")
	timeoutSec := fs.Int("timeout", 300, "per-inference-call timeout in seconds")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return errors.New("entity name required: sea-noise profile-entities \"Entity Name\"")
	}
	entity := fs.Arg(0)

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
	if _, ok := skills["generate-entity-profile"]; !ok {
		return errors.New("generate-entity-profile skill not found; check --skills-dir")
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

	res, err := entityprofile.Generate(context.Background(), exec, entityprofile.Input{
		VaultRoot: vaultRoot,
		Entity:    entity,
	})
	if err != nil {
		return err
	}
	fmt.Printf("Profiled: %s\n", entity)
	fmt.Printf("  mentions: %d\n", res.Mentions)
	fmt.Printf("  profile:  %s\n", res.ProfilePath)
	fmt.Printf("  log:      %s\n", res.LogPath)
	return nil
}
