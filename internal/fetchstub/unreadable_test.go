package fetchstub

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"sea-noise/internal/crawl"
)

func TestHydrateStubs_ContinuesAfterICloudReadFailure(t *testing.T) {
	vaultRoot := t.TempDir()
	writeUnreadableTestNote(t, vaultRoot, "inbox/blocked.md", "https://example.com/blocked")
	writeUnreadableTestNote(t, vaultRoot, "inbox/ready.md", "https://example.com/ready")

	previous := readBulkFile
	readBulkFile = func(_ context.Context, path string) ([]byte, error) {
		if filepath.Base(path) == "blocked.md" {
			return nil, syscall.EDEADLK
		}
		return os.ReadFile(path)
	}
	t.Cleanup(func() { readBulkFile = previous })

	result, err := HydrateStubs(context.Background(), unreadableTestCrawler{}, HydrateInput{
		VaultRoot: vaultRoot,
		Now:       time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("HydrateStubs: %v", err)
	}
	if len(result.Unreadable) != 1 || result.Unreadable[0] != "inbox/blocked.md" {
		t.Fatalf("Unreadable = %v, want [inbox/blocked.md]", result.Unreadable)
	}
	if len(result.Hydrated) != 1 || result.Hydrated[0] != "inbox/ready.md" {
		t.Fatalf("Hydrated = %v, want [inbox/ready.md]", result.Hydrated)
	}
}

type unreadableTestCrawler struct{}

func (unreadableTestCrawler) Crawl(_ context.Context, url string, _, _ int) ([]crawl.Page, error) {
	return []crawl.Page{{URL: url, Markdown: "hydrated body for " + url}}, nil
}

func writeUnreadableTestNote(t *testing.T, root, rel, source string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nsource: " + source + "\ntags: [clippings]\n---\nshort body\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
