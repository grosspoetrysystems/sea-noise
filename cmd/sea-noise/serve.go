package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"sea-noise/internal/ai/inferencerouter"
	"sea-noise/internal/ai/skill"
	"sea-noise/internal/ai/transport"
	"sea-noise/internal/config"
	"sea-noise/internal/crawl"
	"sea-noise/internal/daemon"
	"sea-noise/internal/ipc"
	"sea-noise/internal/todoist"
	"sea-noise/internal/vernacular"
	"sea-noise/internal/watcher"
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
	cliCommand := fs.String("cli-command", "", "executable for the inference CLI (e.g., claude, codex). When unset, steps requiring inference report a missing dependency.")
	cliArgsList := fs.String("cli-args", "-p,{PROMPT},--dangerously-skip-permissions", "comma-separated argv template; {PROMPT} substituted")
	timeoutSec := fs.Int("timeout", 1800, "per-inference-call timeout in seconds")
	dryRun := fs.Bool("dry-run", false, "log pipeline steps without executing them; watcher index refreshes and processing logs still write")
	indexDB := fs.String("index-db", "", "SQLite path for the retrieval index (default ~/.sea-noise/retrieval.db)")
	socket := fs.String("socket", "", "Unix-socket path for the JSON-RPC IPC surface (default $SEA_NOISE_SOCKET or ~/.sea-noise/daemon.sock). Empty uses the default path.")
	noIPC := fs.Bool("no-ipc", false, "disable the IPC listener entirely")
	noWatch := fs.Bool("no-watch", false, "disable the fsnotify vault watcher entirely")
	debounceMs := fs.Int("watch-debounce-ms", int(watcher.DefaultDebounce/time.Millisecond), "watcher per-path debounce window in milliseconds")
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

	var ipcSrv *ipc.Server
	if !*noIPC {
		ipcSrv, err = startIPC(ctx, *socket, srv, deps, *dryRun)
		if err != nil {
			_ = srv.Stop()
			return err
		}
	}

	var w *watcher.Watcher
	if !*noWatch {
		w, err = startWatcher(ctx, srv, deps, time.Duration(*debounceMs)*time.Millisecond)
		if err != nil {
			if ipcSrv != nil {
				_ = ipcSrv.Stop()
			}
			_ = srv.Stop()
			return err
		}
	}

	<-ctx.Done()
	fmt.Println("sea-noise daemon stopping...")
	if w != nil {
		_ = w.Stop()
	}
	if ipcSrv != nil {
		_ = ipcSrv.Stop()
	}
	return srv.Stop()
}

// startWatcher keeps registered project targets separate from the vault's
// inbox pipeline. All note and membership changes refresh the derived index.
func startWatcher(ctx context.Context, srv *daemon.Server, deps daemonStepDeps, debounce time.Duration) (*watcher.Watcher, error) {
	sources, err := vernacular.Sources(ctx, deps.VaultRoot)
	if err != nil {
		return nil, fmt.Errorf("watcher sources: %w", err)
	}
	refresh := func(ctx context.Context) {
		if _, err := rebuildIndex(ctx, deps.VaultRoot, deps.IndexDBPath); err != nil {
			fmt.Printf("sea-noise watch index refresh: %v\n", err)
		}
	}
	var sourceChangeMu sync.Mutex
	var w *watcher.Watcher
	w, err = watcher.New(watcher.Config{
		VaultRoot: deps.VaultRoot,
		Sources:   sources,
		Debounce:  debounce,
		Actions: watcher.Actions{
			OnInboxCreate: func(ctx context.Context, _ string) {
				if err := srv.RunPipeline(ctx, daemon.PipelineInbox, time.Now().UTC()); err != nil &&
					!errors.Is(err, daemon.ErrSkippedAlreadyRan) {
					fmt.Printf("sea-noise watch inbox pipeline: %v\n", err)
				}
			},
			OnNoteModify: func(ctx context.Context, path string) {
				if _, ok := w.LogicalPath(path); ok {
					refresh(ctx)
				}
			},
			OnSourcesChange: func(ctx context.Context) {
				sourceChangeMu.Lock()
				defer sourceChangeMu.Unlock()
				sources, err := vernacular.Sources(ctx, deps.VaultRoot)
				if err != nil {
					if suspendErr := w.SuspendSources(); suspendErr != nil {
						fmt.Printf("sea-noise watch suspend sources: %v\n", suspendErr)
					}
					fmt.Printf("sea-noise watch sources: %v\n", err)
					return
				}
				if err := w.ReloadSources(sources); err != nil {
					fmt.Printf("sea-noise watch reload sources: %v\n", err)
					return
				}
				refresh(ctx)
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("watcher new: %w", err)
	}
	if err := w.Start(ctx); err != nil {
		return nil, fmt.Errorf("watcher start: %w", err)
	}
	fmt.Printf("sea-noise watching %s (debounce %s)\n", deps.VaultRoot, debounce)
	return w, nil
}

// startIPC resolves the socket path (--socket > $SEA_NOISE_SOCKET >
// default), validates the sun_path budget, and starts the listener.
// Returns the running Server so the caller can shut it down on exit.
func startIPC(ctx context.Context, flagPath string, srv *daemon.Server, deps daemonStepDeps, dryRun bool) (*ipc.Server, error) {
	socketPath := flagPath
	if socketPath == "" {
		p, err := ipc.DefaultSocketPath()
		if err != nil {
			return nil, fmt.Errorf("ipc socket path: %w", err)
		}
		socketPath = p
	}
	if err := ipc.ValidateSocketPath(socketPath); err != nil {
		return nil, err
	}
	handler := newIPCHandler(srv, deps.Exec, deps.VaultRoot, deps.IndexDBPath, dryRun)
	ipcSrv := ipc.New(socketPath, handler)
	if err := ipcSrv.Start(ctx); err != nil {
		return nil, fmt.Errorf("ipc start: %w", err)
	}
	fmt.Printf("sea-noise ipc listening on %s\n", socketPath)
	return ipcSrv, nil
}

// buildDaemonDeps resolves vault + optional collaborators. Errors
// only on hard-required state (vault path); soft-missing creds
// (TODOIST_API_TOKEN, --cli-command) leave dependencies nil, and
// individual steps defer or report explicit missing-dependency errors.
func buildDaemonDeps(cliCommand, cliArgsList, skillsDir string, timeoutSec int, indexDB string) (daemonStepDeps, error) {
	vaultRoot, err := config.LoadVaultPath()
	if err != nil {
		return daemonStepDeps{}, err
	}

	resolvedIndexDBPath, err := resolveIndexDBPath(vaultRoot, indexDB)
	if err != nil {
		return daemonStepDeps{}, fmt.Errorf("resolve index db path: %w", err)
	}
	deps := daemonStepDeps{
		VaultRoot:   vaultRoot,
		IndexDBPath: resolvedIndexDBPath,
		Crawler:     crawl.NewCrawlerFromEnv(),
	}

	if cliCommand != "" {
		skills, err := loadSkills(skillsDir)
		if err != nil {
			return daemonStepDeps{}, fmt.Errorf("load skills: %w", err)
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
