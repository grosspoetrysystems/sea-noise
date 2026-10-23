package main

import (
	"os"
	"path/filepath"
	"strings"
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

func TestResolveIndexDBPathRejectsLegacyOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vault := t.TempDir()
	legacy := filepath.Join(vault, "_meta", indexDBFileName)
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := resolveIndexDBPath(vault, "")
	if err == nil {
		t.Fatal("resolveIndexDBPath: expected legacy index error")
	}
	if !strings.Contains(err.Error(), "legacy retrieval index found") {
		t.Fatalf("resolveIndexDBPath error = %v", err)
	}
	if !strings.Contains(err.Error(), "sea-noise index --db") {
		t.Fatalf("resolveIndexDBPath error lacks rebuild command: %v", err)
	}
}

func TestResolveIndexDBPathUsesNewPathWhenPresent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vault := t.TempDir()
	legacy := filepath.Join(vault, "_meta", indexDBFileName)
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".sea-noise", indexDBFileName)
	if err := os.MkdirAll(filepath.Dir(want), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveIndexDBPath(vault, "")
	if err != nil {
		t.Fatalf("resolveIndexDBPath: %v", err)
	}
	if got != want {
		t.Fatalf("resolveIndexDBPath: got %q want %q", got, want)
	}
}

func TestBuildDaemonDepsRejectsLegacyOnlyIndex(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vault := t.TempDir()
	t.Setenv("MIND_PALACE_VAULT", vault)
	legacy := filepath.Join(vault, "_meta", indexDBFileName)
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := buildDaemonDeps("", "", "", 0, "")
	if err == nil {
		t.Fatal("buildDaemonDeps: expected legacy index error")
	}
	if !strings.Contains(err.Error(), "legacy retrieval index found") {
		t.Fatalf("buildDaemonDeps error = %v", err)
	}
}
