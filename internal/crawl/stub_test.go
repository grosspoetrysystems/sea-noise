package crawl

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStubCrawler_LoadsAllMarkdownFiles(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "alpha.md"), "ALPHA BODY")
	mustWrite(t, filepath.Join(dir, "bravo.md"), "BRAVO BODY")
	mustWrite(t, filepath.Join(dir, "ignored.txt"), "NOT MARKDOWN")

	sc, err := NewStubCrawler(dir)
	if err != nil {
		t.Fatalf("NewStubCrawler: %v", err)
	}

	pages, err := sc.Crawl(context.Background(), "http://anything", 100, 5)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("len(pages) = %d, want 2", len(pages))
	}
	if pages[0].URL != "stub://alpha.md" || pages[0].Markdown != "ALPHA BODY" {
		t.Errorf("pages[0] = %+v, want {stub://alpha.md, ALPHA BODY}", pages[0])
	}
	if pages[1].URL != "stub://bravo.md" || pages[1].Markdown != "BRAVO BODY" {
		t.Errorf("pages[1] = %+v, want {stub://bravo.md, BRAVO BODY}", pages[1])
	}
}

func TestStubCrawler_MissingDirIsNotError(t *testing.T) {
	sc, err := NewStubCrawler(filepath.Join(t.TempDir(), "absent"))
	if err != nil {
		t.Fatalf("NewStubCrawler with missing dir: %v", err)
	}
	pages, err := sc.Crawl(context.Background(), "x", 1, 1)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}
	if len(pages) != 0 {
		t.Errorf("len(pages) = %d, want 0 (empty stub crawler)", len(pages))
	}
}

func TestStubCrawler_IgnoresURL(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "only.md"), "ONLY BODY")

	sc, err := NewStubCrawler(dir)
	if err != nil {
		t.Fatalf("NewStubCrawler: %v", err)
	}
	a, _ := sc.Crawl(context.Background(), "http://a.example", 1, 1)
	b, _ := sc.Crawl(context.Background(), "http://b.example", 1, 1)
	if len(a) != 1 || len(b) != 1 || a[0].Markdown != b[0].Markdown {
		t.Errorf("stub crawler should ignore URL: a=%+v b=%+v", a, b)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
