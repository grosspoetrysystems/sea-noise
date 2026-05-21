package markdown

import (
	"regexp"
	"strings"
)

// wikilinkRE matches Obsidian-style wikilinks: [[Target]], [[Target|alias]],
// [[Target#section]]. The first capture group is the target with whitespace
// preserved (callers strip it). Mirrors the regex in
// references/scripts/audit-links.py.
var wikilinkRE = regexp.MustCompile(`\[\[([^\]|#]+)(?:[#|][^\]]+)?\]\]`)

// ExtractWikilinks returns the link targets from a note's content in the
// order they appear. Each target is trimmed of surrounding whitespace, per
// the prototype's audit-links behavior.
//
// Targets are returned as-is, not lowercased — case sensitivity is the
// caller's choice (e.g., broken-link detection lowercases for the
// stem-set comparison).
func ExtractWikilinks(content string) []string {
	matches := wikilinkRE.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return []string{}
	}
	out := make([]string, len(matches))
	for i, m := range matches {
		out[i] = strings.TrimSpace(m[1])
	}
	return out
}
