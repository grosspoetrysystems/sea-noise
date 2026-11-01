package fetchstub

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestHydrateStubs_DeadlineReturnsUnreadableRemainder(t *testing.T) {
	vaultRoot := t.TempDir()
	writeUnreadableTestNote(t, vaultRoot, "inbox/first.md", "https://example.com/first")
	writeUnreadableTestNote(t, vaultRoot, "inbox/second.md", "https://example.com/second")

	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancel()
	result, err := HydrateStubs(ctx, unreadableTestCrawler{}, HydrateInput{VaultRoot: vaultRoot})
	if err != nil {
		t.Fatalf("HydrateStubs: %v", err)
	}
	if len(result.Unreadable) != 2 ||
		result.Unreadable[0] != "inbox/first.md" ||
		result.Unreadable[1] != "inbox/second.md" {
		t.Fatalf("Unreadable = %v, want both remaining inbox notes", result.Unreadable)
	}
}

func TestHydrateStubs_ReadDeadlinePreservesPriorHydration(t *testing.T) {
	vaultRoot := t.TempDir()
	writeUnreadableTestNote(t, vaultRoot, "inbox/first.md", "https://example.com/first")
	writeUnreadableTestNote(t, vaultRoot, "inbox/second.md", "https://example.com/second")

	previous := readVaultFile
	readVaultFile = func(_ context.Context, path string) ([]byte, error) {
		if filepath.Base(path) == "second.md" {
			return nil, context.DeadlineExceeded
		}
		return os.ReadFile(path)
	}
	t.Cleanup(func() { readVaultFile = previous })

	result, err := HydrateStubs(context.Background(), unreadableTestCrawler{}, HydrateInput{
		VaultRoot: vaultRoot,
		Now:       time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("HydrateStubs: %v", err)
	}
	if len(result.Hydrated) != 1 || result.Hydrated[0] != "inbox/first.md" {
		t.Fatalf("Hydrated = %v, want [inbox/first.md]", result.Hydrated)
	}
	if len(result.Unreadable) != 1 || result.Unreadable[0] != "inbox/second.md" {
		t.Fatalf("Unreadable = %v, want [inbox/second.md]", result.Unreadable)
	}
}

func TestHydrateStubs_CrawlDeadlineSkipsOnlyCurrentNote(t *testing.T) {
	vaultRoot := t.TempDir()
	writeUnreadableTestNote(t, vaultRoot, "inbox/first.md", "https://example.com/first")
	writeUnreadableTestNote(t, vaultRoot, "inbox/second.md", "https://example.com/second")

	result, err := HydrateStubs(context.Background(), crawlDeadlineCrawler{}, HydrateInput{
		VaultRoot: vaultRoot,
		Now:       time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("HydrateStubs: %v", err)
	}
	if len(result.Unreadable) != 1 || result.Unreadable[0] != "inbox/first.md" {
		t.Fatalf("Unreadable = %v, want [inbox/first.md]", result.Unreadable)
	}
	if len(result.Hydrated) != 1 || result.Hydrated[0] != "inbox/second.md" {
		t.Fatalf("Hydrated = %v, want [inbox/second.md]", result.Hydrated)
	}
}

type crawlDeadlineCrawler struct{}

func (crawlDeadlineCrawler) Crawl(_ context.Context, url string, _, _ int) ([]crawl.Page, error) {
	if strings.HasSuffix(url, "/first") {
		return nil, context.DeadlineExceeded
	}
	return []crawl.Page{{URL: url, Markdown: "hydrated body for " + url}}, nil
}

func TestHydrateStubs_ContextDeadlineDuringCrawlReturnsPartial(t *testing.T) {
	vaultRoot := t.TempDir()
	writeUnreadableTestNote(t, vaultRoot, "inbox/first.md", "https://example.com/first")
	writeUnreadableTestNote(t, vaultRoot, "inbox/second.md", "https://example.com/second")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	result, err := HydrateStubs(ctx, waitingDeadlineCrawler{}, HydrateInput{
		VaultRoot: vaultRoot,
		Now:       time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("HydrateStubs: %v", err)
	}
	if len(result.Unreadable) != 2 ||
		result.Unreadable[0] != "inbox/first.md" ||
		result.Unreadable[1] != "inbox/second.md" {
		t.Fatalf("Unreadable = %v, want both inbox notes", result.Unreadable)
	}
}

type waitingDeadlineCrawler struct{}

func (waitingDeadlineCrawler) Crawl(ctx context.Context, _ string, _, _ int) ([]crawl.Page, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestReadBulkFileUsesCallerContext(t *testing.T) {
	previous := readVaultFile
	t.Cleanup(func() { readVaultFile = previous })

	observed := make(chan time.Duration, 1)
	readVaultFile = func(ctx context.Context, _ string) ([]byte, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Error("read context has no deadline")
			return nil, nil
		}
		observed <- time.Until(deadline)
		return []byte("ok"), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if _, err := readBulkFile(ctx, "placeholder.md"); err != nil {
		t.Fatalf("readBulkFile: %v", err)
	}
	if remaining := <-observed; remaining < 10*time.Second {
		t.Fatalf("readBulkFile shortened caller deadline to %s", remaining)
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
