package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
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
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	skillsDir := fs.String("skills-dir", "", "directory containing inference skill files")
	cliCommand := fs.String("cli-command", "", "executable for the inference CLI (e.g., claude, codex)")
	cliArgsList := fs.String("cli-args", "-p,{PROMPT},--dangerously-skip-permissions", "comma-separated argv template; {PROMPT} substituted")
	timeoutSec := fs.Int("timeout", 1800, "per-inference-call timeout in seconds")
	dryRun := fs.Bool("dry-run", false, "log every step that would run without executing pipeline steps")
	indexDB := fs.String("index-db", "", "SQLite path for the retrieval index (default ~/.sea-noise/retrieval.db)")
	force := fs.Bool("force", false, "ignore marker file and re-run even if the period already had a successful run")
	pipeline, flagArgs, err := splitRunArgs(fs, args)
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("run: unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *force && pipeline == daemon.PipelineInbox {
		return fmt.Errorf("run: --force is only valid for daily or weekly pipelines")
	}

	deps, err := buildDaemonDeps(*cliCommand, *cliArgsList, *skillsDir, *timeoutSec, *indexDB)
	if err != nil {
		return err
	}

	cfg := daemon.DefaultConfig()
	cfg.VaultRoot = deps.VaultRoot
	cfg.DryRun = *dryRun

	srv := daemon.New(cfg, buildRegistry(deps))
	if *force {
		if err := clearMarker(deps.VaultRoot, pipeline); err != nil {
			return fmt.Errorf("run: clear marker: %w", err)
		}
	}
	if err := srv.RunPipeline(context.Background(), pipeline, time.Now().UTC()); err != nil {
		return err
	}
	fmt.Printf("sea-noise run %s: completed\n", pipeline)
	return nil
}

func splitRunArgs(fs *flag.FlagSet, args []string) (string, []string, error) {
	var pipeline string
	flagArgs := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case daemon.PipelineInbox, daemon.PipelineDaily, daemon.PipelineWeekly:
			if pipeline != "" {
				return "", nil, fmt.Errorf("run: multiple pipelines: %q and %q", pipeline, arg)
			}
			pipeline = arg
			continue
		}
		flagArgs = append(flagArgs, arg)
		name := arg
		eq := strings.IndexByte(name, '=')
		if eq >= 0 {
			name = name[:eq]
		}
		if !strings.HasPrefix(name, "-") || eq >= 0 {
			continue
		}
		flag := fs.Lookup(strings.TrimLeft(name, "-"))
		if flag == nil {
			continue
		}
		if boolFlag, ok := flag.Value.(interface{ IsBoolFlag() bool }); ok && boolFlag.IsBoolFlag() {
			continue
		}
		if i+1 == len(args) {
			return "", nil, fmt.Errorf("run: value required for %s", arg)
		}
		i++
		flagArgs = append(flagArgs, args[i])
	}
	if pipeline == "" {
		return "", nil, fmt.Errorf("run: pipeline name required (inbox|daily|weekly)")
	}
	return pipeline, flagArgs, nil
}

// clearMarker removes the marker file for the given pipeline so the
// next RunPipeline call ignores the previous run's gate.
func clearMarker(vaultRoot, pipeline string) error {
	switch pipeline {
	case daemon.PipelineDaily:
		return removeIfExists(vaultRoot + "/_meta/.daily-last-run")
	case daemon.PipelineWeekly:
		return removeIfExists(vaultRoot + "/_meta/.weekly-last-run")
	default:
		return nil
	}
}

func removeIfExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return os.Remove(path)
}
