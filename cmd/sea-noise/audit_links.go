package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"sea-noise/internal/auditlinks"
	"sea-noise/internal/config"
)

// runAuditLinks implements `sea-noise audit-links`.
// Mirrors references/scripts/audit-links.py: builds the broken-links
// report for the configured vault and writes it to _meta/broken-links.md.
func runAuditLinks(args []string) error {
	vaultRoot, err := config.LoadVaultPath()
	if err != nil {
		return err
	}

	report, err := auditlinks.BuildReport(vaultRoot, time.Now())
	if err != nil {
		return fmt.Errorf("build report: %w", err)
	}

	dest := filepath.Join(vaultRoot, "_meta", "broken-links.md")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dest), err)
	}
	if err := os.WriteFile(dest, []byte(report), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}

	// Mirror the prototype's final summary line.
	rel, _ := filepath.Rel(vaultRoot, dest)
	count := countBrokenLinksFromReport(report)
	fmt.Printf("Audit complete: %d broken links → %s\n", count, rel)
	return nil
}

// countBrokenLinksFromReport extracts the count from the "Found **N**"
// line in the report. Avoids re-running the scan; the report itself is
// the source of truth for the count.
func countBrokenLinksFromReport(report string) int {
	const marker = "Found **"
	idx := indexOf(report, marker)
	if idx < 0 {
		return 0
	}
	rest := report[idx+len(marker):]
	end := indexOf(rest, "**")
	if end < 0 {
		return 0
	}
	n := 0
	for _, c := range rest[:end] {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
