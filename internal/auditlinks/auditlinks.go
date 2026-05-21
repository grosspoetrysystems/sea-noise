// Package auditlinks ports references/scripts/audit-links.py.
//
// Walk the vault, extract wikilinks per note, detect targets whose
// lowercased stem isn't present in the vault, and produce a markdown
// report grouped by source note.
//
// Parity contract: BuildReport's output bytes match what the Python
// prototype writes to _meta/broken-links.md for the same vault state
// and date. The golden file lives at testdata/expected/audit-links/.
package auditlinks

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"sea-noise/internal/markdown"
	"sea-noise/internal/vault"
)

// BuildReport returns the broken-links report for the vault at root,
// using today as the report's date. Output matches references/scripts/
// audit-links.py:main() byte-for-byte.
func BuildReport(root string, today time.Time) (string, error) {
	// Collect all stems (lowercased) per the prototype's
	// collect_note_stems. Uses the same exclude set as scan_links.
	notes, err := vault.Walk(root, vault.LinksExcludeDirs)
	if err != nil {
		return "", fmt.Errorf("walk vault: %w", err)
	}

	stems := make(map[string]struct{}, len(notes))
	for _, n := range notes {
		stem := strings.ToLower(strings.TrimSuffix(filepath.Base(n.Path), ".md"))
		stems[stem] = struct{}{}
	}

	// Scan each note for wikilinks; collect (source, broken-target) pairs.
	type pair struct{ source, target string }
	var broken []pair

	for _, n := range notes {
		// vault.Walk already returns each note's post-frontmatter Body,
		// but for link extraction we want the full file content
		// (frontmatter could contain wikilinks too, matching the
		// prototype which reads the whole text). Re-read.
		text, err := readFile(filepath.Join(root, n.Path))
		if err != nil {
			// Match prototype: read_text(errors="replace") tolerates,
			// but a totally unreadable file is skipped silently.
			continue
		}
		for _, target := range markdown.ExtractWikilinks(text) {
			if _, ok := stems[strings.ToLower(target)]; !ok {
				broken = append(broken, pair{source: n.Path, target: target})
			}
		}
	}

	dateStr := today.Format("2006-01-02")
	var lines []string
	lines = append(lines, fmt.Sprintf("# Broken Links Audit — %s\n", dateStr))

	if len(broken) == 0 {
		lines = append(lines, "No broken links found.\n")
	} else {
		lines = append(lines, fmt.Sprintf("Found **%d** broken wikilinks:\n", len(broken)))

		// Group by source, then sort sources alphabetically; targets
		// within each source also sorted (per prototype).
		grouped := make(map[string][]string)
		for _, p := range broken {
			grouped[p.source] = append(grouped[p.source], p.target)
		}
		sources := make([]string, 0, len(grouped))
		for s := range grouped {
			sources = append(sources, s)
		}
		sort.Strings(sources)
		for _, source := range sources {
			targets := grouped[source]
			sort.Strings(targets)
			lines = append(lines, "\n### "+source)
			for _, t := range targets {
				lines = append(lines, "- [["+t+"]]")
			}
		}
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n"), nil
}

// readFile is a thin os.ReadFile shim. Defined here so tests can swap
// it if needed (no current swap; the package is keep-it-simple).
var readFile = realReadFile
