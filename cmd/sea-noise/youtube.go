package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"sea-noise/internal/config"
	"sea-noise/internal/youtube"
	"sea-noise/internal/youtubefetch"
)

// runYoutube implements `sea-noise youtube`. Walks the vault for notes
// whose `source:` frontmatter points at YouTube and that don't yet
// have a `## Transcript` section, fetches the transcript, and appends
// the section. Mirrors references/scripts/fetch-youtube-transcript.py.
func runYoutube(args []string) error {
	fs := flag.NewFlagSet("youtube", flag.ExitOnError)
	langCSV := fs.String("lang", "en", "comma-separated language preference list for transcripts")
	if err := fs.Parse(args); err != nil {
		return err
	}

	vaultRoot, err := config.LoadVaultPath()
	if err != nil {
		return err
	}

	client := youtube.New()
	res, err := youtubefetch.FetchTranscripts(context.Background(), client, youtubefetch.Input{
		VaultRoot: vaultRoot,
		Languages: splitCSVList(*langCSV),
	})
	if err != nil {
		return err
	}

	fmt.Printf("Updated:  %d\n", len(res.Updated))
	for _, p := range res.Updated {
		fmt.Printf("  ✓ %s\n", p)
	}
	fmt.Printf("Skipped:  %d (already have transcript or non-YouTube)\n", len(res.Skipped))
	fmt.Printf("Failed:   %d\n", len(res.Failed))
	for _, p := range res.Failed {
		fmt.Printf("  ✗ %s\n", p)
	}
	return nil
}

func splitCSVList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
