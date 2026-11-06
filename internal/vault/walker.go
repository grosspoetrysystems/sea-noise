// Package vault holds the vault abstraction: vault path resolution,
// markdown file enumeration, and the note record shape that the rest of
// the runtime consumes.
//
// Behavior mirrors the Python prototype's walk_vault rules (sourced from
// references/scripts/_rag/indexer.py and references/scripts/audit-links.py).
// Different surfaces use different directory exclude sets — the indexer
// excludes signal/ and bloom/ from retrieval scope, audit-links does not.
// Walker.Walk takes the exclude set as a parameter so each caller can
// pick the right surface.
package vault

import (
	"context"
	"errors"
)

// Sentinel errors. Wrap with %w when returning; callers check via errors.Is.
var (
	// ErrVaultNotFound is returned when the configured vault path does
	// not exist or is not a directory at walk time. (Note: config layer
	// has its own sentinels in internal/config; this one fires if the
	// vault is removed between config load and walk.)
	ErrVaultNotFound = errors.New("vault path not found")
)

// IndexerExcludeDirs matches references/scripts/_rag/indexer.py:EXCLUDE_DIRS.
// Used by retrieval indexing surfaces.
var IndexerExcludeDirs = []string{"_rag", "_meta", "templates", ".git", ".obsidian", "signal", "bloom"}

// LinksExcludeDirs matches references/scripts/audit-links.py:EXCLUDE_DIRS.
// Used by link-extraction surfaces. Note that signal/ and bloom/ are
// NOT excluded here — different surface, different rules.
var LinksExcludeDirs = []string{"_rag", ".git", ".obsidian", ".claude", "templates", "_scripts"}

// Note is the canonical record the walker produces.
type Note struct {
	// ID is the logical path of the note relative to the vault root.
	// Registered external sources use their explicit logical source root;
	// they are never replaced with a physical path.
	ID string

	// Path is the logical path used by retrieval and graph callers.
	Path string

	// PhysicalPath is the resolved original file read by the walker.
	PhysicalPath string

	// Title is the resolved title: frontmatter title -> first `# heading`
	// in body -> filename stem with dashes/underscores replaced with spaces.
	Title string

	// Tags is the space-separated tags string from the frontmatter
	// (empty when absent), per the prototype's parse_frontmatter shape.
	Tags string

	// Body is the post-frontmatter content (or the full content if no
	// frontmatter is present), left-stripped of whitespace.
	Body string
}

// Walk visits all *.md files under root, skipping any path that contains
// a segment in excludeDirs. It does not follow arbitrary symlinks.
func Walk(root string, excludeDirs []string) ([]Note, error) {
	return WalkContext(context.Background(), root, excludeDirs)
}

// WalkContext is Walk with cancellation during directory walking and reads.
// It retains the vault-root logical namespace; bridge-owned external roots
// are added through WalkSourcesContext.
func WalkContext(ctx context.Context, root string, excludeDirs []string) ([]Note, error) {
	return WalkSourcesContext(ctx, root, excludeDirs, nil)
}
