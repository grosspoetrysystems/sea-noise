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

// runProfileEntities implements `sea-noise profile-entities [flags] <entity|--auto>`.
//
// Manual mode: profile one entity by name. Auto mode (--auto): regex
// NER + stopword filter + Claude classification + per-entity profile.
// Mirrors references/scripts/profile-entities.py.
func runProfileEntities(args []string) error {
	fs := flag.NewFlagSet("profile-entities", flag.ExitOnError)
	skillsDir := fs.String("skills-dir", "", "directory containing inference skill files")
	cliCommand := fs.String("cli-command", "", "executable for the inference CLI (e.g., claude, codex)")
	cliArgsList := fs.String("cli-args", "-p,{PROMPT},--dangerously-skip-permissions", "comma-separated argv template; {PROMPT} is substituted")
	timeoutSec := fs.Int("timeout", 300, "per-inference-call timeout in seconds")
	auto := fs.Bool("auto", false, "auto mode: scan the vault for repeated entities and profile every eligible candidate")
	threshold := fs.Int("threshold", entityprofile.DefaultAutoThreshold, "[--auto only] minimum mention count for an entity to be a candidate")
	maxPerRun := fs.Int("max-per-run", entityprofile.DefaultMaxAutoPerRun, "[--auto only] cap on number of profiles written per invocation")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*auto && fs.NArg() == 0 {
		return errors.New("entity name required: sea-noise profile-entities \"Entity Name\" (or pass --auto)")
	}
	if *auto && fs.NArg() > 0 {
		return errors.New("--auto takes no positional entity argument")
	}

	vaultRoot, err := config.LoadVaultPath()
	if err != nil {
		return err
	}

	skills, err := loadSkills(*skillsDir)
	if err != nil {
		return fmt.Errorf("load skills: %w", err)
	}
	if _, ok := skills["generate-entity-profile"]; !ok {
		return errors.New("generate-entity-profile skill not found; pass --skills-dir to override embedded skills")
	}
	if *auto {
		if _, ok := skills["filter-entity-candidates"]; !ok {
			return errors.New("filter-entity-candidates skill not found (required for --auto); pass --skills-dir to override embedded skills")
		}
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

	if *auto {
		res, err := entityprofile.GenerateAuto(context.Background(), exec, entityprofile.AutoInput{
			VaultRoot: vaultRoot,
			Threshold: *threshold,
			MaxPerRun: *maxPerRun,
		})
		if err != nil {
			return err
		}
		fmt.Printf("Considered: %d\n", res.Considered)
		fmt.Printf("Profiled:   %d\n", len(res.Profiled))
		for _, p := range res.Profiled {
			fmt.Printf("  ✓ %s\n", p.ProfilePath)
		}
		fmt.Printf("Skipped:    %d (already in log, below threshold, or filtered out)\n", len(res.Skipped))
		fmt.Printf("Failed:     %d\n", len(res.Failed))
		for _, n := range res.Failed {
			fmt.Printf("  ✗ %s\n", n)
		}
		return nil
	}

	entity := fs.Arg(0)
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
