package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"sea-noise/internal/daemon"
)

// runRun implements `sea-noise run <pipeline>` — the one-shot
// developer/debug entry point for the daemon's pipelines. Same step
// list as `sea-noise serve` fires on the timer, but driven manually.
// Useful for debugging a specific chain, or for users who prefer to
// fire pipelines from a shell script rather than running a resident
// process.
func runRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	skillsDir := fs.String("skills-dir", "", "directory containing inference skill files")
	cliCommand := fs.String("cli-command", "", "executable for the inference CLI (e.g., claude, codex)")
	cliArgsList := fs.String("cli-args", "-p,{PROMPT},--dangerously-skip-permissions", "comma-separated argv template; {PROMPT} substituted")
	timeoutSec := fs.Int("timeout", 1800, "per-inference-call timeout in seconds")
	dryRun := fs.Bool("dry-run", false, "log every step that would run without executing any side effects")
	indexDB := fs.String("index-db", "", "SQLite path for the retrieval index (default ~/.sea-noise/retrieval.db)")
	force := fs.Bool("force", false, "ignore marker file and re-run even if the period already had a successful run")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("run: pipeline name required (inbox|daily|weekly)")
	}
	pipeline := fs.Arg(0)

	deps, err := buildDaemonDeps(*cliCommand, *cliArgsList, *skillsDir, *timeoutSec, *indexDB)
	if err != nil {
		return err
	}

	cfg := daemon.DefaultConfig()
	cfg.VaultRoot = deps.VaultRoot
	cfg.DryRun = *dryRun

	srv := daemon.New(cfg, buildRegistry(deps))
	if *force {
		clearMarker(deps.VaultRoot, pipeline)
	}
	if err := srv.RunPipeline(context.Background(), pipeline, time.Now().UTC()); err != nil {
		return err
	}
	fmt.Printf("sea-noise run %s: completed\n", pipeline)
	return nil
}

// clearMarker removes the marker file for the given pipeline so the
// next RunPipeline call ignores the previous run's gate.
func clearMarker(vaultRoot, pipeline string) {
	switch pipeline {
	case daemon.PipelineDaily:
		_ = removeIfExists(vaultRoot + "/_meta/.daily-last-run")
	case daemon.PipelineWeekly:
		_ = removeIfExists(vaultRoot + "/_meta/.weekly-last-run")
	}
}

func removeIfExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	return os.Remove(path)
}
