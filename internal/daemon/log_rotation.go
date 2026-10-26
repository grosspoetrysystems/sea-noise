package daemon

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// DefaultLogRetentionDays is how many days of processing logs to keep
// in <vault>/_meta/processing-logs/. Older logs are deleted on every
// daily-pipeline fire via RotateProcessingLogs. Logs are useful for
// error catching when fresh; after a week they accumulate as vault
// noise.
const DefaultLogRetentionDays = 7

// logFileNameRe matches the canonical processing-log filename shape
// (YYYY-MM-DD.md). Files that don't match — README.md, .DS_Store,
// anything else a user may drop in by accident — are left alone.
var logFileNameRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})\.md$`)

// RotateProcessingLogs deletes processing-log files in
// <vaultRoot>/_meta/processing-logs/ whose filename date is older
// than retentionDays. Date comparison uses the filename (canonical
// "what day was this for"), not filesystem mtime — robust to clock
// skew, vault sync, and manual file copies.
//
// Returns the count of files deleted. Errors during deletion of
// individual files are tolerated (a stuck file shouldn't block the
// whole rotation); the first IO error against the directory itself
// surfaces.
//
// Pass retentionDays = 0 for DefaultLogRetentionDays.
func RotateProcessingLogs(vaultRoot string, now time.Time, retentionDays int) (int, error) {
	if retentionDays <= 0 {
		retentionDays = DefaultLogRetentionDays
	}
	return rotateLogsImpl(filepath.Join(vaultRoot, logsDirRel), now, retentionDays)
}

func rotateLogsImpl(dir string, now time.Time, retentionDays int) (int, error) {
	cutoff := now.AddDate(0, 0, -retentionDays)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("daemon: read log dir %s: %w", dir, err)
	}

	deleted := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := logFileNameRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		date, err := time.Parse("2006-01-02", m[1])
		if err != nil {
			continue
		}
		if date.Before(cutoff) {
			if err := os.Remove(filepath.Join(dir, e.Name())); err == nil {
				deleted++
			}
			// Tolerate per-file errors — a stuck file shouldn't
			// block the rotation. Next run will pick up the survivors.
		}
	}
	return deleted, nil
}
