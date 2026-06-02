package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRotateProcessingLogs_DeletesOldKeepsRecent(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	// Files older than 7 days: should be deleted.
	mustWriteLog(t, dir, "2026-06-01.md") // 9 days old
	mustWriteLog(t, dir, "2026-06-02.md") // 8 days old
	// Files within retention window: should be kept.
	mustWriteLog(t, dir, "2026-06-04.md") // 6 days old
	mustWriteLog(t, dir, "2026-06-10.md") // today
	// Non-log file: should be untouched even if old.
	mustWriteLog(t, dir, "README.md")

	deleted, err := rotateLogsImpl(dir, now, DefaultLogRetentionDays)
	if err != nil {
		t.Fatalf("rotateLogsImpl: %v", err)
	}
	if deleted != 2 {
		t.Errorf("deleted = %d, want 2", deleted)
	}

	for _, name := range []string{"2026-06-04.md", "2026-06-10.md", "README.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected %s to be kept, got %v", name, err)
		}
	}
	for _, name := range []string{"2026-06-01.md", "2026-06-02.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("expected %s to be deleted", name)
		}
	}
}

func TestRotateProcessingLogs_MissingDirIsNoop(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")
	deleted, err := rotateLogsImpl(dir, time.Now(), DefaultLogRetentionDays)
	if err != nil {
		t.Errorf("expected nil error on missing dir, got %v", err)
	}
	if deleted != 0 {
		t.Errorf("deleted = %d, want 0", deleted)
	}
}

func TestRotateProcessingLogs_LeavesNonMatchingFilesAlone(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	mustWriteLog(t, dir, "2026-06-01.md") // old, should delete
	mustWriteLog(t, dir, "notes.md")      // not date-shaped, leave
	mustWriteLog(t, dir, "2026-06-01.txt") // wrong extension, leave
	mustWriteLog(t, dir, "2026-13-99.md")  // invalid date, leave

	deleted, err := rotateLogsImpl(dir, now, DefaultLogRetentionDays)
	if err != nil {
		t.Fatalf("rotateLogsImpl: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1 (only the valid YYYY-MM-DD.md old file)", deleted)
	}
	for _, name := range []string{"notes.md", "2026-06-01.txt", "2026-13-99.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected %s to be untouched, got %v", name, err)
		}
	}
}

func mustWriteLog(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("## test\n\n- step — OK\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
