package enrichtopics_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sea-noise/internal/ai/inferencerouter"
	"sea-noise/internal/ai/skill"
	"sea-noise/internal/ai/transport"
	"sea-noise/internal/enrichtopics"
)

// writeVault materialises a synthetic vault and returns the root.
func writeVault(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", abs, err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", abs, err)
		}
	}
	return root
}

func TestEnrichTopic_HappyPath_NoCrawl(t *testing.T) {
	root := writeVault(t, map[string]string{
		// tag substring: "ai" inside "tech/ai-agents".
		"notes/references/agents-overview.md": "---\ntitle: Agents Overview\ntags: [reference, tech/ai-agents]\nsource: https://example.com/agents\n---\nIntro to agents.\nMore body.\n",
		// slug substring: "ai" inside "openai-symphony".
		"notes/references/openai-symphony.md": "---\ntitle: OpenAI Symphony\ntags: [reference]\nsource: https://example.com/symphony\n---\nSymphony notes.\n",
		// not a match.
		"notes/references/cooking.md": "---\ntitle: Recipe\ntags: [reference, life/cooking]\n---\nFood\n",
		// excluded dir.
		"_meta/notes.md": "---\ntitle: Meta\ntags: [tech/ai]\n---\nshould be skipped\n",
	})

	mock := &transport.MockTransport{}
	mock.EnqueueResponses("### Follow-up Questions\n- Q1\n\n### Related Materials\n- M1\n")
	exec := skill.NewExecutor(loadRealSkills(t), inferencerouter.NewLocalRouter(), mock)

	res, err := enrichtopics.EnrichTopic(context.Background(), exec, enrichtopics.NoopCrawler{}, enrichtopics.Input{
		VaultRoot: root,
		Topic:     "ai",
		Today:     time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}
	if res.Notes != 2 {
		t.Errorf("Notes = %d, want 2 (excluded _meta and cooking)", res.Notes)
	}
	if res.Crawled != 0 {
		t.Errorf("Crawled = %d, want 0 (NoopCrawler)", res.Crawled)
	}
	if res.BriefPath != "notes/references/briefs/2026-05-23-ai-research-brief.md" {
		t.Errorf("BriefPath = %q", res.BriefPath)
	}
	briefAbs := filepath.Join(root, res.BriefPath)
	got, err := os.ReadFile(briefAbs)
	if err != nil {
		t.Fatalf("read brief: %v", err)
	}
	body := string(got)
	for _, want := range []string{
		"title: \"Research Brief: ai\"",
		"## Vault Notes in This Cluster",
		"[[agents-overview]]",
		"[[openai-symphony]]",
		"### Follow-up Questions",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("brief missing %q. Body:\n%s", want, body)
		}
	}

	// Log line.
	logBytes, err := os.ReadFile(filepath.Join(root, res.LogPath))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(logBytes), "- ai — 2026-05-23 — notes/references/briefs/2026-05-23-ai-research-brief.md") {
		t.Errorf("log missing expected line:\n%s", string(logBytes))
	}
}

func TestEnrichTopic_NoMatches_SentinelError(t *testing.T) {
	root := writeVault(t, map[string]string{
		"notes/references/cooking.md": "---\ntitle: Recipe\ntags: [reference, life/cooking]\n---\nFood\n",
	})
	mock := &transport.MockTransport{}
	exec := skill.NewExecutor(loadRealSkills(t), inferencerouter.NewLocalRouter(), mock)

	_, err := enrichtopics.EnrichTopic(context.Background(), exec, enrichtopics.NoopCrawler{}, enrichtopics.Input{
		VaultRoot: root,
		Topic:     "ai-agents",
	})
	if !errors.Is(err, enrichtopics.ErrTopicNotFound) {
		t.Errorf("err = %v, want ErrTopicNotFound", err)
	}
	if len(mock.Calls) != 0 {
		t.Errorf("LLM should not be called when no notes match; got %d calls", len(mock.Calls))
	}
}

func TestEnrichTopic_PromptShape(t *testing.T) {
	root := writeVault(t, map[string]string{
		"notes/references/x.md": "---\ntitle: X\ntags: [tech/ai]\nsource: https://example.com\n---\nBody of X.\n",
	})
	mock := &transport.MockTransport{}
	mock.EnqueueResponses("RESP")
	exec := skill.NewExecutor(loadRealSkills(t), inferencerouter.NewLocalRouter(), mock)

	_, err := enrichtopics.EnrichTopic(context.Background(), exec, enrichtopics.NoopCrawler{}, enrichtopics.Input{
		VaultRoot: root,
		Topic:     "ai",
		Today:     time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}
	if len(mock.Calls) != 1 {
		t.Fatalf("expected 1 LLM call, got %d", len(mock.Calls))
	}
	prompt := mock.Calls[0].Prompt
	for _, want := range []string{
		"topic 'ai'",
		"critical mass with 1 notes",
		// Excerpt includes the trailing newline (mirrors prototype's
		// `post.content[:500]`); the format string ends with "..." so
		// the literal substring is split across two lines.
		"- X (notes/references/x.md): Body of X.\n...",
		"### Follow-up Questions",
		"### Related Materials",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q. Prompt:\n%s", want, prompt)
		}
	}
}

func TestEnrichTopic_WithCrawler(t *testing.T) {
	root := writeVault(t, map[string]string{
		"notes/references/x.md": "---\ntitle: X\ntags: [tech/ai]\nsource: https://example.com/x\n---\nBody.\n",
	})
	mock := &transport.MockTransport{}
	mock.EnqueueResponses("RESP")
	exec := skill.NewExecutor(loadRealSkills(t), inferencerouter.NewLocalRouter(), mock)

	crawler := &fakeCrawler{
		pages: []enrichtopics.Page{
			{URL: "https://example.com/x/a", Markdown: "Page A body"},
			{URL: "https://example.com/x/b", Markdown: "Page B body"},
		},
	}

	res, err := enrichtopics.EnrichTopic(context.Background(), exec, crawler, enrichtopics.Input{
		VaultRoot: root,
		Topic:     "ai",
		Today:     time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("EnrichTopic: %v", err)
	}
	if res.Crawled != 2 {
		t.Errorf("Crawled = %d, want 2", res.Crawled)
	}
	prompt := mock.Calls[0].Prompt
	for _, want := range []string{
		"## Web Research",
		"### https://example.com/x/a",
		"Page A body",
		"### https://example.com/x/b",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestEnrichTopic_CrawlerErrorTolerated(t *testing.T) {
	root := writeVault(t, map[string]string{
		"notes/references/x.md": "---\ntitle: X\ntags: [tech/ai]\nsource: https://example.com/x\n---\nBody.\n",
	})
	mock := &transport.MockTransport{}
	mock.EnqueueResponses("RESP")
	exec := skill.NewExecutor(loadRealSkills(t), inferencerouter.NewLocalRouter(), mock)

	crawler := &fakeCrawler{err: errors.New("upstream is down")}

	res, err := enrichtopics.EnrichTopic(context.Background(), exec, crawler, enrichtopics.Input{
		VaultRoot: root,
		Topic:     "ai",
	})
	if err != nil {
		t.Fatalf("crawl error should not propagate: %v", err)
	}
	if res.Crawled != 0 {
		t.Errorf("Crawled = %d, want 0 after crawler error", res.Crawled)
	}
	if !strings.Contains(mock.Calls[0].Prompt, "## Existing Vault Notes") {
		t.Errorf("prompt missing vault notes section")
	}
	if strings.Contains(mock.Calls[0].Prompt, "## Web Research") {
		t.Errorf("web research section should be absent when no records crawled")
	}
}

func TestNewCrawlerFromEnv(t *testing.T) {
	t.Run("noop when env unset", func(t *testing.T) {
		t.Setenv("CF_ACCOUNT_ID", "")
		t.Setenv("CF_API_TOKEN", "")
		c := enrichtopics.NewCrawlerFromEnv()
		if _, ok := c.(enrichtopics.NoopCrawler); !ok {
			t.Errorf("expected NoopCrawler, got %T", c)
		}
	})
	t.Run("cf when both set", func(t *testing.T) {
		t.Setenv("CF_ACCOUNT_ID", "acct")
		t.Setenv("CF_API_TOKEN", "tok")
		c := enrichtopics.NewCrawlerFromEnv()
		if _, ok := c.(*enrichtopics.CFCrawler); !ok {
			t.Errorf("expected *CFCrawler, got %T", c)
		}
	})
	t.Run("noop when only one set", func(t *testing.T) {
		t.Setenv("CF_ACCOUNT_ID", "acct")
		t.Setenv("CF_API_TOKEN", "")
		c := enrichtopics.NewCrawlerFromEnv()
		if _, ok := c.(enrichtopics.NoopCrawler); !ok {
			t.Errorf("expected NoopCrawler, got %T", c)
		}
	})
}

func TestCFCrawler_StartAndPoll(t *testing.T) {
	// Two-phase: POST returns job-id; GET returns completed with records.
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result": {"id": "job-1"}}`))
			return
		}
		// GET /<jobid>
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": {"status": "completed", "finished": 2, "total": 2, "records": [
			{"url": "https://x/a", "status": "completed", "markdown": "A"},
			{"url": "https://x/b", "status": "skipped", "markdown": "B"},
			{"url": "https://x/c", "status": "completed", "markdown": "C"}
		]}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := &enrichtopics.CFCrawler{
		AccountID:    "acct",
		APIToken:     "tok",
		HTTPClient:   server.Client(),
		BaseURL:      server.URL,
		PollInterval: 5 * time.Millisecond,
		PollMax:      5,
	}
	pages, err := c.Crawl(context.Background(), "https://seed.example.com", 5, 1)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("pages = %d, want 2 (skipped record filtered out)", len(pages))
	}
	if pages[0].URL != "https://x/a" || pages[1].URL != "https://x/c" {
		t.Errorf("unexpected pages: %+v", pages)
	}
}

func TestCFCrawler_Errored(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"result": {"id": "job-x"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result": {"status": "errored"}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := &enrichtopics.CFCrawler{
		AccountID: "acct", APIToken: "tok",
		HTTPClient: server.Client(), BaseURL: server.URL,
		PollInterval: 1 * time.Millisecond, PollMax: 3,
	}
	_, err := c.Crawl(context.Background(), "https://seed", 5, 1)
	if !errors.Is(err, enrichtopics.ErrCrawl) {
		t.Errorf("err = %v, want ErrCrawl", err)
	}
}

// === helpers ===

type fakeCrawler struct {
	pages []enrichtopics.Page
	err   error
}

func (f *fakeCrawler) Crawl(context.Context, string, int, int) ([]enrichtopics.Page, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.pages, nil
}

func loadRealSkills(t *testing.T) map[string]*skill.Skill {
	t.Helper()
	root := findRepoRoot(t)
	skills, err := skill.LoadFromDir(filepath.Join(root, ".agents/shared/skills/inference"))
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	return skills
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for d := wd; d != "/" && d != "."; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
	}
	t.Fatal("could not locate repo root from " + wd)
	return ""
}
