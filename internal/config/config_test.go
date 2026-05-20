package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadVaultPath_EnvVarWinsOverConfigFile(t *testing.T) {
	dir := t.TempDir()
	otherDir := t.TempDir()

	tomlPath := filepath.Join(dir, ConfigFileName)
	if err := os.WriteFile(tomlPath, []byte(`vault.path = "`+otherDir+`"`), 0o644); err != nil {
		t.Fatal(err)
	}

	chdir(t, dir)
	t.Setenv(EnvVar, dir)

	got, err := LoadVaultPath()
	if err != nil {
		t.Fatalf("LoadVaultPath() unexpected error: %v", err)
	}
	wantAbs, _ := filepath.Abs(dir)
	if got != wantAbs {
		t.Errorf("LoadVaultPath() = %q, want env-var value %q (config file held %q)", got, wantAbs, otherDir)
	}
}

func TestLoadVaultPath_ConfigFileFallback(t *testing.T) {
	dir := t.TempDir()
	vault := t.TempDir()
	tomlPath := filepath.Join(dir, ConfigFileName)
	if err := os.WriteFile(tomlPath, []byte(`# comment line
vault.path = "`+vault+`"
other.key = "ignored"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	chdir(t, dir)
	t.Setenv(EnvVar, "")

	got, err := LoadVaultPath()
	if err != nil {
		t.Fatalf("LoadVaultPath() unexpected error: %v", err)
	}
	wantAbs, _ := filepath.Abs(vault)
	if got != wantAbs {
		t.Errorf("LoadVaultPath() = %q, want %q", got, wantAbs)
	}
}

func TestLoadVaultPath_NothingConfigured(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Setenv(EnvVar, "")

	_, err := LoadVaultPath()
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("LoadVaultPath() = %v, want errors.Is(err, ErrNotConfigured)", err)
	}
}

func TestLoadVaultPath_EnvVarPathMissing(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Setenv(EnvVar, filepath.Join(dir, "does-not-exist"))

	_, err := LoadVaultPath()
	if !errors.Is(err, ErrPathMissing) {
		t.Fatalf("LoadVaultPath() = %v, want errors.Is(err, ErrPathMissing)", err)
	}
}

func TestLoadVaultPath_PathPointsAtFile(t *testing.T) {
	dir := t.TempDir()
	notADir := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(notADir, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	chdir(t, dir)
	t.Setenv(EnvVar, notADir)

	_, err := LoadVaultPath()
	if !errors.Is(err, ErrPathNotDir) {
		t.Fatalf("LoadVaultPath() = %v, want errors.Is(err, ErrPathNotDir)", err)
	}
}

func TestSplitKeyValue(t *testing.T) {
	cases := []struct {
		in        string
		wantKey   string
		wantValue string
		wantOK    bool
	}{
		{`vault.path = "abc"`, "vault.path", "abc", true},
		{`vault.path="abc"`, "vault.path", "abc", true},
		{`  vault.path  =  "abc"  `, "vault.path", "abc", true},
		{`# comment`, "", "", false},
		{`vault.path = abc`, "", "", false},
		{``, "", "", false},
		{`vault.path = "`, "", "", false},
	}
	for _, c := range cases {
		k, v, ok := splitKeyValue(c.in)
		if k != c.wantKey || v != c.wantValue || ok != c.wantOK {
			t.Errorf("splitKeyValue(%q) = (%q,%q,%v), want (%q,%q,%v)",
				c.in, k, v, ok, c.wantKey, c.wantValue, c.wantOK)
		}
	}
}

// chdir is a t.TempDir-style helper: change to dir for the duration of
// the test, restoring afterwards.
func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		os.Chdir(prev)
	})
}
