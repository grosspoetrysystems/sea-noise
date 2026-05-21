// Package markdown parses note bodies: frontmatter, wikilinks, tags.
//
// Behavior mirrors the Python prototype's parsing logic verbatim, sourced
// from references/scripts/_rag/indexer.py (parse_frontmatter) and
// references/scripts/audit-links.py (WIKILINK regex). Parity is judged
// behaviorally against testdata/expected/ golden files captured from the
// prototype on the fixture vault.
package markdown

import (
	"regexp"
	"strings"
)

var firstHeadingRE = regexp.MustCompile(`(?m)^#\s+(.+)`)

// Frontmatter holds the parsed result of a note's leading YAML block.
type Frontmatter struct {
	// Title is the value of the `title:` key in the frontmatter, stripped
	// of surrounding double or single quotes. If frontmatter has no title,
	// or no frontmatter is present, Title falls back to the first `# heading`
	// in the body. Title is empty if both sources are absent — callers
	// (e.g., vault walker) typically fall back to a filename-stem default
	// in that case.
	Title string

	// Tags is the value of the `tags:` key in the frontmatter, parsed as
	// a bracketed comma-separated list (`tags: [a, b, c]`) and re-emitted
	// as a single space-separated string ("a b c"). Empty when absent.
	Tags string

	// Body is the note content with the frontmatter block removed and
	// leading whitespace stripped. If no frontmatter is present, Body
	// equals the full input content.
	Body string
}

// ParseFrontmatter extracts title, tags, and body from a note's contents.
// Matches the Python prototype's parse_frontmatter byte-for-byte:
//
//   - Frontmatter is delimited by a leading "---" and a closing "\n---".
//   - Inside, lines beginning with "title:" or "tags:" are scanned for values.
//   - The title value has surrounding " and ' stripped.
//   - The tags value has surrounding [ ] stripped, then split on commas
//     and re-joined with single spaces, dropping empty entries.
//   - If no title is found in frontmatter, the first "# heading" in the
//     body is used. If that's also absent, Title is left empty for the
//     caller to fill in (e.g., from the filename stem).
func ParseFrontmatter(content string) Frontmatter {
	fm := Frontmatter{Body: content}

	if strings.HasPrefix(content, "---") {
		// Python: content.find("\n---", 3) — searches starting at index 3.
		// Go strings.Index doesn't accept a start offset; offset manually.
		rest := content[3:]
		idx := strings.Index(rest, "\n---")
		if idx != -1 {
			fmBlock := rest[:idx]
			// Body starts after "\n---" plus one more char to skip the
			// trailing newline after the close (Python's content[end + 4:]
			// where end is the position of "\n---" in the original; +4
			// skips "\n---" itself). Then lstrip.
			body := rest[idx+4:]
			fm.Body = strings.TrimLeft(body, " \t\n\v\f\r")

			for _, line := range strings.Split(fmBlock, "\n") {
				switch {
				case strings.HasPrefix(line, "title:"):
					v := strings.TrimSpace(line[6:])
					v = strings.Trim(v, `"`)
					v = strings.Trim(v, `'`)
					fm.Title = v
				case strings.HasPrefix(line, "tags:"):
					raw := strings.TrimSpace(line[5:])
					raw = strings.Trim(raw, "[]")
					parts := strings.Split(raw, ",")
					out := make([]string, 0, len(parts))
					for _, p := range parts {
						p = strings.TrimSpace(p)
						if p != "" {
							out = append(out, p)
						}
					}
					fm.Tags = strings.Join(out, " ")
				}
			}
		}
	}

	if fm.Title == "" {
		if m := firstHeadingRE.FindStringSubmatch(fm.Body); m != nil {
			fm.Title = strings.TrimSpace(m[1])
		}
	}

	return fm
}

// TitleFromFilenameStem produces the title fallback the Python prototype's
// indexer uses when parse_frontmatter returns an empty title: replace
// dashes and underscores in the filename stem with spaces.
func TitleFromFilenameStem(stem string) string {
	stem = strings.ReplaceAll(stem, "-", " ")
	stem = strings.ReplaceAll(stem, "_", " ")
	return stem
}
