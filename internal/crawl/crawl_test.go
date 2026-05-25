package crawl_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"sea-noise/internal/crawl"
)

func TestCFCrawler_HappyPath(t *testing.T) {
	const jobID = "job-abc-123"

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var got map[string]any
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode start body: %v", err)
			}
			if got["url"] != "https://example.com" {
				t.Errorf("start body url = %v, want example.com", got["url"])
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{"id": jobID},
			})
		case http.MethodGet:
			if !strings.HasSuffix(r.URL.Path, "/"+jobID) {
				t.Errorf("poll path = %s, want suffix /%s", r.URL.Path, jobID)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"status": "completed",
					"records": []map[string]any{
						{"url": "https://example.com/a", "status": "completed", "markdown": "# A\nbody"},
						{"url": "https://example.com/b", "status": "completed", "markdown": "# B"},
						{"url": "https://example.com/c", "status": "failed", "markdown": ""},
					},
				},
			})
		default:
			http.Error(w, "method", http.StatusMethodNotAllowed)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := &crawl.CFCrawler{
		AccountID:    "acct",
		APIToken:     "tok",
		HTTPClient:   srv.Client(),
		BaseURL:      srv.URL,
		PollInterval: 1 * time.Millisecond,
		PollMax:      5,
	}

	pages, err := c.Crawl(context.Background(), "https://example.com", 5, 1)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("len(pages) = %d, want 2 (failed record filtered)", len(pages))
	}
	if pages[0].URL != "https://example.com/a" || pages[0].Markdown != "# A\nbody" {
		t.Errorf("pages[0] = %+v", pages[0])
	}
	if pages[1].URL != "https://example.com/b" {
		t.Errorf("pages[1] = %+v", pages[1])
	}
}

func TestCFCrawler_Errored(t *testing.T) {
	const jobID = "job-x"
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"id": jobID}})
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{"status": "errored"},
			})
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := &crawl.CFCrawler{HTTPClient: srv.Client(), BaseURL: srv.URL, PollInterval: 1 * time.Millisecond, PollMax: 5}
	_, err := c.Crawl(context.Background(), "https://example.com", 1, 1)
	if !errors.Is(err, crawl.ErrCrawl) {
		t.Fatalf("want ErrCrawl, got %v", err)
	}
}

func TestNoopCrawler(t *testing.T) {
	pages, err := crawl.NoopCrawler{}.Crawl(context.Background(), "x", 1, 1)
	if err != nil || pages != nil {
		t.Errorf("NoopCrawler: pages=%v err=%v", pages, err)
	}
}

func TestNewCrawlerFromEnv(t *testing.T) {
	saveID := os.Getenv("CF_ACCOUNT_ID")
	saveTok := os.Getenv("CF_API_TOKEN")
	defer func() {
		os.Setenv("CF_ACCOUNT_ID", saveID)
		os.Setenv("CF_API_TOKEN", saveTok)
	}()

	os.Unsetenv("CF_ACCOUNT_ID")
	os.Unsetenv("CF_API_TOKEN")
	if _, ok := crawl.NewCrawlerFromEnv().(crawl.NoopCrawler); !ok {
		t.Errorf("env empty: want NoopCrawler")
	}

	os.Setenv("CF_ACCOUNT_ID", "acct")
	os.Setenv("CF_API_TOKEN", "tok")
	if _, ok := crawl.NewCrawlerFromEnv().(*crawl.CFCrawler); !ok {
		t.Errorf("env set: want *CFCrawler")
	}
}
