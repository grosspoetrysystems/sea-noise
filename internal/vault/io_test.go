package vault

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestReadFileRetry_SuccessOnFirstAttempt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.md")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	raw, err := ReadFileRetry(path)
	if err != nil {
		t.Fatalf("ReadFileRetry: %v", err)
	}
	if string(raw) != "hello" {
		t.Errorf("got %q, want %q", raw, "hello")
	}
}

func TestReadFileRetry_NonEDEADLKErrorShortCircuits(t *testing.T) {
	// Path doesn't exist → ENOENT, not EDEADLK. Should NOT retry.
	start := time.Now()
	_, err := ReadFileRetry(filepath.Join(t.TempDir(), "absent.md"))
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected error on missing file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected ErrNotExist, got %v", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("non-EDEADLK error retried (elapsed %s)", elapsed)
	}
}

func TestReadFileRetry_RetriesOnEDEADLKThenSucceeds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "delayed.md")
	if err := os.WriteFile(path, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var calls int32
	withFakeReadFile(t, func(name string) ([]byte, error) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return nil, syscall.EDEADLK
		}
		return os.ReadFile(name)
	})

	cfg := retryConfig{
		initialBackoff: 1 * time.Millisecond,
		maxBackoff:     8 * time.Millisecond,
		ceiling:        100 * time.Millisecond,
	}
	raw, err := readFileRetryWith(path, cfg)
	if err != nil {
		t.Fatalf("readFileRetryWith: %v", err)
	}
	if string(raw) != "payload" {
		t.Errorf("got %q, want %q", raw, "payload")
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("calls = %d, want 3", got)
	}
}

func TestReadFileRetry_TimesOutAfterCeiling(t *testing.T) {
	withFakeReadFile(t, func(_ string) ([]byte, error) {
		return nil, syscall.EDEADLK
	})

	cfg := retryConfig{
		initialBackoff: 1 * time.Millisecond,
		maxBackoff:     4 * time.Millisecond,
		ceiling:        20 * time.Millisecond,
	}
	start := time.Now()
	_, err := readFileRetryWith("/whatever", cfg)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, syscall.EDEADLK) {
		t.Errorf("expected wrapped EDEADLK, got %v", err)
	}
	if elapsed < 20*time.Millisecond {
		t.Errorf("returned before ceiling: %s", elapsed)
	}
	// Generous upper bound — exponential growth + the last clamped
	// sleep shouldn't push elapsed much past the ceiling.
	if elapsed > 50*time.Millisecond {
		t.Errorf("ran significantly past ceiling: %s", elapsed)
	}
}

func withFakeReadFile(t *testing.T, fake func(string) ([]byte, error)) {
	t.Helper()
	prev := osReadFile
	osReadFile = fake
	t.Cleanup(func() { osReadFile = prev })
}
