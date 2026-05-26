package todoist_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/go-retryablehttp"

	"sea-noise/internal/todoist"
)

func newTestClient(t *testing.T, server *httptest.Server) *todoist.Client {
	t.Helper()
	r := retryablehttp.NewClient()
	r.Logger = nil
	r.RetryMax = 3
	r.RetryWaitMin = time.Millisecond
	r.RetryWaitMax = 2 * time.Millisecond
	c := todoist.NewWithRetry("test-token", r)
	c.BaseURL = server.URL
	return c
}

func TestCreateTask_Success(t *testing.T) {
	var gotAuth, gotCT, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tasks" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"7000"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := c.CreateTask(context.Background(), todoist.CreateTaskRequest{
		Content:   "Read paper [→ vault]",
		Labels:    []string{"mind-palace"},
		ProjectID: "proj-123",
		DueString: "tomorrow 9am",
		DueLang:   "en",
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if got.ID != "7000" {
		t.Errorf("ID = %q, want %q", got.ID, "7000")
	}
	if got.URL != "https://todoist.com/app/task/7000" {
		t.Errorf("URL = %q, want https://todoist.com/app/task/7000", got.URL)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q", gotCT)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(gotBody), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["content"] != "Read paper [→ vault]" {
		t.Errorf("content = %v", payload["content"])
	}
	if payload["project_id"] != "proj-123" {
		t.Errorf("project_id = %v", payload["project_id"])
	}
	if payload["due_string"] != "tomorrow 9am" {
		t.Errorf("due_string = %v", payload["due_string"])
	}
	if payload["due_lang"] != "en" {
		t.Errorf("due_lang = %v", payload["due_lang"])
	}
}

func TestCreateTask_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"bad token"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)

	_, err := c.CreateTask(context.Background(), todoist.CreateTaskRequest{Content: "x"})
	if !errors.Is(err, todoist.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestCreateTask_RetriesOn429(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"42"}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)

	got, err := c.CreateTask(context.Background(), todoist.CreateTaskRequest{Content: "x"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if got.ID != "42" {
		t.Errorf("ID = %q", got.ID)
	}
	if h := atomic.LoadInt32(&hits); h != 2 {
		t.Errorf("hits = %d, want 2 (one 429 + one success)", h)
	}
}

func TestCreateTask_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad input", http.StatusBadRequest)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)

	_, err := c.CreateTask(context.Background(), todoist.CreateTaskRequest{Content: "x"})
	var apiErr *todoist.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d", apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.Body, "bad input") {
		t.Errorf("Body = %q", apiErr.Body)
	}
}

func TestCompletedRange_Pagination(t *testing.T) {
	pages := []string{
		`{"items":[{"id":"1","content":"a","labels":["mind-palace"],"completed_at":"2026-05-20T10:00:00Z"},{"id":"2","content":"b","labels":[],"completed_at":"2026-05-20T11:00:00Z"}],"next_cursor":"cur-2"}`,
		`{"items":[{"id":"3","content":"c","labels":["mind-palace"],"completed_at":"2026-05-21T08:00:00Z"}],"next_cursor":""}`,
	}
	var cursors []string
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tasks/completed/by_completion_date" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		cursors = append(cursors, r.URL.Query().Get("cursor"))
		idx := atomic.AddInt32(&hits, 1) - 1
		w.Header().Set("Content-Type", "application/json")
		if int(idx) < len(pages) {
			_, _ = w.Write([]byte(pages[idx]))
		} else {
			_, _ = w.Write([]byte(`{"items":[],"next_cursor":""}`))
		}
	}))
	defer srv.Close()
	c := newTestClient(t, srv)

	since := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC)
	got, err := c.CompletedRange(context.Background(), since, until, 200)
	if err != nil {
		t.Fatalf("CompletedRange: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(got))
	}
	if got[0].ID != "1" || got[2].ID != "3" {
		t.Errorf("ids = %q,%q,%q", got[0].ID, got[1].ID, got[2].ID)
	}
	if cursors[0] != "" {
		t.Errorf("first cursor = %q, want empty", cursors[0])
	}
	if cursors[1] != "cur-2" {
		t.Errorf("second cursor = %q, want cur-2", cursors[1])
	}
}

func TestCompletedRange_DateParams(t *testing.T) {
	var gotSince, gotUntil, gotLimit string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSince = r.URL.Query().Get("since")
		gotUntil = r.URL.Query().Get("until")
		gotLimit = r.URL.Query().Get("limit")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"next_cursor":""}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)

	since := time.Date(2026, 5, 10, 14, 30, 0, 0, time.UTC)
	until := time.Date(2026, 5, 24, 7, 15, 0, 0, time.UTC)
	if _, err := c.CompletedRange(context.Background(), since, until, 0); err != nil {
		t.Fatalf("CompletedRange: %v", err)
	}
	if gotSince != "2026-05-10T00:00:00Z" {
		t.Errorf("since = %q", gotSince)
	}
	if gotUntil != "2026-05-24T23:59:59Z" {
		t.Errorf("until = %q", gotUntil)
	}
	if gotLimit != "200" {
		t.Errorf("limit = %q, want default 200", gotLimit)
	}
}

func TestNewFromEnv(t *testing.T) {
	t.Setenv("TODOIST_API_TOKEN", "")
	if _, err := todoist.NewFromEnv(); !errors.Is(err, todoist.ErrNotConfigured) {
		t.Errorf("err = %v, want ErrNotConfigured", err)
	}

	t.Setenv("TODOIST_API_TOKEN", "abc")
	c, err := todoist.NewFromEnv()
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if c == nil {
		t.Fatal("client is nil")
	}
}
