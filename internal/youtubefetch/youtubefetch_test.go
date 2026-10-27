package youtubefetch_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/go-retryablehttp"

	"sea-noise/internal/youtube"
	"sea-noise/internal/youtubefetch"
)

func TestFetchTranscripts_HappyPath(t *testing.T) {
	vault := t.TempDir()
	writeNote(t, vault, "notes/references/talk.md", `---
title: Demo Talk
source: https://www.youtube.com/watch?v=vid12345678
---

Some pre-existing notes about this video.
`)

	srv := newYouTubeServer(t, map[string]string{
		"vid12345678": "hello from the test transcript",
	})
	defer srv.Close()
	client := newYouTubeClient(t, srv)

	res, err := youtubefetch.FetchTranscripts(context.Background(), client, youtubefetch.Input{VaultRoot: vault})
	if err != nil {
		t.Fatalf("FetchTranscripts: %v", err)
	}
	if len(res.Updated) != 1 || res.Updated[0] != "notes/references/talk.md" {
		t.Fatalf("Updated = %v, want [notes/references/talk.md]", res.Updated)
	}
	if len(res.Skipped) != 0 || len(res.Failed) != 0 {
		t.Errorf("Skipped=%v Failed=%v, want both empty", res.Skipped, res.Failed)
	}

	got := readNote(t, vault, "notes/references/talk.md")
	want := `---
title: Demo Talk
source: https://www.youtube.com/watch?v=vid12345678
---

Some pre-existing notes about this video.

## Transcript

hello from the test transcript
`
	if got != want {
		t.Errorf("note mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFetchTranscripts_SkipsAlreadyEnriched(t *testing.T) {
	vault := t.TempDir()
	original := `---
title: Already Done
source: https://youtu.be/vid12345678
---

Body.

## Transcript

old text
`
	writeNote(t, vault, "notes/references/done.md", original)

	srv := newYouTubeServer(t, map[string]string{"vid12345678": "SHOULD NOT BE FETCHED"})
	defer srv.Close()
	client := newYouTubeClient(t, srv)

	res, err := youtubefetch.FetchTranscripts(context.Background(), client, youtubefetch.Input{VaultRoot: vault})
	if err != nil {
		t.Fatalf("FetchTranscripts: %v", err)
	}
	if len(res.Updated) != 0 {
		t.Errorf("Updated = %v, want empty", res.Updated)
	}
	if len(res.Skipped) != 1 {
		t.Errorf("Skipped = %v, want 1", res.Skipped)
	}
	if got := readNote(t, vault, "notes/references/done.md"); got != original {
		t.Errorf("note mutated when it should not have been")
	}
}

func TestFetchTranscripts_SkipsNonYouTubeSource(t *testing.T) {
	vault := t.TempDir()
	writeNote(t, vault, "notes/fleeting/blog.md", `---
title: Blog
source: https://example.com/post
---

body
`)

	srv := newYouTubeServer(t, nil)
	defer srv.Close()
	client := newYouTubeClient(t, srv)

	res, err := youtubefetch.FetchTranscripts(context.Background(), client, youtubefetch.Input{VaultRoot: vault})
	if err != nil {
		t.Fatalf("FetchTranscripts: %v", err)
	}
	if len(res.Updated) != 0 || len(res.Failed) != 0 {
		t.Errorf("expected only Skipped; got Updated=%v Failed=%v", res.Updated, res.Failed)
	}
}

func TestFetchTranscripts_FailedFetch(t *testing.T) {
	vault := t.TempDir()
	writeNote(t, vault, "inbox/talk.md", `---
source: https://youtu.be/vid99999999
---

body
`)

	srv := newYouTubeServer(t, nil) // empty map → no captions → ErrNoTranscript
	defer srv.Close()
	client := newYouTubeClient(t, srv)

	res, err := youtubefetch.FetchTranscripts(context.Background(), client, youtubefetch.Input{VaultRoot: vault})
	if err != nil {
		t.Fatalf("FetchTranscripts: %v", err)
	}
	if len(res.Failed) != 1 {
		t.Errorf("Failed = %v, want 1", res.Failed)
	}
}

func TestFetchTranscripts_ReadFailureDoesNotAbortLaterNotes(t *testing.T) {
	vault := t.TempDir()
	unreadable := filepath.Join(vault, "notes/references/a-unreadable.md")
	if err := os.MkdirAll(filepath.Dir(unreadable), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(vault, "missing.md"), unreadable); err != nil {
		t.Fatal(err)
	}
	writeNote(t, vault, "notes/references/z-good.md", `---
source: https://youtu.be/vid12345678
---

body
`)

	srv := newYouTubeServer(t, map[string]string{"vid12345678": "transcript"})
	defer srv.Close()

	res, err := youtubefetch.FetchTranscripts(context.Background(), newYouTubeClient(t, srv), youtubefetch.Input{VaultRoot: vault})
	if err != nil {
		t.Fatalf("FetchTranscripts: %v", err)
	}
	if len(res.Failed) != 1 || res.Failed[0] != "notes/references/a-unreadable.md" {
		t.Fatalf("Failed = %v, want [notes/references/a-unreadable.md]", res.Failed)
	}
	if len(res.Updated) != 1 || res.Updated[0] != "notes/references/z-good.md" {
		t.Fatalf("Updated = %v, want [notes/references/z-good.md]", res.Updated)
	}
}

func TestFetchTranscripts_MissingDirsAreSkipped(t *testing.T) {
	vault := t.TempDir()
	// no notes/, no inbox/ — every search dir is missing
	srv := newYouTubeServer(t, nil)
	defer srv.Close()
	client := newYouTubeClient(t, srv)

	res, err := youtubefetch.FetchTranscripts(context.Background(), client, youtubefetch.Input{VaultRoot: vault})
	if err != nil {
		t.Fatalf("FetchTranscripts: %v", err)
	}
	if len(res.Updated)+len(res.Skipped)+len(res.Failed) != 0 {
		t.Errorf("expected zero results, got %+v", res)
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

func newYouTubeClient(t *testing.T, srv *httptest.Server) *youtube.Client {
	t.Helper()
	r := retryablehttp.NewClient()
	r.Logger = nil
	r.RetryMax = 1
	r.RetryWaitMin = time.Millisecond
	r.RetryWaitMax = 2 * time.Millisecond
	c := youtube.NewWithRetry(r)
	c.BaseURL = srv.URL
	return c
}

// newYouTubeServer returns an httptest server that serves a fake
// watch-page + timedtext XML. videoIDToTranscript maps the requested
// video id to the text that the en-track will return. A missing entry
// causes the watch page to advertise zero caption tracks (which the
// client surfaces as ErrNoTranscript).
func newYouTubeServer(t *testing.T, videoIDToTranscript map[string]string) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/watch":
			vid := r.URL.Query().Get("v")
			tracks := []map[string]any{}
			if _, ok := videoIDToTranscript[vid]; ok {
				tracks = []map[string]any{
					{
						"baseUrl":      srv.URL + "/timedtext?v=" + vid + "&lang=en",
						"languageCode": "en",
					},
				}
			}
			player := map[string]any{
				"captions": map[string]any{
					"playerCaptionsTracklistRenderer": map[string]any{
						"captionTracks": tracks,
					},
				},
			}
			b, _ := json.Marshal(player)
			fmt.Fprintf(w, `<html><script>var ytInitialPlayerResponse = %s;</script></html>`, b)
		case "/timedtext":
			vid := r.URL.Query().Get("v")
			text := videoIDToTranscript[vid]
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<transcript><text start="0" dur="1">%s</text></transcript>`, text)
		default:
			t.Errorf("unexpected request path %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	return srv
}
