package vault

import "sea-noise/internal/markdown"

// extractWikilinksForTest is a tiny shim so the parity test can call
// the markdown package's extractor without exporting test-only helpers
// from a non-test file.
func extractWikilinksForTest(content string) []string {
	return markdown.ExtractWikilinks(content)
}
