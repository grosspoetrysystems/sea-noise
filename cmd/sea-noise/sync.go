package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"sea-noise/internal/config"
	"sea-noise/internal/todoist"
	"sea-noise/internal/todoistsync"
)

// runSync implements `sea-noise sync <system> [mode]`. Currently the
// only system is `todoist`. Additional systems (e.g. Things, Anybox)
// would register as new sub-cases here.
func runSync(args []string) error {
	if len(args) == 0 {
		return errors.New("sync: missing system; expected `sea-noise sync todoist [push|--completions]`")
	}
	system := args[0]
	rest := args[1:]
	switch system {
	case "todoist":
		return runSyncTodoist(rest)
	default:
		return fmt.Errorf("sync: unknown system %q (only `todoist` is implemented)", system)
	}
}

// runSyncTodoist dispatches between push (default) and --completions
// modes, mirroring references/scripts/sync-todoist.py +
// references/scripts/sync-todoist-completions.py.
func runSyncTodoist(args []string) error {
	fs := flag.NewFlagSet("sync todoist", flag.ExitOnError)
	completions := fs.Bool("completions", false, "pull completed tasks and mark vault notes done (instead of pushing new tasks)")
	vaultName := fs.String("vault-name", todoistsync.DefaultVaultName, "Obsidian vault name used in the obsidian:// deep-link inside pushed task content")
	lookbackDays := fs.Int("lookback-days", 14, "[--completions only] how many days back to query for completed tasks")
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Allow a positional `push` for parity with the prototype usage
	// pattern but treat it as a no-op (push is the default mode).
	for _, a := range fs.Args() {
		if a != "push" {
			return fmt.Errorf("sync todoist: unexpected argument %q", a)
		}
	}

	vaultRoot, err := config.LoadVaultPath()
	if err != nil {
		return err
	}
	client, err := todoist.NewFromEnv()
	if err != nil {
		return err
	}

	if *completions {
		return runSyncTodoistCompletions(client, vaultRoot, time.Duration(*lookbackDays)*24*time.Hour)
	}
	return runSyncTodoistPush(client, vaultRoot, *vaultName)
}

func runSyncTodoistPush(client *todoist.Client, vaultRoot, vaultName string) error {
	res, err := todoistsync.PushTasks(context.Background(), client, todoistsync.PushTasksInput{
		VaultRoot: vaultRoot,
		VaultName: vaultName,
		Projects: todoistsync.ProjectIDs{
			Inbox:    os.Getenv("TODOIST_INBOX_ID"),
			Work:     os.Getenv("TODOIST_WORK_ID"),
			Personal: os.Getenv("TODOIST_PERSONAL_ID"),
		},
	})
	if err != nil {
		return err
	}
	fmt.Printf("Synced:   %d\n", len(res.Synced))
	for _, p := range res.Synced {
		fmt.Printf("  ✓ %s\n", p)
	}
	fmt.Printf("Skipped:  %d\n", len(res.Skipped))
	fmt.Printf("Failed:   %d\n", len(res.Failed))
	for _, p := range res.Failed {
		fmt.Printf("  ✗ %s\n", p)
	}
	return nil
}

func runSyncTodoistCompletions(client *todoist.Client, vaultRoot string, lookback time.Duration) error {
	res, err := todoistsync.PullCompletions(context.Background(), client, todoistsync.PullCompletionsInput{
		VaultRoot: vaultRoot,
		Lookback:  lookback,
	})
	if err != nil {
		return err
	}
	fmt.Printf("Completed: %d\n", len(res.Completed))
	for _, p := range res.Completed {
		fmt.Printf("  ✓ %s\n", p)
	}
	fmt.Printf("Skipped:   %d\n", len(res.Skipped))
	return nil
}
