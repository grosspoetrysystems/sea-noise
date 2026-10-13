package vault

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// os* indirections so tests can swap in fakes if needed. At this scaffold
// stage they are direct passthroughs to the stdlib; no swapping yet.

var (
	osStat     = func(name string) (fs.FileInfo, error) { return os.Stat(name) }
	osReadFile = func(name string) ([]byte, error) { return os.ReadFile(name) }
	osWalkDir  = func(root string, fn fs.WalkDirFunc) error { return filepath.WalkDir(root, fn) }
)

// ReadFileRetry wraps os.ReadFile with exponential-backoff retry on
// macOS iCloud Drive's "resource deadlock avoided" (syscall.EDEADLK).
// fsnotify can fire a Create event the instant iCloud creates a
// placeholder file, before iCloud has finished materializing the
// content from the cloud. The kernel returns EDEADLK on read until
// materialization completes.
//
// Backoff schedule: 250ms, 500ms, 1s, 2s, 4s, 8s, 8s, ...
// (caps at 8s per sleep). Default ceiling 30s. Empirical observation
// in dogfood: warm-cached files materialize in <1s; cold or large
// (~50KB) clips can take 10-15s on residential connections; very
// large clips (>500KB) may need more.
//
// Any non-EDEADLK error short-circuits (no retry).
//
// This is the single source of truth for iCloud-tolerant file reads
// across the codebase. Callers in fetchstub, daemon, summarizer,
// etc. should use this instead of os.ReadFile when reading from a
// path under the vault root.
func ReadFileRetry(path string) ([]byte, error) {
	return ReadFileRetryContext(context.Background(), path)
}

// ReadFileRetryContext is ReadFileRetry with cancellation during retries.
func ReadFileRetryContext(ctx context.Context, path string) ([]byte, error) {
	return readFileRetryWithContext(ctx, path, defaultRetryConfig)
}

// retryConfig is exposed via the readFileRetryWith helper for tests
// that need shorter windows; production callers use the package
// default.
type retryConfig struct {
	initialBackoff time.Duration
	maxBackoff     time.Duration
	ceiling        time.Duration
}

var defaultRetryConfig = retryConfig{
	initialBackoff: 250 * time.Millisecond,
	maxBackoff:     8 * time.Second,
	ceiling:        30 * time.Second,
}

func readFileRetryWith(path string, cfg retryConfig) ([]byte, error) {
	return readFileRetryWithContext(context.Background(), path, cfg)
}

func readFileRetryWithContext(ctx context.Context, path string, cfg retryConfig) ([]byte, error) {
	backoff := cfg.initialBackoff
	var elapsed time.Duration
	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		raw, err := osReadFile(path)
		if err == nil {
			return raw, nil
		}
		lastErr = err
		if !errors.Is(err, syscall.EDEADLK) {
			return nil, err
		}
		if elapsed >= cfg.ceiling {
			return nil, fmt.Errorf("vault: read %s: iCloud materialization timeout after %s: %w", path, elapsed.Round(time.Second), lastErr)
		}
		remaining := cfg.ceiling - elapsed
		sleep := backoff
		if sleep > remaining {
			sleep = remaining
		}
		timer := time.NewTimer(sleep)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
		elapsed += sleep
		backoff *= 2
		if backoff > cfg.maxBackoff {
			backoff = cfg.maxBackoff
		}
	}
}
