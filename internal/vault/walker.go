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
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"sea-noise/internal/markdown"
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
	// ID is the path of the note relative to the vault root, using OS
	// separators in the source path but normalized to forward slashes
	// here (matching the Python prototype's str(path.relative_to(VAULT))
	// on Unix-like systems).
	ID string

	// Path is the same as ID for now. Kept separate so future surfaces
	// can distinguish ID (stable identifier) from Path (filesystem
	// location) without a schema change.
	Path string

	// Title is the resolved title: frontmatter title -> first `# heading`
	// in body -> filename stem with dashes/underscores replaced by spaces.
	Title string

	// Tags is the space-separated tags string from the frontmatter
	// (empty when absent), per the prototype's parse_frontmatter shape.
	Tags string

	// Body is the post-frontmatter content (or the full content if no
	// frontmatter is present), left-stripped of whitespace.
	Body string
}

// Walk visits all *.md files under root, skipping any path that contains
// a segment in excludeDirs. Returns notes in path-sorted order to match
// the prototype's sorted(rglob("*.md")) behavior.
//
// Skips empty (whitespace-only) notes, matching the prototype's
// "if not content_raw.strip(): continue" guard in indexer.py:main().
func Walk(root string, excludeDirs []string) ([]Note, error) {
	rootInfo, err := osStat(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrVaultNotFound
		}
		return nil, err
	}
	if !rootInfo.IsDir() {
		return nil, ErrVaultNotFound
	}

	excludeSet := make(map[string]struct{}, len(excludeDirs))
	for _, d := range excludeDirs {
		excludeSet[d] = struct{}{}
	}

	var paths []string
	err = osWalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip excluded dirs entirely (don't descend).
			if _, ok := excludeSet[d.Name()]; ok && path != root {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		// Also exclude files whose ancestor matches the exclude set.
		// SkipDir above handles most of this, but the rel path check
		// catches absolute-path entries that bypass dir descent (rare).
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
			if _, ok := excludeSet[part]; ok {
				return nil
			}
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)

	notes := make([]Note, 0, len(paths))
	for _, p := range paths {
		content, err := osReadFile(p)
		if err != nil {
			// Match the prototype's lenient behavior: skip unreadable
			// files. The capture script does the same.
			continue
		}
		if strings.TrimSpace(string(content)) == "" {
			continue
		}

		fm := markdown.ParseFrontmatter(string(content))
		title := fm.Title
		if title == "" {
			rel, _ := filepath.Rel(root, p)
			stem := strings.TrimSuffix(filepath.Base(rel), ".md")
			title = markdown.TitleFromFilenameStem(stem)
		}

		rel, _ := filepath.Rel(root, p)
		relSlash := filepath.ToSlash(rel)
		notes = append(notes, Note{
			ID:    relSlash,
			Path:  relSlash,
			Title: title,
			Tags:  fm.Tags,
			Body:  fm.Body,
		})
	}

	return notes, nil
}
