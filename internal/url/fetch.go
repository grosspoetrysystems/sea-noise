package url

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Sentinel errors for the HTTP fetcher. Callers check via errors.Is.
var (
	// ErrNon2xx is returned when an HTTP response has a status code
	// outside the 2xx range. The error string includes the status; the
	// sentinel locks the contract.
	ErrNon2xx = errors.New("non-2xx response")

	// ErrTimeout is returned when the fetch exceeds the context deadline
	// or the configured per-request timeout.
	ErrTimeout = errors.New("request timed out")

	// ErrNetwork is returned for any other transport-level failure.
	ErrNetwork = errors.New("network error")
)

// Response is the result of a successful Fetch.
type Response struct {
	Body        []byte
	ContentType string
	Status      int
}

// Fetcher is a small wrapper around http.Client. Construct via
// NewFetcher; tests can swap the underlying client by passing one in.
type Fetcher struct {
	client *http.Client
}

// NewFetcher returns a Fetcher with a sensible default timeout. Callers
// that need a different timeout pass it via WithTimeout.
func NewFetcher() *Fetcher {
	return &Fetcher{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// WithTimeout returns a new Fetcher whose underlying client uses the
// provided overall timeout.
func (f *Fetcher) WithTimeout(d time.Duration) *Fetcher {
	return &Fetcher{client: &http.Client{Timeout: d}}
}

// WithClient returns a new Fetcher using the supplied http.Client. Used
// by tests that need to point at an httptest.Server.
func (f *Fetcher) WithClient(c *http.Client) *Fetcher {
	return &Fetcher{client: c}
}

// Fetch performs a GET request against rawURL and returns the body
// plus content-type and status. No retries are performed at this layer;
// callers in the pipeline-commands layer choose retry behavior.
//
// The fetcher does NOT call Normalize on rawURL — callers normalize
// first if they care about deduplication; some callers want the
// original URL preserved (e.g., for redirected hosts that depend on
// trailing-slash semantics).
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %v", ErrNetwork, err)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isTimeoutErr(err) {
			return nil, fmt.Errorf("%w: %v", ErrTimeout, err)
		}
		return nil, fmt.Errorf("%w: %v", ErrNetwork, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read body: %v", ErrNetwork, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: status %d", ErrNon2xx, resp.StatusCode)
	}

	return &Response{
		Body:        body,
		ContentType: resp.Header.Get("Content-Type"),
		Status:      resp.StatusCode,
	}, nil
}

// isTimeoutErr unwraps the chain to find a net.Error with Timeout()==true.
func isTimeoutErr(err error) bool {
	for ; err != nil; err = errors.Unwrap(err) {
		type timeoutErr interface {
			Timeout() bool
		}
		if te, ok := err.(timeoutErr); ok && te.Timeout() {
			return true
		}
	}
	return false
}
