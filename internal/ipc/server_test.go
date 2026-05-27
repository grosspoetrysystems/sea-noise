package ipc_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"sea-noise/internal/ipc"
)

// fakeHandler is a controllable Handler for dispatcher tests. Each
// method records its calls and returns canned results / errors.
type fakeHandler struct {
	mu sync.Mutex

	skillOut    ipc.SkillRunResult
	skillErr    error
	skillCalls  []ipc.SkillRunParams
	pipelineOut ipc.PipelineTriggerResult
	pipelineErr error
	queryOut    ipc.VaultQueryResult
	queryErr    error
	indexOut    ipc.IndexRefreshResult
	indexErr    error
	statusOut   ipc.StatusResult
	statusErr   error
}

func (f *fakeHandler) SkillRun(_ context.Context, p ipc.SkillRunParams) (ipc.SkillRunResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.skillCalls = append(f.skillCalls, p)
	return f.skillOut, f.skillErr
}
func (f *fakeHandler) PipelineTrigger(_ context.Context, _ ipc.PipelineTriggerParams) (ipc.PipelineTriggerResult, error) {
	return f.pipelineOut, f.pipelineErr
}
func (f *fakeHandler) VaultQuery(_ context.Context, _ ipc.VaultQueryParams) (ipc.VaultQueryResult, error) {
	return f.queryOut, f.queryErr
}
func (f *fakeHandler) IndexRefresh(_ context.Context) (ipc.IndexRefreshResult, error) {
	return f.indexOut, f.indexErr
}
func (f *fakeHandler) Status(_ context.Context) (ipc.StatusResult, error) {
	return f.statusOut, f.statusErr
}

// shortSocketPath builds a socket path short enough to fit in
// sun_path even when t.TempDir lives under /var/folders. macOS caps
// sun_path at 104 bytes; we keep the filename a single character.
func shortSocketPath(t *testing.T) string {
	t.Helper()
	// /tmp is short and writable on darwin + linux; isolate per-test
	// with a random suffix to avoid collisions.
	dir, err := os.MkdirTemp("/tmp", "sn-ipc-")
	if err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "s")
}

// startServer launches an ipc.Server on an ephemeral socket and
// returns the path + a teardown.
func startServer(t *testing.T, h ipc.Handler) (string, func()) {
	t.Helper()
	path := shortSocketPath(t)
	srv := ipc.New(path, h)
	ctx, cancel := context.WithCancel(context.Background())
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("ipc start: %v", err)
	}
	return path, func() {
		cancel()
		_ = srv.Stop()
	}
}

// callOnce dials the socket, sends one request, reads one response,
// and hangs up. Used for the "simple client" path.
func callOnce(t *testing.T, socketPath string, method, id string, params any) ipc.Response {
	t.Helper()
	conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("marshal params: %v", err)
		}
		raw = b
	}
	enc := json.NewEncoder(conn)
	if err := enc.Encode(ipc.Request{ID: id, Method: method, Params: raw}); err != nil {
		t.Fatalf("encode req: %v", err)
	}

	dec := json.NewDecoder(conn)
	var resp ipc.Response
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("decode resp: %v", err)
	}
	return resp
}

func TestServer_SkillRun_Roundtrip(t *testing.T) {
	h := &fakeHandler{skillOut: ipc.SkillRunResult{Output: "hello"}}
	path, stop := startServer(t, h)
	defer stop()

	resp := callOnce(t, path, ipc.VerbSkillRun, "req-1", ipc.SkillRunParams{
		Name: "summarize-transcripts",
		Data: json.RawMessage(`{"note":"n1.md"}`),
	})

	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	if resp.ID != "req-1" {
		t.Fatalf("id: got %q want req-1", resp.ID)
	}
	var got ipc.SkillRunResult
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if got.Output != "hello" {
		t.Fatalf("output: got %q want hello", got.Output)
	}
	if len(h.skillCalls) != 1 || h.skillCalls[0].Name != "summarize-transcripts" {
		t.Fatalf("handler not called with expected params: %+v", h.skillCalls)
	}
}

func TestServer_PipelineTrigger_AlreadyRan(t *testing.T) {
	h := &fakeHandler{pipelineErr: ipc.ErrAlreadyRan}
	path, stop := startServer(t, h)
	defer stop()

	resp := callOnce(t, path, ipc.VerbPipelineTrigger, "", ipc.PipelineTriggerParams{Name: "daily"})
	if resp.Error == nil {
		t.Fatalf("expected error, got result %s", resp.Result)
	}
	if resp.Error.Code != ipc.ErrCodeAlreadyRan {
		t.Fatalf("code: got %q want %q", resp.Error.Code, ipc.ErrCodeAlreadyRan)
	}
}

func TestServer_PipelineTrigger_UnknownPipeline(t *testing.T) {
	h := &fakeHandler{pipelineErr: fmt.Errorf("%w: %q", ipc.ErrUnknownPipeline, "nope")}
	path, stop := startServer(t, h)
	defer stop()

	resp := callOnce(t, path, ipc.VerbPipelineTrigger, "", ipc.PipelineTriggerParams{Name: "nope"})
	if resp.Error == nil || resp.Error.Code != ipc.ErrCodeUnknownPipeline {
		t.Fatalf("expected unknown_pipeline error, got %+v", resp.Error)
	}
}

func TestServer_VaultQuery_Roundtrip(t *testing.T) {
	h := &fakeHandler{queryOut: ipc.VaultQueryResult{
		Mode: "FTSSearch",
		Hits: []ipc.VaultQueryHit{{NoteID: "topics/foo.md", Score: -1.2, Snippet: "snip"}},
	}}
	path, stop := startServer(t, h)
	defer stop()

	resp := callOnce(t, path, ipc.VerbVaultQuery, "", ipc.VaultQueryParams{
		Mode:  "fts",
		Query: "foo",
		Limit: 5,
	})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	var got ipc.VaultQueryResult
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Hits) != 1 || got.Hits[0].NoteID != "topics/foo.md" {
		t.Fatalf("hits: %+v", got.Hits)
	}
}

func TestServer_IndexRefresh_Roundtrip(t *testing.T) {
	h := &fakeHandler{indexOut: ipc.IndexRefreshResult{Notes: 12, Chunks: 47, Links: 8}}
	path, stop := startServer(t, h)
	defer stop()

	resp := callOnce(t, path, ipc.VerbIndexRefresh, "", nil)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	var got ipc.IndexRefreshResult
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Notes != 12 || got.Chunks != 47 || got.Links != 8 {
		t.Fatalf("stats wrong: %+v", got)
	}
}

func TestServer_Status_Roundtrip(t *testing.T) {
	h := &fakeHandler{statusOut: ipc.StatusResult{
		Version: ipc.Version, VaultRoot: "/tmp/vault", UptimeSec: 5,
	}}
	path, stop := startServer(t, h)
	defer stop()

	resp := callOnce(t, path, ipc.VerbStatus, "", nil)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	var got ipc.StatusResult
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.VaultRoot != "/tmp/vault" {
		t.Fatalf("vault_root: %q", got.VaultRoot)
	}
}

func TestServer_UnknownMethod(t *testing.T) {
	h := &fakeHandler{}
	path, stop := startServer(t, h)
	defer stop()

	resp := callOnce(t, path, "nope.invalid", "", nil)
	if resp.Error == nil || resp.Error.Code != ipc.ErrCodeUnknownMethod {
		t.Fatalf("expected unknown_method, got %+v", resp.Error)
	}
}

func TestServer_BadParams(t *testing.T) {
	h := &fakeHandler{}
	path, stop := startServer(t, h)
	defer stop()

	// Hand-roll a request whose params are a JSON string instead of
	// the expected object — params decoder must reject this with
	// bad_request.
	conn, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	bad := []byte(`{"id":"x","method":"skill.run","params":"not-an-object"}` + "\n")
	if _, err := conn.Write(bad); err != nil {
		t.Fatalf("write: %v", err)
	}
	var resp ipc.Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != ipc.ErrCodeBadRequest {
		t.Fatalf("expected bad_request, got %+v", resp.Error)
	}
}

func TestServer_PersistentConnection_MultipleRequests(t *testing.T) {
	h := &fakeHandler{skillOut: ipc.SkillRunResult{Output: "ok"}}
	path, stop := startServer(t, h)
	defer stop()

	conn, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("r-%d", i)
		if err := enc.Encode(ipc.Request{
			ID:     id,
			Method: ipc.VerbSkillRun,
			Params: json.RawMessage(`{"name":"x"}`),
		}); err != nil {
			t.Fatalf("encode: %v", err)
		}
		var resp ipc.Response
		if err := dec.Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.ID != id {
			t.Fatalf("id mismatch: got %q want %q", resp.ID, id)
		}
		if resp.Error != nil {
			t.Fatalf("err: %+v", resp.Error)
		}
	}
	if len(h.skillCalls) != 3 {
		t.Fatalf("handler should have 3 calls, got %d", len(h.skillCalls))
	}
}

func TestServer_ConcurrentConnections(t *testing.T) {
	h := &fakeHandler{skillOut: ipc.SkillRunResult{Output: "k"}}
	path, stop := startServer(t, h)
	defer stop()

	const n = 10
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			resp := callOnce(t, path, ipc.VerbSkillRun, fmt.Sprintf("c-%d", i), ipc.SkillRunParams{Name: "x"})
			if resp.Error != nil {
				t.Errorf("conn %d: err %+v", i, resp.Error)
			}
		}(i)
	}
	wg.Wait()
}

func TestServer_StopUnblocksAccept(t *testing.T) {
	h := &fakeHandler{}
	path := shortSocketPath(t)
	srv := ipc.New(path, h)
	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- srv.Stop() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("stop returned err: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return within 2s")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket file should be removed after Stop, got stat err: %v", err)
	}
}

func TestServer_StalePathRemovedOnStart(t *testing.T) {
	path := shortSocketPath(t)
	if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	h := &fakeHandler{}
	srv := ipc.New(path, h)
	defer srv.Stop()
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start over stale path: %v", err)
	}
}

func TestValidateSocketPath(t *testing.T) {
	if err := ipc.ValidateSocketPath("/tmp/short.sock"); err != nil {
		t.Fatalf("short path rejected: %v", err)
	}
	long := "/tmp/" + string(make([]byte, 200)) + ".sock"
	if err := ipc.ValidateSocketPath(long); err == nil {
		t.Fatalf("long path should be rejected")
	}
}
