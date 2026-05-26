package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sea-noise/internal/ai/inferencerouter"
	"sea-noise/internal/ai/skill"
	"sea-noise/internal/ai/transport"
	"sea-noise/internal/config"
	"sea-noise/internal/crawl"
	"sea-noise/internal/daemon"
	"sea-noise/internal/todoist"
)

// runServe implements `sea-noise serve`. Starts the resident daemon
// process: an in-process scheduler firing the daily and weekly
// pipelines on Go timers, with steps wired to the in-process workflow
// packages. The daemon stays running until the process receives
// SIGTERM/SIGINT or its parent context is cancelled.
//
// IPC (Unix-socket JSON-RPC) and the fsnotify vault watcher are
// separate subsystems shipped as their own tracks — the daemon
// exposes `Server.RunPipeline` as their plug-in seam.
func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	skillsDir := fs.String("skills-dir", "", "directory containing inference skill files")
	cliCommand := fs.String("cli-command", "", "executable for the inference CLI (e.g., claude, codex). When unset, steps that need inference are skipped.")
	cliArgsList := fs.String("cli-args", "-p,{PROMPT},--dangerously-skip-permissions", "comma-separated argv template; {PROMPT} substituted")
	timeoutSec := fs.Int("timeout", 1800, "per-inference-call timeout in seconds")
	dryRun := fs.Bool("dry-run", false, "log every step that would run without executing any side effects")
	indexDB := fs.String("index-db", "", "SQLite path for the retrieval index (default <vault>/_meta/retrieval.db)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	deps, err := buildDaemonDeps(*cliCommand, *cliArgsList, *skillsDir, *timeoutSec, *indexDB)
	if err != nil {
		return err
	}

	cfg := daemon.DefaultConfig()
	cfg.VaultRoot = deps.VaultRoot
	cfg.DryRun = *dryRun

	srv := daemon.New(cfg, buildRegistry(deps))
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("sea-noise daemon starting (vault: %s, dry-run: %v)\n", deps.VaultRoot, *dryRun)
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("daemon start: %w", err)
	}
	<-ctx.Done()
	fmt.Println("sea-noise daemon stopping...")
	return srv.Stop()
}

// buildDaemonDeps resolves vault + optional collaborators. Errors
// only on hard-required state (vault path); soft-missing creds
// (TODOIST_API_TOKEN, --cli-command) produce a nil dep and the
// dependent step becomes deferred at run time.
func buildDaemonDeps(cliCommand, cliArgsList, skillsDir string, timeoutSec int, indexDB string) (daemonStepDeps, error) {
	vaultRoot, err := config.LoadVaultPath()
	if err != nil {
		return daemonStepDeps{}, err
	}

	deps := daemonStepDeps{
		VaultRoot:   vaultRoot,
		IndexDBPath: indexDB,
		Crawler:     crawl.NewCrawlerFromEnv(),
	}

	if cliCommand != "" {
		skillsPath := skillsDir
		if skillsPath == "" {
			skillsPath = guessSkillsDir()
		}
		skills, err := skill.LoadFromDir(skillsPath)
		if err != nil {
			return daemonStepDeps{}, fmt.Errorf("load skills from %s: %w", skillsPath, err)
		}
		tx := &transport.CLITransport{
			Command:      cliCommand,
			Args:         splitCSV(cliArgsList),
			Timeout:      time.Duration(timeoutSec) * time.Second,
			EnvOverrides: map[string]string{"CLAUDECODE": ""},
		}
		deps.Exec = skill.NewExecutor(skills, inferencerouter.NewLocalRouter(), tx)
	}

	if c, err := todoist.NewFromEnv(); err == nil {
		deps.TodoistClient = c
	} else if !errors.Is(err, todoist.ErrNotConfigured) {
		return daemonStepDeps{}, fmt.Errorf("todoist init: %w", err)
	}

	return deps, nil
}
