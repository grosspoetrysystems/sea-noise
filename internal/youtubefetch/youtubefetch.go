// Package youtubefetch ports references/scripts/fetch-youtube-transcript.py.
//
// FetchTranscripts walks the configured vault directories, finds
// markdown notes whose `source:` frontmatter points at a YouTube URL
// and that do not yet have a `## Transcript` section, fetches the
// transcript via internal/youtube, and appends the section to the note.
//
// Frontmatter bytes are preserved verbatim — only the body region is
// mutated (append-only), so notes round-trip byte-for-byte except for
// the appended transcript block.
package youtubefetch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sea-noise/internal/markdown"
	"sea-noise/internal/vault"
	"sea-noise/internal/youtube"
)

// DefaultSearchDirs mirrors the prototype's hard-coded scan set.
var DefaultSearchDirs = []string{
	"notes/fleeting",
	"notes/references",
	"inbox",
}

// DefaultLanguages is the language preference list used when none is
// supplied. The prototype passes ["en"] to YouTubeTranscriptApi.fetch.
var DefaultLanguages = []string{"en"}

// Input configures a FetchTranscripts run.
type Input struct {
	VaultRoot  string
	SearchDirs []string
	Languages  []string
}

// Result reports per-note outcomes. All paths are vault-relative.
type Result struct {
	Updated []string
	Skipped []string
	Failed  []string
}

// FetchTranscripts walks the configured search dirs, hydrating
// transcripts in matching notes. Per-note fetch and read failures are
// recorded in Failed so one temporarily unavailable cloud note does not
// abort the rest of the scan; directory errors still abort the run.
func FetchTranscripts(ctx context.Context, client *youtube.Client, in Input) (Result, error) {
	var result Result
	dirs := in.SearchDirs
	if len(dirs) == 0 {
		dirs = DefaultSearchDirs
	}
	langs := in.Languages
	if len(langs) == 0 {
		langs = DefaultLanguages
	}

	for _, rel := range dirs {
		dir := filepath.Join(in.VaultRoot, rel)
		entries, err := os.ReadDir(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return result, fmt.Errorf("youtubefetch: read %s: %w", dir, err)
		}

		paths := make([]string, 0, len(entries))
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
		sort.Strings(paths)

		for _, p := range paths {
			rel, _ := filepath.Rel(in.VaultRoot, p)
			switch fetchOne(ctx, client, p, langs) {
			case outcomeUpdated:
				result.Updated = append(result.Updated, rel)
			case outcomeSkipped:
				result.Skipped = append(result.Skipped, rel)
			case outcomeFailed:
				result.Failed = append(result.Failed, rel)
			}
		}
	}
	return result, nil
}

type outcomeKind int

const (
	outcomeSkipped outcomeKind = iota
	outcomeUpdated
	outcomeFailed
)

func fetchOne(ctx context.Context, client *youtube.Client, path string, langs []string) outcomeKind {
	raw, err := vault.ReadFileRetryContext(ctx, path)
	if err != nil {
		return outcomeFailed
	}
	content := string(raw)

	if strings.Contains(content, "## Transcript") {
		return outcomeSkipped
	}

	fm, err := markdown.ParseFrontmatterYAML(content)
	if err != nil {
		return outcomeSkipped
	}
	source, _ := fm.Fields["source"].(string)
	if source == "" || (!strings.Contains(source, "youtube.com") && !strings.Contains(source, "youtu.be")) {
		return outcomeSkipped
	}
	videoID, ok := youtube.ExtractVideoID(source)
	if !ok {
		return outcomeFailed
	}

	transcript, err := client.FetchTranscript(ctx, videoID, langs)
	if err != nil {
		return outcomeFailed
	}

	// Mirror references/scripts/fetch-youtube-transcript.py:
	//   post.content = content.rstrip() + f"\n\n## Transcript\n\n{transcript}\n"
	// We operate on raw bytes (not the frontmatter-stripped body) so the
	// frontmatter block survives untouched.
	newContent := strings.TrimRight(content, " \t\n\v\f\r") + "\n\n## Transcript\n\n" + transcript + "\n"
	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return outcomeFailed
	}
	return outcomeUpdated
}
