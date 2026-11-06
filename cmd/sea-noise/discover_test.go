package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"sea-noise/internal/discovery"
)

func TestDiscoverDoesNotCreateAMissingIndex(t *testing.T) {
	// The literal query delimiter in the filename must not become URI options.
	path := filepath.Join(t.TempDir(), "missing?mode=rw.db")
	err := runDiscover([]string{
		"--db", path,
		"--cli-command", "unused-discovery-provider",
		"--cli-args", "{PROMPT}",
		"--read-only-cli",
		"lantern",
	})
	if !errors.Is(err, discovery.ErrRetrieval) {
		t.Fatalf("expected an unavailable-index retrieval error, got %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("discovery must not create a missing index: %v", err)
	}
}

func TestDiscoverRejectsUnrepresentableTimeoutBeforeOpeningIndex(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("time.Duration overflow boundary is not representable by int flags")
	}
	path := filepath.Join(t.TempDir(), "missing.db")
	baseArgs := []string{
		"--db", path,
		"--cli-command", "unused-discovery-provider",
		"--cli-args", "{PROMPT}",
		"--read-only-cli",
	}
	controlArgs := append(baseArgs, "--timeout", strconv.FormatInt(maxDiscoverTimeoutSeconds, 10), "lantern")
	if err := runDiscover(controlArgs); !errors.Is(err, discovery.ErrRetrieval) {
		t.Fatalf("maximum representable timeout = %v, want ErrRetrieval", err)
	}

	overflowArgs := append(baseArgs, "--timeout", strconv.FormatInt(maxDiscoverTimeoutSeconds+1, 10), "lantern")
	err := runDiscover(overflowArgs)
	if err == nil {
		t.Fatal("unrepresentable timeout unexpectedly succeeded")
	}
	if errors.Is(err, discovery.ErrRetrieval) {
		t.Fatalf("timeout must be rejected before opening the index: %v", err)
	}
}
