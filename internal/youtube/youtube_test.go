package youtube_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/go-retryablehttp"

	"sea-noise/internal/youtube"
)

func TestExtractVideoID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"watch_url", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ", true},
		{"watch_url_with_extra_params", "https://www.youtube.com/watch?v=abcDEFghi_1&t=42s", "abcDEFghi_1", true},
		{"short_url", "https://youtu.be/abcDEFghi_1", "abcDEFghi_1", true},
		{"non_youtube", "https://example.com/watch?v=abcDEFghi_1", "", false},
		{"empty", "", "", false},
		{"short_id_too_short", "https://youtu.be/abc", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := youtube.ExtractVideoID(tc.in)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func newTestClient(t *testing.T, srv *httptest.Server) *youtube.Client {
	t.Helper()
	r := retryablehttp.NewClient()
	r.Logger = nil
	r.RetryMax = 2
	r.RetryWaitMin = time.Millisecond
	r.RetryWaitMax = 2 * time.Millisecond
	c := youtube.NewWithRetry(r)
	c.BaseURL = srv.URL
	return c
}

func TestFetchTranscript_Success(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/watch":
			if r.URL.Query().Get("v") != "vid12345678" {
				t.Errorf("video id query = %q", r.URL.Query().Get("v"))
			}
			player := map[string]any{
				"captions": map[string]any{
					"playerCaptionsTracklistRenderer": map[string]any{
						"captionTracks": []map[string]any{
							{"baseUrl": srv.URL + "/timedtext?lang=en", "languageCode": "en"},
							{"baseUrl": srv.URL + "/timedtext?lang=es", "languageCode": "es"},
						},
					},
				},
			}
			b, _ := json.Marshal(player)
			fmt.Fprintf(w, `<html><script>var ytInitialPlayerResponse = %s; var other = 1;</script></html>`, b)
		case "/timedtext":
			lang := r.URL.Query().Get("lang")
			if lang != "en" {
				t.Errorf("expected en timedtext, got %q", lang)
			}
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<transcript>
<text start="0.0" dur="1.5">Hello &amp; welcome</text>
<text start="1.5" dur="1.0">to the show</text>
<text start="2.5" dur="0.5">  </text>
</transcript>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := c.FetchTranscript(context.Background(), "vid12345678", []string{"en"})
	if err != nil {
		t.Fatalf("FetchTranscript: %v", err)
	}
	want := "Hello & welcome to the show"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFetchTranscript_NoMatchingLanguage(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/watch" {
			http.NotFound(w, r)
			return
		}
		player := map[string]any{
			"captions": map[string]any{
				"playerCaptionsTracklistRenderer": map[string]any{
					"captionTracks": []map[string]any{
						{"baseUrl": srv.URL + "/timedtext?lang=fr", "languageCode": "fr"},
					},
				},
			},
		}
		b, _ := json.Marshal(player)
		fmt.Fprintf(w, `<html><script>var ytInitialPlayerResponse = %s;</script></html>`, b)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.FetchTranscript(context.Background(), "vid12345678", []string{"en"})
	if !errors.Is(err, youtube.ErrNoTranscript) {
		t.Fatalf("err = %v, want ErrNoTranscript", err)
	}
}

func TestFetchTranscript_PlayerResponseMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>no captions here</body></html>`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.FetchTranscript(context.Background(), "vid12345678", []string{"en"})
	if !errors.Is(err, youtube.ErrPlayerResponseMissing) {
		t.Fatalf("err = %v, want ErrPlayerResponseMissing", err)
	}
}

func TestFetchTranscript_PrefersFirstLanguage(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/watch":
			player := map[string]any{
				"captions": map[string]any{
					"playerCaptionsTracklistRenderer": map[string]any{
						"captionTracks": []map[string]any{
							{"baseUrl": srv.URL + "/timedtext?lang=es", "languageCode": "es"},
							{"baseUrl": srv.URL + "/timedtext?lang=en", "languageCode": "en"},
						},
					},
				},
			}
			b, _ := json.Marshal(player)
			fmt.Fprintf(w, `<html><script>var ytInitialPlayerResponse = %s;</script></html>`, b)
		case "/timedtext":
			lang := r.URL.Query().Get("lang")
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<transcript><text start="0" dur="1">%s line</text></transcript>`, lang)
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := c.FetchTranscript(context.Background(), "vid12345678", []string{"es", "en"})
	if err != nil {
		t.Fatalf("FetchTranscript: %v", err)
	}
	if got != "es line" {
		t.Errorf("got %q, want %q (first language wins)", got, "es line")
	}
}
