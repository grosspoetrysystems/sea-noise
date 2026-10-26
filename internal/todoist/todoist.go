// Package todoist is the Todoist API client used by Sea Noise's
// task-sync workflows.
//
// The package exposes a single Client that wraps two slices of the
// Todoist API:
//
//   - the transactional task surface (POST /api/v1/tasks etc.) used to
//     push notes from notes/todos/ into Todoist;
//   - the completion-stream surface (GET /api/v1/tasks/completed/
//     by_completion_date) used to mark vault notes done when their
//     mirrored Todoist tasks are completed.
//
// The Python prototype's sync-todoist.py and sync-todoist-completions.py
// scripts hit the same endpoints. v8 referred to these surfaces as
// "REST v2" and "Sync v9"; Todoist has since unified them under
// api.todoist.com/api/v1. The Sea Noise port targets the v1 URLs
// directly per AGENTS.md parity discipline (features over endpoints).
//
// HTTP transport is backed by github.com/hashicorp/go-retryablehttp
// for exponential backoff on 429 and 5xx responses, mirroring the
// retry posture the prototype gets implicitly through Todoist's own
// rate-limit-tolerant behaviour.
package todoist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// DefaultBaseURL is the Todoist API root. Override via Client.BaseURL
// in tests to redirect to an httptest.Server.
const DefaultBaseURL = "https://api.todoist.com"

// ErrUnauthorized is returned when the API rejects the configured token.
var ErrUnauthorized = errors.New("todoist: unauthorized (check TODOIST_API_TOKEN)")

// ErrNotConfigured is returned by NewFromEnv when TODOIST_API_TOKEN is
// unset. Callers can treat this as "skip Todoist sync" rather than fail.
var ErrNotConfigured = errors.New("todoist: TODOIST_API_TOKEN not set")

// Client talks to the Todoist v1 API.
type Client struct {
	token   string
	http    *http.Client
	BaseURL string
}

// New returns a Client backed by a retryablehttp transport with
// exponential backoff (default 4 retries, 1s–30s window). Override
// retry tuning by mutating the embedded http.Client transport in tests.
func New(token string) *Client {
	r := retryablehttp.NewClient()
	r.Logger = nil
	return newWithRetryable(token, r)
}

// NewWithRetry returns a Client backed by a caller-provided
// retryablehttp configuration. Tests use this seam to compress retry
// timing; production code should call New.
func NewWithRetry(token string, r *retryablehttp.Client) *Client {
	if r == nil {
		r = retryablehttp.NewClient()
		r.Logger = nil
	}
	return newWithRetryable(token, r)
}

// NewFromEnv reads TODOIST_API_TOKEN. Returns (nil, ErrNotConfigured)
// when the env var is absent so callers can skip the workflow silently
// (matching the prototype scripts' env-required posture, but expressed
// as a sentinel error instead of an exception).
func NewFromEnv() (*Client, error) {
	tok := os.Getenv("TODOIST_API_TOKEN")
	if tok == "" {
		return nil, ErrNotConfigured
	}
	return New(tok), nil
}

func newWithRetryable(token string, r *retryablehttp.Client) *Client {
	return &Client{
		token:   token,
		http:    r.StandardClient(),
		BaseURL: DefaultBaseURL,
	}
}

// CreateTaskRequest is the payload accepted by CreateTask. Fields
// mirror the Todoist task-create JSON contract; empty values are
// omitted so the endpoint applies its own defaults.
type CreateTaskRequest struct {
	Content   string   `json:"content"`
	Labels    []string `json:"labels,omitempty"`
	ProjectID string   `json:"project_id,omitempty"`
	DueString string   `json:"due_string,omitempty"`
	DueLang   string   `json:"due_lang,omitempty"`
}

// Task is the subset of the created-task response the prototype reads
// (id + the canonical web URL derived from it).
type Task struct {
	ID  string `json:"id"`
	URL string `json:"-"`
}

// CreateTask POSTs req to /api/v1/tasks and returns the created task
// with its public URL populated. Authentication failures surface as
// ErrUnauthorized; other non-2xx responses surface as *APIError.
func (c *Client) CreateTask(ctx context.Context, req CreateTaskRequest) (Task, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return Task{}, fmt.Errorf("todoist: encode CreateTask payload: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/v1/tasks", bytes.NewReader(body))
	if err != nil {
		return Task{}, fmt.Errorf("todoist: build CreateTask request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return Task{}, fmt.Errorf("todoist: CreateTask: %w", err)
	}
	defer resp.Body.Close()

	if err := classify(resp); err != nil {
		return Task{}, err
	}

	var out Task
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Task{}, fmt.Errorf("todoist: decode CreateTask response: %w", err)
	}
	if out.ID != "" {
		out.URL = "https://todoist.com/app/task/" + out.ID
	}
	return out, nil
}

// CompletedTask is one entry from the completed-by-completion-date
// stream. Labels are preserved so callers can filter to a known tag
// (the prototype filters on "mind-palace").
type CompletedTask struct {
	ID          string   `json:"id"`
	Content     string   `json:"content"`
	Labels      []string `json:"labels"`
	CompletedAt string   `json:"completed_at"`
}

type completedPage struct {
	Items      []CompletedTask `json:"items"`
	NextCursor string          `json:"next_cursor"`
}

// CompletedRange fetches every task completed between since and until
// (inclusive), paginating via the cursor field the API returns. The
// limit parameter is the page size; the prototype uses 200.
func (c *Client) CompletedRange(ctx context.Context, since, until time.Time, limit int) ([]CompletedTask, error) {
	if limit <= 0 {
		limit = 200
	}
	var (
		out    []CompletedTask
		cursor string
	)
	for {
		q := url.Values{}
		q.Set("since", since.UTC().Format("2006-01-02")+"T00:00:00Z")
		q.Set("until", until.UTC().Format("2006-01-02")+"T23:59:59Z")
		q.Set("limit", strconv.Itoa(limit))
		if cursor != "" {
			q.Set("cursor", cursor)
		}

		u := c.BaseURL + "/api/v1/tasks/completed/by_completion_date?" + q.Encode()
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, fmt.Errorf("todoist: build CompletedRange request: %w", err)
		}
		httpReq.Header.Set("Authorization", "Bearer "+c.token)

		resp, err := c.http.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("todoist: CompletedRange: %w", err)
		}
		if err := classify(resp); err != nil {
			resp.Body.Close()
			return nil, err
		}
		var page completedPage
		if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("todoist: decode CompletedRange page: %w", err)
		}
		resp.Body.Close()

		out = append(out, page.Items...)
		if page.NextCursor == "" || len(page.Items) == 0 {
			break
		}
		cursor = page.NextCursor
	}
	return out, nil
}

// APIError carries the HTTP status and response body from a non-2xx
// Todoist response.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("todoist: api error %d: %s", e.StatusCode, e.Body)
}

func classify(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	return &APIError{StatusCode: resp.StatusCode, Body: string(body)}
}
