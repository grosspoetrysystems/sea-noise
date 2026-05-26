package main

import (
	"context"
	"flag"
	"fmt"

	"sea-noise/internal/config"
	"sea-noise/internal/crawl"
	"sea-noise/internal/fetchstub"
)

// runFetch implements `sea-noise fetch` and `sea-noise fetch --retry`.
// Default mode hydrates inbox stubs with crawled markdown; --retry
// reads _meta/crawl-failures.json and retries entries whose backoff
// window has elapsed. Mirrors references/scripts/fetch-stub-content.py.
func runFetch(args []string) error {
	fs := flag.NewFlagSet("fetch", flag.ExitOnError)
	retry := fs.Bool("retry", false, "retry failed crawls from _meta/crawl-failures.json instead of scanning the inbox")
	if err := fs.Parse(args); err != nil {
		return err
	}

	vaultRoot, err := config.LoadVaultPath()
	if err != nil {
		return err
	}
	crawler := crawl.NewCrawlerFromEnv()

	if *retry {
		res, err := fetchstub.RetryFailures(context.Background(), crawler, fetchstub.RetryInput{VaultRoot: vaultRoot})
		if err != nil {
			return err
		}
		fmt.Printf("Retried:    %d\n", res.Retried)
		fmt.Printf("Succeeded:  %d\n", len(res.Succeeded))
		for _, p := range res.Succeeded {
			fmt.Printf("  ✓ %s\n", p)
		}
		fmt.Printf("Still failed: %d\n", res.StillFailed)
		fmt.Printf("Skipped:    %d (abandoned or not yet due)\n", res.Skipped)
		return nil
	}

	res, err := fetchstub.ProcessInbox(context.Background(), crawler, fetchstub.InboxInput{VaultRoot: vaultRoot})
	if err != nil {
		return err
	}
	fmt.Printf("Hydrated:  %d\n", len(res.Hydrated))
	for _, p := range res.Hydrated {
		fmt.Printf("  ✓ %s\n", p)
	}
	fmt.Printf("Deduped:   %d\n", len(res.Deduped))
	for _, p := range res.Deduped {
		fmt.Printf("  → %s (moved to _meta/discarded/)\n", p)
	}
	fmt.Printf("Failed:    %d (logged to _meta/crawl-failures.json)\n", len(res.Failed))
	fmt.Printf("Skipped:   %d (non-stub or no source)\n", len(res.Skipped))
	return nil
}
