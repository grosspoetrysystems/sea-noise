package crawl

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// StubCrawler returns a fixed set of pages on every Crawl call,
// regardless of URL. Loaded once from a directory of *.md files at
// construction time.
//
// Used by parity tests (and the parity-discover seam) to feed the
// enrich-topics workflow deterministic web context without making
// live HTTP calls. Per .tempor/exec/P1-W3-kickoff.md decision 4: this
// stub does NOT try to mirror what live Cloudflare would return — its
// job is to give the LLM call some reasonable input so the post-state
// assembly path runs end-to-end.
type StubCrawler struct {
	pages []Page
}

// NewStubCrawler loads every *.md file under dir as a Page, with the
// file's path (relative to dir) as the URL and its contents as the
// Markdown body. Files are loaded in path-sorted order for
// determinism. Missing dir is not an error — returns an empty crawler.
func NewStubCrawler(dir string) (*StubCrawler, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return &StubCrawler{}, nil
		}
		return nil, fmt.Errorf("crawl: read stub dir %s: %w", dir, err)
	}

	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		paths = append(paths, filepath.Join(dir, e.Name()))
	}
	sort.Strings(paths)

	pages := make([]Page, 0, len(paths))
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("crawl: read stub %s: %w", p, err)
		}
		rel, _ := filepath.Rel(dir, p)
		pages = append(pages, Page{
			URL:      "stub://" + filepath.ToSlash(rel),
			Markdown: string(raw),
		})
	}
	return &StubCrawler{pages: pages}, nil
}

// Crawl returns the loaded pages regardless of url / limit / depth.
// The workflow caps records itself (MaxRecordsInPrompt = 10); the
// stub just hands over what it has and lets the consumer truncate.
func (s *StubCrawler) Crawl(_ context.Context, _ string, _, _ int) ([]Page, error) {
	out := make([]Page, len(s.pages))
	copy(out, s.pages)
	return out, nil
}
