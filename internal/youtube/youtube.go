// Package youtube fetches public YouTube video transcripts.
//
// The Python prototype's fetch-youtube-transcript.py uses the
// youtube-transcript-api library, which scrapes the public watch page
// to discover caption tracks and then downloads the timedtext XML
// stream those tracks point at. There is no first-party public API
// for transcripts; this package mirrors the same scraping flow in Go
// so the port stays free of Python at runtime.
//
// The flow:
//  1. GET https://www.youtube.com/watch?v={videoID}
//  2. Extract the ytInitialPlayerResponse JSON embedded in the page.
//  3. Locate the captionTracks entry matching the requested language.
//  4. GET the track's baseUrl (XML transcript with <text> elements).
//  5. Concatenate the decoded <text> bodies, space-separated.
//
// Both the watch URL host and the transcript URL host can be
// overridden via Client.BaseURL for tests; the production default
// targets www.youtube.com.
package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/hashicorp/go-retryablehttp"
)

// DefaultBaseURL is the YouTube watch-page host.
const DefaultBaseURL = "https://www.youtube.com"

// ErrNoTranscript is returned when no caption track matches the
// requested languages (or none exist at all).
var ErrNoTranscript = errors.New("youtube: no transcript available")

// ErrPlayerResponseMissing is returned when the watch page does not
// contain the expected ytInitialPlayerResponse JSON.
var ErrPlayerResponseMissing = errors.New("youtube: player response not found in watch page")

// Client scrapes YouTube watch pages for caption metadata and
// downloads the timedtext transcripts.
type Client struct {
	http    *http.Client
	BaseURL string
}

// New returns a Client backed by a retryablehttp transport with default
// exponential backoff. The default BaseURL is the public YouTube host.
func New() *Client {
	r := retryablehttp.NewClient()
	r.Logger = nil
	return NewWithRetry(r)
}

// NewWithRetry returns a Client backed by a caller-provided
// retryablehttp configuration. Tests use this seam to compress retry
// timing and point at a local httptest.Server.
func NewWithRetry(r *retryablehttp.Client) *Client {
	if r == nil {
		r = retryablehttp.NewClient()
		r.Logger = nil
	}
	return &Client{
		http:    r.StandardClient(),
		BaseURL: DefaultBaseURL,
	}
}

var videoIDPattern = regexp.MustCompile(`(?:youtube\.com/watch\?v=|youtu\.be/)([\w-]{11})`)

// ExtractVideoID returns the 11-character video ID embedded in a
// YouTube URL, matching both youtube.com/watch?v=… and youtu.be/…
// shapes. The bool reports whether a match was found.
func ExtractVideoID(rawURL string) (string, bool) {
	m := videoIDPattern.FindStringSubmatch(rawURL)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// FetchTranscript returns the concatenated transcript text for the
// given video, preferring the first language in langs that has a
// caption track. When langs is empty, "en" is tried.
//
// Returns ErrNoTranscript if no matching track is available, or
// ErrPlayerResponseMissing if the watch page is malformed.
func (c *Client) FetchTranscript(ctx context.Context, videoID string, langs []string) (string, error) {
	if len(langs) == 0 {
		langs = []string{"en"}
	}
	tracks, err := c.fetchCaptionTracks(ctx, videoID)
	if err != nil {
		return "", err
	}
	track := pickTrack(tracks, langs)
	if track == nil {
		return "", ErrNoTranscript
	}
	return c.fetchTranscript(ctx, track.BaseURL)
}

type captionTrack struct {
	BaseURL      string `json:"baseUrl"`
	LanguageCode string `json:"languageCode"`
}

type playerResponse struct {
	Captions struct {
		PlayerCaptionsTracklistRenderer struct {
			CaptionTracks []captionTrack `json:"captionTracks"`
		} `json:"playerCaptionsTracklistRenderer"`
	} `json:"captions"`
}

func (c *Client) fetchCaptionTracks(ctx context.Context, videoID string) ([]captionTrack, error) {
	u := c.BaseURL + "/watch?v=" + videoID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("youtube: build watch request: %w", err)
	}
	// YouTube serves a captions-less player response to bots without a
	// realistic Accept-Language; the prototype gets one implicitly via
	// the youtube-transcript-api library. Set one explicitly here.
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("youtube: fetch watch page: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youtube: watch page status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("youtube: read watch page: %w", err)
	}

	raw, ok := extractPlayerResponse(body)
	if !ok {
		return nil, ErrPlayerResponseMissing
	}
	var pr playerResponse
	if err := json.Unmarshal(raw, &pr); err != nil {
		return nil, fmt.Errorf("youtube: decode player response: %w", err)
	}
	return pr.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks, nil
}

// extractPlayerResponse returns the JSON object assigned to
// ytInitialPlayerResponse in the watch page HTML. The page embeds it
// as `var ytInitialPlayerResponse = {…};` near the bottom of a
// <script> block; we scan for the assignment and balance braces to
// find the end of the JSON literal.
func extractPlayerResponse(page []byte) ([]byte, bool) {
	i := bytes.Index(page, []byte("ytInitialPlayerResponse"))
	if i < 0 {
		return nil, false
	}
	idx := i + len("ytInitialPlayerResponse")
	for idx < len(page) && page[idx] != '{' {
		idx++
	}
	if idx >= len(page) {
		return nil, false
	}
	start := idx
	depth, inStr, escaped := 0, false, false
	for ; idx < len(page); idx++ {
		ch := page[idx]
		if inStr {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return page[start : idx+1], true
			}
		}
	}
	return nil, false
}

func pickTrack(tracks []captionTrack, langs []string) *captionTrack {
	for _, want := range langs {
		for i, t := range tracks {
			if strings.EqualFold(t.LanguageCode, want) {
				return &tracks[i]
			}
		}
	}
	return nil
}

type transcript struct {
	XMLName xml.Name        `xml:"transcript"`
	Texts   []transcriptRow `xml:"text"`
}

type transcriptRow struct {
	Body string `xml:",chardata"`
}

func (c *Client) fetchTranscript(ctx context.Context, trackURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, trackURL, nil)
	if err != nil {
		return "", fmt.Errorf("youtube: build transcript request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("youtube: fetch transcript: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("youtube: transcript status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("youtube: read transcript: %w", err)
	}
	var doc transcript
	if err := xml.Unmarshal(body, &doc); err != nil {
		return "", fmt.Errorf("youtube: decode transcript xml: %w", err)
	}
	parts := make([]string, 0, len(doc.Texts))
	for _, row := range doc.Texts {
		s := strings.TrimSpace(html.UnescapeString(row.Body))
		if s == "" {
			continue
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " "), nil
}
