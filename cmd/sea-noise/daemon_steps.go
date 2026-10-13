package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"sea-noise/internal/ai/skill"
	"sea-noise/internal/auditlinks"
	"sea-noise/internal/crawl"
	"sea-noise/internal/daemon"
	"sea-noise/internal/enrichtopics"
	"sea-noise/internal/entityprofile"
	"sea-noise/internal/fetchstub"
	"sea-noise/internal/inboxprocess"
	"sea-noise/internal/retrieval"
	"sea-noise/internal/signal"
	"sea-noise/internal/summarizer"
	"sea-noise/internal/todoist"
	"sea-noise/internal/todoistsync"
	"sea-noise/internal/youtube"
	"sea-noise/internal/youtubefetch"
)

// daemonStepDeps holds the long-lived collaborators each step binding
// captures. A nil dep means the step it backs is unavailable; the
// matching daemon.Step then has a nil Fn and is logged as deferred.
type daemonStepDeps struct {
	VaultRoot     string
	IndexDBPath   string // default: ~/.sea-noise/retrieval.db
	Exec          *skill.Executor
	Crawler       crawl.Crawler
	TodoistClient *todoist.Client
}

// buildRegistry wires the daily/weekly/inbox chains. Steps whose
// underlying workflow needs a missing dep (e.g. summarize requires
// Exec which is nil when --cli-command is unset) are still registered
// with their canonical name but their Fn is nil — the daemon logs
// them as deferred and continues. This keeps the step list shape
// identical to the prototype's catchup scripts even when the operator
// has not configured every collaborator yet.
func buildRegistry(deps daemonStepDeps) daemon.Registry {
	v := deps.VaultRoot
	if deps.IndexDBPath == "" {
		if p, err := defaultIndexDBPath(); err == nil {
			deps.IndexDBPath = p
		} else {
			deps.IndexDBPath = filepath.Join(v, "_meta", "retrieval.db")
		}
	}
	hydrationReady := true
	hydrationSkipPaths := ""

	retryCrawls := func(ctx context.Context) error {
		_, err := fetchstub.RetryFailures(ctx, deps.Crawler, fetchstub.RetryInput{VaultRoot: v})
		return err
	}
	rotateLogsStep := func(ctx context.Context) error {
		_, err := daemon.RotateProcessingLogs(v, time.Now(), 0)
		return err
	}
	hydrateStubsStep := func(ctx context.Context) error {
		hydrationReady = false
		hydrationSkipPaths = ""
		// Do not triage unhydrated web clips; a missing body can look like
		// low-signal content and get discarded.
		hydrateCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		result, err := fetchstub.HydrateStubs(hydrateCtx, deps.Crawler, fetchstub.HydrateInput{VaultRoot: v})
		hydrationSkipPaths = strings.Join(append(result.Failed, result.Unreadable...), "\n")
		if err != nil {
			return err
		}
		hydrationReady = true
		return nil
	}

	indexStep := func(ctx context.Context) error {
		if err := os.MkdirAll(filepath.Dir(deps.IndexDBPath), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(deps.IndexDBPath), err)
		}
		db, err := sql.Open("sqlite", deps.IndexDBPath)
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer db.Close()
		if err := retrieval.Migrate(ctx, db); err != nil {
			return err
		}
		_, err = retrieval.IndexVault(ctx, db, v, nil)
		return err
	}
	signalStep := func(ctx context.Context) error {
		_, _, err := signal.BuildAndWrite(v, time.Now())
		return err
	}
	auditStep := func(ctx context.Context) error {
		report, err := auditlinks.BuildReport(v, time.Now())
		if err != nil {
			return err
		}
		dest := filepath.Join(v, "_meta", "broken-links.md")
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dest, []byte(report), 0o644)
	}
	summarizeStep := func(ctx context.Context) error {
		if deps.Exec == nil {
			return errors.New("daemon: summarize requires --cli-command")
		}
		_, err := summarizer.EnrichTranscripts(ctx, deps.Exec, summarizer.EnrichTranscriptsInput{VaultRoot: v})
		return err
	}
	enrichAutoStep := func(ctx context.Context) error {
		if deps.Exec == nil {
			return errors.New("daemon: enrich-topics requires --cli-command")
		}
		_, err := enrichtopics.EnrichTopicsAuto(ctx, deps.Exec, deps.Crawler, enrichtopics.AutoInput{VaultRoot: v})
		return err
	}
	profileAutoStep := func(ctx context.Context) error {
		if deps.Exec == nil {
			return errors.New("daemon: profile-entities requires --cli-command")
		}
		_, err := entityprofile.GenerateAuto(ctx, deps.Exec, entityprofile.AutoInput{VaultRoot: v})
		return err
	}
	todoistPushStep := func(ctx context.Context) error {
		if deps.TodoistClient == nil {
			return errors.New("daemon: todoist-push requires TODOIST_API_TOKEN")
		}
		_, err := todoistsync.PushTasks(ctx, deps.TodoistClient, todoistsync.PushTasksInput{
			VaultRoot: v,
			Projects: todoistsync.ProjectIDs{
				Inbox:    os.Getenv("TODOIST_INBOX_ID"),
				Work:     os.Getenv("TODOIST_WORK_ID"),
				Personal: os.Getenv("TODOIST_PERSONAL_ID"),
			},
		})
		return err
	}
	todoistPullStep := func(ctx context.Context) error {
		if deps.TodoistClient == nil {
			return errors.New("daemon: todoist-pull requires TODOIST_API_TOKEN")
		}
		_, err := todoistsync.PullCompletions(ctx, deps.TodoistClient, todoistsync.PullCompletionsInput{VaultRoot: v})
		return err
	}
	processInboxStep := func(ctx context.Context) error {
		if !hydrationReady {
			return errors.New("daemon: process-inbox skipped until hydrate-stubs succeeds")
		}
		if deps.Exec == nil {
			return errors.New("daemon: process-inbox requires --cli-command")
		}
		_, err := inboxprocess.Run(ctx, deps.Exec, inboxprocess.Input{VaultRoot: v, SkipPaths: hydrationSkipPaths})
		return err
	}
	youtubeTranscriptStep := func(ctx context.Context) error {
		client := youtube.New()
		_, err := youtubefetch.FetchTranscripts(ctx, client, youtubefetch.Input{VaultRoot: v})
		return err
	}

	return daemon.Registry{
		Daily: []daemon.Step{
			{Name: "rotate-logs", Fn: rotateLogsStep},
			{Name: "retry-crawls", Fn: retryCrawls},
			{Name: "summarize", Fn: summarizeStep},
			{Name: "index", Fn: indexStep},
			{Name: "daily-synthesis", Fn: nil},
			{Name: "sync-consumed", Fn: nil},
			{Name: "signal", Fn: signalStep},
			{Name: "plunge", Fn: nil},
			{Name: "home", Fn: nil},
			{Name: "todoist-push", Fn: todoistPushStep},
			{Name: "todoist-pull", Fn: todoistPullStep},
		},
		Weekly: []daemon.Step{
			{Name: "retry-crawls", Fn: retryCrawls},
			{Name: "weekly-synthesis", Fn: nil},
			{Name: "index", Fn: indexStep},
			{Name: "signal", Fn: signalStep},
			{Name: "audit-links", Fn: auditStep},
			{Name: "enrich-topics", Fn: enrichAutoStep},
			{Name: "profile-entities", Fn: profileAutoStep},
		},
		Inbox: []daemon.Step{
			{Name: "hydrate-stubs", Fn: hydrateStubsStep},
			{Name: "process-inbox", Fn: processInboxStep},
			{Name: "youtube-transcript", Fn: youtubeTranscriptStep},
		},
	}
}
