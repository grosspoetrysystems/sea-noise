package url

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetcher_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello"))
	}))
	t.Cleanup(srv.Close)

	f := NewFetcher().WithClient(srv.Client())
	resp, err := f.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if resp.Status != http.StatusOK {
		t.Errorf("Status = %d, want 200", resp.Status)
	}
	if string(resp.Body) != "hello" {
		t.Errorf("Body = %q, want %q", resp.Body, "hello")
	}
	if resp.ContentType != "text/plain" {
		t.Errorf("ContentType = %q, want %q", resp.ContentType, "text/plain")
	}
}

func TestFetcher_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	f := NewFetcher().WithClient(srv.Client())
	_, err := f.Fetch(context.Background(), srv.URL)
	if !errors.Is(err, ErrNon2xx) {
		t.Fatalf("err = %v, want errors.Is(err, ErrNon2xx)", err)
	}
}

func TestFetcher_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block long enough that the context deadline fires first.
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	f := NewFetcher().WithClient(srv.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	t.Cleanup(cancel)
	_, err := f.Fetch(ctx, srv.URL)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want errors.Is(err, ErrTimeout)", err)
	}
}

func TestFetcher_NetworkError(t *testing.T) {
	f := NewFetcher()
	// Reserved TEST-NET-1 (RFC 5737), guaranteed unroutable.
	_, err := f.Fetch(context.Background(), "http://192.0.2.0:1/")
	if err == nil {
		t.Fatal("expected error reaching TEST-NET-1, got nil")
	}
	// Either ErrTimeout or ErrNetwork depending on resolution behavior.
	if !errors.Is(err, ErrNetwork) && !errors.Is(err, ErrTimeout) {
		t.Errorf("err = %v, want one of {ErrNetwork, ErrTimeout}", err)
	}
}
