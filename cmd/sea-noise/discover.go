package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sea-noise/internal/ai/inferencerouter"
	"sea-noise/internal/ai/skill"
	"sea-noise/internal/ai/transport"
	"sea-noise/internal/config"
	"sea-noise/internal/discovery"
	"sea-noise/internal/vernacular"
)

const maxDiscoverTimeoutSeconds = int64((1<<63 - 1) / int64(time.Second))

func runDiscover(args []string) error {
	fs := flag.NewFlagSet("discover", flag.ContinueOnError)
	dbPath := fs.String("db", "", "SQLite retrieval index (defaults to ~/.sea-noise/retrieval.db)")
	projectKey := fs.String("project", "", "linked project key to supply context; search still covers the whole vault")
	limit := fs.Int("limit", 5, "maximum connection candidates (1–10)")
	expand := fs.Bool("expand", false, "ask the model for bounded query expansions before retrieval")
	skillsDir := fs.String("skills-dir", "", "directory containing inference skill files")
	cliCommand := fs.String("cli-command", "", "explicit prompt-in/response-out inference executable")
	cliArgs := fs.String("cli-args", "", "explicit comma-separated argv template; {PROMPT} is substituted")
	readOnlyCLI := fs.Bool("read-only-cli", false, "attest that the selected executable and arguments disable tools and source writes; not an OS sandbox")
	timeout := fs.Int("timeout", 300, "per-inference-call timeout in seconds")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage: sea-noise discover [flags] QUERY...

Explain connections using real indexed passages; does not write source notes.
Search stays deterministic and requires no inference CLI.

Required:
  --cli-command COMMAND  trusted prompt-in/response-out executable
  --cli-args TEMPLATE    explicit comma-separated argv; {PROMPT} is substituted
  --read-only-cli        acknowledge that this invocation disables tools/writes

Optional:
  --project KEY          supply a linked project's overview, without filtering out personal notes
  --db PATH              previously built SQLite retrieval index
  --limit N              maximum candidates, 1–10 (default 5)
  --expand               request bounded AI query expansion
  --skills-dir PATH      override embedded inference skills
  --timeout SECONDS      per-call timeout (default 300)

Sea-noise only reads the index. The executable is trusted configuration: this
acknowledgment does not sandbox it or disable its tools. Verify its arguments
before use. Results are candidates, not persisted graph edges.`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	query := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if query == "" {
		return fmt.Errorf("query is required")
	}
	argsProvided := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "cli-args" {
			argsProvided = true
		}
	})
	if *cliCommand == "" || !argsProvided || !*readOnlyCLI {
		return fmt.Errorf("discovery requires --cli-command, explicit --cli-args and --read-only-cli; verify a prompt-only invocation with tools and writes disabled")
	}
	if *timeout <= 0 || int64(*timeout) > maxDiscoverTimeoutSeconds {
		return fmt.Errorf("--timeout must be between 1 and %d seconds", maxDiscoverTimeoutSeconds)
	}
	if *limit < 1 || *limit > 10 {
		return fmt.Errorf("--limit must be between 1 and 10")
	}

	resolvedDB := *dbPath
	var vaultRoot string
	if resolvedDB == "" || *projectKey != "" {
		var err error
		vaultRoot, err = config.LoadVaultPath()
		if err != nil {
			return err
		}
	}
	if resolvedDB == "" {
		var err error
		resolvedDB, err = resolveIndexDBPath(vaultRoot, "")
		if err != nil {
			return err
		}
	}
	var overviewID string
	if *projectKey != "" {
		projects, err := vernacular.List(vaultRoot)
		if err != nil {
			return err
		}
		for _, project := range projects {
			if project.Key == *projectKey {
				overviewID = project.Overview
				break
			}
		}
		if overviewID == "" {
			return fmt.Errorf("project %q is not linked; use sea-noise vernacular list", *projectKey)
		}
	}

	skills, err := loadSkills(*skillsDir)
	if err != nil {
		return fmt.Errorf("load skills: %w", err)
	}
	tx := &transport.CLITransport{
		Command:        *cliCommand,
		Args:           splitCSV(*cliArgs),
		Timeout:        time.Duration(*timeout) * time.Second,
		MaxStdoutBytes: discovery.MaxResponseBytes,
		MaxStderrBytes: 200,
	}
	executor := skill.NewExecutor(skills, inferencerouter.NewLocalRouter(), tx)
	absoluteDB, err := filepath.Abs(resolvedDB)
	if err != nil {
		return fmt.Errorf("resolve index path: %w", err)
	}
	indexURI := url.URL{Scheme: "file", Path: absoluteDB, RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", indexURI.String())
	if err != nil {
		return fmt.Errorf("open index: %w", err)
	}
	defer db.Close()
	result, err := discovery.Discover(context.Background(), discovery.Config{
		DB: db, Executor: executor, ReadOnlyCLI: *readOnlyCLI,
	}, discovery.Request{
		Query: query, ProjectOverviewID: overviewID, Limit: *limit, ExpandQueries: *expand,
	})
	if err != nil {
		return err
	}
	out := json.NewEncoder(os.Stdout)
	out.SetIndent("", "  ")
	return out.Encode(result)
}
