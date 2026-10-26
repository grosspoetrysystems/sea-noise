package main

import (
	"fmt"
	"time"

	"sea-noise/internal/config"
	"sea-noise/internal/signal"
)

// runGenerateSignal implements `sea-noise generate-signal`.
// Mirrors references/scripts/generate-signal.py: assembles the rolling
// signal queue from unconsumed notes and writes it to signal/queue.md
// inside the configured vault.
func runGenerateSignal(args []string) error {
	vaultRoot, err := config.LoadVaultPath()
	if err != nil {
		return err
	}
	count, rel, err := signal.BuildAndWrite(vaultRoot, time.Now())
	if err != nil {
		return err
	}
	fmt.Printf("Signal queue generated: %d items → %s\n", count, rel)
	return nil
}
