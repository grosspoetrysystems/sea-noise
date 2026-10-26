package parity

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// LoadFixture copies the contents of srcDir into a per-test temp dir
// and returns the writable destination root. The returned path is
// safe to mutate; cleanup is handled by t.TempDir.
//
// Failures (missing source, copy errors) abort the test via t.Fatalf.
// Callers that need to assert the sentinel error directly can use the
// unexported loadFixtureInto core via export_test.go.
//
// Non-regular filesystem entries (pipes, devices) are skipped silently;
// a fixture vault should never contain them. File modes are normalized
// to 0o644 / 0o755 so tests run consistently across umasks.
func LoadFixture(t *testing.T, srcDir string) string {
	t.Helper()
	dst, err := loadFixtureInto(srcDir, t.TempDir())
	if err != nil {
		t.Fatalf("parity load fixture: %v", err)
	}
	return dst
}

// loadFixtureInto is the non-fatal core of LoadFixture. It returns
// ErrFixtureMissing (wrapped) when srcDir does not exist or is not a
// directory; any other error from the copy walk is wrapped verbatim.
func loadFixtureInto(srcDir, dstDir string) (string, error) {
	info, err := os.Stat(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", ErrFixtureMissing, srcDir)
		}
		return "", fmt.Errorf("parity: stat fixture %s: %w", srcDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%w: %s is not a directory", ErrFixtureMissing, srcDir)
	}
	if err := copyTree(srcDir, dstDir); err != nil {
		return "", fmt.Errorf("parity: copy %s -> %s: %w", srcDir, dstDir, err)
	}
	return dstDir, nil
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		switch {
		case info.IsDir():
			return os.MkdirAll(target, 0o755)
		case info.Mode().IsRegular():
			return copyFile(path, target)
		default:
			// Skip non-regular entries (pipes, devices) silently; a
			// fixture vault should never contain them.
			return nil
		}
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir dst parent: %w", err)
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open dst: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return nil
}
