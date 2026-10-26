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
// transcripts in matching notes. Per-note fetch failures (no video
// ID, no caption track, network error) are recorded in Failed and do
// not abort the run; IO errors during walk surface immediately.
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
			outcome, err := fetchOne(ctx, client, p, langs)
			if err != nil {
				return result, err
			}
			switch outcome {
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

func fetchOne(ctx context.Context, client *youtube.Client, path string, langs []string) (outcomeKind, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("youtubefetch: read %s: %w", path, err)
	}
	content := string(raw)

	if strings.Contains(content, "## Transcript") {
		return outcomeSkipped, nil
	}

	fm, err := markdown.ParseFrontmatterYAML(content)
	if err != nil {
		return outcomeSkipped, nil
	}
	source, _ := fm.Fields["source"].(string)
	if source == "" || (!strings.Contains(source, "youtube.com") && !strings.Contains(source, "youtu.be")) {
		return outcomeSkipped, nil
	}
	videoID, ok := youtube.ExtractVideoID(source)
	if !ok {
		return outcomeFailed, nil
	}

	transcript, err := client.FetchTranscript(ctx, videoID, langs)
	if err != nil {
		return outcomeFailed, nil
	}

	// Mirror references/scripts/fetch-youtube-transcript.py:
	//   post.content = content.rstrip() + f"\n\n## Transcript\n\n{transcript}\n"
	// We operate on raw bytes (not the frontmatter-stripped body) so the
	// frontmatter block survives untouched.
	newContent := strings.TrimRight(content, " \t\n\v\f\r") + "\n\n## Transcript\n\n" + transcript + "\n"
	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return 0, fmt.Errorf("youtubefetch: write %s: %w", path, err)
	}
	return outcomeUpdated, nil
}
