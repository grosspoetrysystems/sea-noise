package main

import (
	"path/filepath"
	"testing"
)

func TestDefaultIndexDBPathUsesSeaNoiseHomeDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := defaultIndexDBPath()
	if err != nil {
		t.Fatalf("defaultIndexDBPath: %v", err)
	}
	want := filepath.Join(home, ".sea-noise", "retrieval.db")
	if got != want {
		t.Fatalf("defaultIndexDBPath: got %q want %q", got, want)
	}
}
