package retrieval

import (
	"regexp"
	"strings"
)

// blankLineRE splits on one or more lines that are empty or whitespace-only.
// Used to chunk a note body into paragraphs.
var blankLineRE = regexp.MustCompile(`(?m)(?:\r?\n[ \t]*){2,}`)

// ChunkBody splits a note body into ordered chunks. The current strategy
// is paragraph-based: split on blank lines, drop whitespace-only chunks,
// preserve order.
//
// Chunking is a Go-side design departure from the Python prototype (the
// prototype stored one row per note for vector retrieval). The schema is
// shaped for sub-note granularity per ARCHITECTURE.md §3; the strategy
// here is a starting point. If a note has no blank-line separation it is
// stored as a single chunk.
func ChunkBody(body string) []string {
	if strings.TrimSpace(body) == "" {
		return nil
	}
	parts := blankLineRE.Split(body, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
