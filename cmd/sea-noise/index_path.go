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
