package enrichtopics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Page is one crawled page returned by a Crawler.
type Page struct {
	URL      string
	Markdown string
}

// Crawler fetches web pages around a seed URL. Implementations live
// behind this interface so tests can use a fake without speaking HTTP.
type Crawler interface {
	// Crawl returns up to limit pages reachable within depth hops from
	// url, in markdown form. Implementations may return nil pages and
	// no error to signal "not configured / nothing to crawl" — callers
	// must handle that gracefully (proceed without web context).
	Crawl(ctx context.Context, url string, limit, depth int) ([]Page, error)
}

// NoopCrawler returns no pages. Used when Cloudflare credentials are
// absent so enrichment proceeds with vault context only.
type NoopCrawler struct{}

// Crawl always returns (nil, nil).
func (NoopCrawler) Crawl(context.Context, string, int, int) ([]Page, error) {
	return nil, nil
}

// NewCrawlerFromEnv inspects CF_ACCOUNT_ID and CF_API_TOKEN and returns
// a CFCrawler when both are set, otherwise a NoopCrawler. Never errors —
// the prototype silently falls back when credentials are missing.
func NewCrawlerFromEnv() Crawler {
	id := os.Getenv("CF_ACCOUNT_ID")
	token := os.Getenv("CF_API_TOKEN")
	if id == "" || token == "" {
		return NoopCrawler{}
	}
	return NewCFCrawler(id, token)
}

// CFCrawler talks to the Cloudflare browser-rendering /crawl endpoint.
// Mirrors references/scripts/enrich-topics.py:crawl_url — start a job,
// poll until completion (or timeout / cancelled / errored), return the
// completed records.
type CFCrawler struct {
	AccountID string
	APIToken  string

	// HTTPClient is exported so tests can inject a recording client or
	// point at httptest.Server via BaseURL.
	HTTPClient *http.Client

	// BaseURL overrides the Cloudflare endpoint. Tests set this to an
	// httptest.Server URL. Production callers leave it zero.
	BaseURL string

	// PollInterval is how long to wait between status polls. Mirrors
	// the prototype's CRAWL_POLL_INTERVAL (10s). Tests reduce this.
	PollInterval time.Duration

	// PollMax is the max number of polls before giving up. Mirrors the
	// prototype's CRAWL_POLL_MAX (60).
	PollMax int
}

// NewCFCrawler returns a CFCrawler with production defaults.
func NewCFCrawler(accountID, apiToken string) *CFCrawler {
	return &CFCrawler{
		AccountID:    accountID,
		APIToken:     apiToken,
		HTTPClient:   http.DefaultClient,
		PollInterval: 10 * time.Second,
		PollMax:      60,
	}
}

// ErrCrawl is the sentinel wrapper for crawler failures. Wrap with %w;
// check via errors.Is.
var ErrCrawl = errors.New("enrichtopics: crawl failed")

// Crawl starts a crawl job and polls until it completes. Returns the
// completed records.
func (c *CFCrawler) Crawl(ctx context.Context, url string, limit, depth int) ([]Page, error) {
	base := c.BaseURL
	if base == "" {
		base = fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/browser-rendering/crawl", c.AccountID)
	}

	jobID, err := c.startJob(ctx, base, url, limit, depth)
	if err != nil {
		return nil, err
	}

	for i := 0; i < c.PollMax; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(c.PollInterval):
		}

		status, records, done, err := c.pollJob(ctx, base, jobID)
		if err != nil {
			// Transient poll errors are tolerated; keep trying until
			// PollMax. Matches Python's continue-on-non-ok behavior.
			continue
		}
		if done {
			if status == "errored" {
				return nil, fmt.Errorf("%w: job %s errored", ErrCrawl, jobID)
			}
			return records, nil
		}
	}

	return nil, fmt.Errorf("%w: poll exhausted after %d attempts", ErrCrawl, c.PollMax)
}

// startJob posts the crawl request and returns the job ID.
func (c *CFCrawler) startJob(ctx context.Context, base, url string, limit, depth int) (string, error) {
	body, err := json.Marshal(map[string]any{
		"url":     url,
		"limit":   limit,
		"depth":   depth,
		"formats": []string{"markdown"},
		"render":  false,
	})
	if err != nil {
		return "", fmt.Errorf("%w: marshal: %v", ErrCrawl, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("%w: build request: %v", ErrCrawl, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: post: %v", ErrCrawl, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("%w: start status %d: %s", ErrCrawl, resp.StatusCode, string(b))
	}

	var parsed struct {
		Result any `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("%w: decode start response: %v", ErrCrawl, err)
	}
	switch r := parsed.Result.(type) {
	case map[string]any:
		if id, ok := r["id"].(string); ok && id != "" {
			return id, nil
		}
	case string:
		if r != "" {
			return r, nil
		}
	}
	return "", fmt.Errorf("%w: start response missing job id", ErrCrawl)
}

// pollJob fetches a job status. Returns (status, completedRecords, done, err).
// done is true when the job has reached a terminal state.
func (c *CFCrawler) pollJob(ctx context.Context, base, jobID string) (string, []Page, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/"+jobID, nil)
	if err != nil {
		return "", nil, false, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", nil, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", nil, false, fmt.Errorf("status %d", resp.StatusCode)
	}

	var parsed struct {
		Result struct {
			Status   string `json:"status"`
			Finished int    `json:"finished"`
			Total    int    `json:"total"`
			Records  []struct {
				URL      string `json:"url"`
				Status   string `json:"status"`
				Markdown string `json:"markdown"`
			} `json:"records"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", nil, false, err
	}

	status := parsed.Result.Status
	terminal := false
	switch status {
	case "completed", "cancelled_due_to_timeout", "cancelled_due_to_limits", "errored":
		terminal = true
	}
	if !terminal {
		return status, nil, false, nil
	}
	if status == "errored" {
		return status, nil, true, nil
	}

	out := make([]Page, 0, len(parsed.Result.Records))
	for _, r := range parsed.Result.Records {
		if r.Status != "completed" {
			continue
		}
		out = append(out, Page{URL: r.URL, Markdown: r.Markdown})
	}
	return status, out, true, nil
}
