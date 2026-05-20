// Package config resolves runtime configuration.
//
// For this scaffold the only configuration surface is the vault path,
// resolved from MIND_PALACE_VAULT env or a vault.path entry in
// mind-palace.toml. Both sources resolve to an existing directory or
// LoadVaultPath returns a clear error.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ConfigFileName is the name of the optional TOML config file at the
// repo root or the working directory.
const ConfigFileName = "mind-palace.toml"

// EnvVar is the environment variable that overrides the config file's
// vault.path.
const EnvVar = "MIND_PALACE_VAULT"

// LoadVaultPath resolves the vault path. MIND_PALACE_VAULT wins over
// mind-palace.toml if both are set. The resolved path must point to an
// existing directory.
func LoadVaultPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv(EnvVar)); p != "" {
		return verifyDir(p, fmt.Sprintf("%s env var", EnvVar))
	}

	p, err := readVaultPathFromFile(ConfigFileName)
	if err != nil {
		return "", err
	}
	if p == "" {
		return "", fmt.Errorf(
			"no vault path configured: set %s env var or add vault.path to %s",
			EnvVar, ConfigFileName,
		)
	}
	return verifyDir(p, fmt.Sprintf("vault.path in %s", ConfigFileName))
}

// readVaultPathFromFile reads vault.path from the named file. Returns
// ("", nil) if the file does not exist. Returns ("", error) on read or
// parse errors.
//
// The reader honors a small TOML subset: it looks for a line of the form
//   vault.path = "..."
// (with optional surrounding whitespace, optional inline comment after
// the value). Anything else is ignored. A full TOML parser is not in
// scope at this scaffold wave; subsequent waves can swap this for a
// real parser when more keys are added.
func readVaultPathFromFile(name string) (string, error) {
	f, err := os.Open(name)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("open %s: %w", name, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := splitKeyValue(line)
		if !ok {
			continue
		}
		if key != "vault.path" {
			continue
		}
		return value, nil
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read %s: %w", name, err)
	}
	return "", nil
}

// splitKeyValue parses a single TOML-shaped `key = "value"` line.
// Inline comments after the closing quote are stripped.
func splitKeyValue(line string) (key, value string, ok bool) {
	eq := strings.Index(line, "=")
	if eq < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:eq])
	rest := strings.TrimSpace(line[eq+1:])
	if len(rest) < 2 || rest[0] != '"' {
		return "", "", false
	}
	end := strings.IndexByte(rest[1:], '"')
	if end < 0 {
		return "", "", false
	}
	value = rest[1 : 1+end]
	return key, value, true
}

func verifyDir(path, source string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", source, err)
	}
	info, err := os.Stat(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("%s points to %q which does not exist", source, abs)
	}
	if err != nil {
		return "", fmt.Errorf("stat %s (%s): %w", abs, source, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s points to %q which is not a directory", source, abs)
	}
	return abs, nil
}
