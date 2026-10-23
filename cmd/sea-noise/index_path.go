package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const indexDBFileName = "retrieval.db"

func defaultIndexDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".sea-noise", indexDBFileName), nil
}

func resolveIndexDBPath(vaultRoot, configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}

	path, err := defaultIndexDBPath()
	if err != nil {
		return "", err
	}

	legacy := filepath.Join(vaultRoot, "_meta", indexDBFileName)
	if _, err := os.Stat(legacy); err != nil {
		if os.IsNotExist(err) {
			return path, nil
		}
		return "", fmt.Errorf("inspect legacy index %s: %w", legacy, err)
	}
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect index %s: %w", path, err)
	}

	return "", fmt.Errorf(
		"legacy retrieval index found at %s; rebuild it at %s with `sea-noise index --db %s`",
		legacy, path, path,
	)
}
