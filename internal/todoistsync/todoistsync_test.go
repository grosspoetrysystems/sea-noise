package todoistsync_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-retryablehttp"

	"sea-noise/internal/todoist"
	"sea-noise/internal/todoistsync"
)

func TestPushTasks_HappyPath(t *testing.T) {
	vault := t.TempDir()
	writeNote(t, vault, "notes/todos/read-paper.md", `---
title: Read the Reranker Paper
tags: [research]
due: tomorrow 4pm
---

Some context about the paper.
`)

	srv := newTodoistServer(t)
	defer srv.Close()
	client := newTodoistClient(t, srv)

	res, err := todoistsync.PushTasks(context.Background(), client, todoistsync.PushTasksInput{
		VaultRoot: vault,
		VaultName: "test-vault",
		Projects:  todoistsync.ProjectIDs{Inbox: "inbox-1"},
	})
	if err != nil {
		t.Fatalf("PushTasks: %v", err)
	}
	if len(res.Synced) != 1 || res.Synced[0] != "notes/todos/read-paper.md" {
		t.Fatalf("Synced = %v", res.Synced)
	}

	// Frontmatter mutation captured the returned id + url.
	got := readNote(t, vault, "notes/todos/read-paper.md")
	if !strings.Contains(got, "synced-todoist: '"+srv.LastID()+"'") {
		t.Errorf("missing synced-todoist key:\n%s", got)
	}
	if !strings.Contains(got, "synced-todoist-url: 'https://todoist.com/app/task/"+srv.LastID()+"'") {
		t.Errorf("missing synced-todoist-url key:\n%s", got)
	}
	// Pre-existing keys + body survive.
	if !strings.HasPrefix(got, "---\ntitle: Read the Reranker Paper\ntags: [research]") {
		t.Errorf("frontmatter prefix mutated:\n%s", got[:200])
	}
	if !strings.HasSuffix(got, "Some context about the paper.\n") {
		t.Errorf("body mutated:\n%s", got)
	}

	// Server saw the right payload.
	req := srv.LastRequest()
	if req.Content != "Read the Reranker Paper [→ vault](obsidian://open?vault=test-vault&file=notes/todos/read-paper)" {
		t.Errorf("content = %q", req.Content)
	}
	if !sliceEqual(req.Labels, []string{"mind-palace"}) {
		t.Errorf("labels = %v", req.Labels)
	}
	if req.ProjectID != "inbox-1" {
		t.Errorf("project_id = %q", req.ProjectID)
	}
	if req.DueString != "tomorrow 4pm" {
		t.Errorf("due_string = %q", req.DueString)
	}
	if req.DueLang != "en" {
		t.Errorf("due_lang = %q", req.DueLang)
	}
}

func TestPushTasks_SkipsAlreadySynced(t *testing.T) {
	vault := t.TempDir()
	original := `---
title: Done Already
synced-todoist: '1234'
---

body
`
	writeNote(t, vault, "notes/todos/done.md", original)

	srv := newTodoistServer(t)
	defer srv.Close()
	client := newTodoistClient(t, srv)

	res, err := todoistsync.PushTasks(context.Background(), client, todoistsync.PushTasksInput{VaultRoot: vault})
	if err != nil {
		t.Fatalf("PushTasks: %v", err)
	}
	if len(res.Skipped) != 1 || len(res.Synced) != 0 {
		t.Errorf("Synced=%v Skipped=%v", res.Synced, res.Skipped)
	}
	if got := readNote(t, vault, "notes/todos/done.md"); got != original {
		t.Errorf("note mutated when it should not have been")
	}
	if srv.CallCount() != 0 {
		t.Errorf("server hit %d times when it should not have been", srv.CallCount())
	}
}

func TestPushTasks_TitleFallbackFromStem(t *testing.T) {
	vault := t.TempDir()
	writeNote(t, vault, "notes/todos/2026-05-26-call-the-vet.md", `---
tags: ['#personal']
---
body
`)

	srv := newTodoistServer(t)
	defer srv.Close()
	client := newTodoistClient(t, srv)

	_, err := todoistsync.PushTasks(context.Background(), client, todoistsync.PushTasksInput{
		VaultRoot: vault,
		VaultName: "v",
		Projects:  todoistsync.ProjectIDs{Personal: "personal-1"},
	})
	if err != nil {
		t.Fatalf("PushTasks: %v", err)
	}
	req := srv.LastRequest()
	if !strings.HasPrefix(req.Content, "Call The Vet [→ vault]") {
		t.Errorf("content = %q (expected stem-derived title)", req.Content)
	}
	if req.ProjectID != "personal-1" {
		t.Errorf("project_id = %q (expected personal route)", req.ProjectID)
	}
}

func TestPushTasks_DueStringNormalization(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"4p", "4pm"},
		{"9a", "9am"},
		{"before 5pm", "5pm"},
		{"by tomorrow 8a", "tomorrow 8am"},
		{"  next monday  ", "next monday"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			vault := t.TempDir()
			writeNote(t, vault, "notes/todos/x.md", "---\ndue: "+tc.in+"\n---\nbody\n")

			srv := newTodoistServer(t)
			defer srv.Close()
			client := newTodoistClient(t, srv)
			if _, err := todoistsync.PushTasks(context.Background(), client, todoistsync.PushTasksInput{VaultRoot: vault}); err != nil {
				t.Fatalf("PushTasks: %v", err)
			}
			if got := srv.LastRequest().DueString; got != tc.want {
				t.Errorf("due_string = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPushTasks_FailedAPIRecorded(t *testing.T) {
	vault := t.TempDir()
	writeNote(t, vault, "notes/todos/bad.md", "---\ntitle: X\n---\nbody\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rejected", http.StatusBadRequest)
	}))
	defer srv.Close()
	client := newTodoistClientForServer(t, srv)

	res, err := todoistsync.PushTasks(context.Background(), client, todoistsync.PushTasksInput{VaultRoot: vault})
	if err != nil {
		t.Fatalf("PushTasks: %v", err)
	}
	if len(res.Failed) != 1 {
		t.Errorf("Failed = %v, want 1", res.Failed)
	}
}

func TestPushTasks_MissingTodosDir(t *testing.T) {
	vault := t.TempDir()
	srv := newTodoistServer(t)
	defer srv.Close()
	client := newTodoistClient(t, srv)
	res, err := todoistsync.PushTasks(context.Background(), client, todoistsync.PushTasksInput{VaultRoot: vault})
	if err != nil {
		t.Fatalf("PushTasks: %v", err)
	}
	if len(res.Synced)+len(res.Skipped)+len(res.Failed) != 0 {
		t.Errorf("expected empty result, got %+v", res)
	}
}

func TestPullCompletions_HappyPath(t *testing.T) {
	vault := t.TempDir()
	writeNote(t, vault, "notes/todos/a.md", "---\ntitle: A\nsynced-todoist: '100'\n---\nbody\n")
	writeNote(t, vault, "notes/todos/b.md", "---\ntitle: B\nsynced-todoist: '200'\n---\nbody\n")
	writeNote(t, vault, "notes/todos/c.md", "---\ntitle: C\nsynced-todoist: '300'\ndone: true\n---\nbody\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[
			{"id":"100","labels":["mind-palace"],"completed_at":"2026-05-20T12:34:56Z","content":"a"},
			{"id":"300","labels":["mind-palace"],"completed_at":"2026-05-21T01:00:00Z","content":"c"},
			{"id":"999","labels":["other"],"completed_at":"2026-05-22T00:00:00Z","content":"unrelated"}
		],"next_cursor":""}`))
	}))
	defer srv.Close()
	client := newTodoistClientForServer(t, srv)

	res, err := todoistsync.PullCompletions(context.Background(), client, todoistsync.PullCompletionsInput{
		VaultRoot: vault,
		Now:       time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("PullCompletions: %v", err)
	}
	if !sliceEqual(res.Completed, []string{"notes/todos/a.md"}) {
		t.Errorf("Completed = %v, want [notes/todos/a.md]", res.Completed)
	}
	got := readNote(t, vault, "notes/todos/a.md")
	if !strings.Contains(got, "done: true") {
		t.Errorf("missing done: true in:\n%s", got)
	}
	if !strings.Contains(got, "completed-date: '2026-05-20'") {
		t.Errorf("missing completed-date in:\n%s", got)
	}
	// b unchanged (no matching completion).
	if strings.Contains(readNote(t, vault, "notes/todos/b.md"), "done: true") {
		t.Errorf("b.md got marked done")
	}
	// c unchanged (already done).
	cContent := readNote(t, vault, "notes/todos/c.md")
	if strings.Count(cContent, "done: true") != 1 {
		t.Errorf("c.md done flag mutated; content:\n%s", cContent)
	}
}

// --- helpers ---

func writeNote(t *testing.T, vault, rel, content string) {
	t.Helper()
	p := filepath.Join(vault, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func readNote(t *testing.T, vault, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(vault, rel))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(b)
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func newTodoistClient(t *testing.T, srv *todoistServer) *todoist.Client {
	t.Helper()
	return newTodoistClientForServer(t, srv.Server)
}

func newTodoistClientForServer(t *testing.T, srv *httptest.Server) *todoist.Client {
	t.Helper()
	r := retryablehttp.NewClient()
	r.Logger = nil
	r.RetryMax = 1
	r.RetryWaitMin = time.Millisecond
	r.RetryWaitMax = 2 * time.Millisecond
	c := todoist.NewWithRetry("test-token", r)
	c.BaseURL = srv.URL
	return c
}

// todoistServer is a tiny httptest wrapper that records POSTed task
// payloads and replies with a synthesized incrementing id.
type todoistServer struct {
	Server   *httptest.Server
	mu       sync.Mutex
	nextID   int
	requests []todoist.CreateTaskRequest
}

func (s *todoistServer) Close()      { s.Server.Close() }
func (s *todoistServer) URL() string { return s.Server.URL }
func (s *todoistServer) CallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}
func (s *todoistServer) LastRequest() todoist.CreateTaskRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.requests) == 0 {
		return todoist.CreateTaskRequest{}
	}
	return s.requests[len(s.requests)-1]
}
func (s *todoistServer) LastID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nextID == 0 {
		return ""
	}
	id := s.nextID - 1
	return itoa(id + 1000)
}

func newTodoistServer(t *testing.T) *todoistServer {
	t.Helper()
	s := &todoistServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req todoist.CreateTaskRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		s.nextID++
		id := itoa(s.nextID + 999)
		s.requests = append(s.requests, req)
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + id + `"}`))
	}))
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	out := ""
	for n > 0 {
		d := n % 10
		out = string(rune('0'+d)) + out
		n /= 10
	}
	return out
}
