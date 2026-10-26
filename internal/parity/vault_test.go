package parity_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"sea-noise/internal/parity"
)

func TestLoadFixture_CopiesTreeIntoHermeticDir(t *testing.T) {
	src := t.TempDir()
	writeTree(t, src, map[string]string{
		"a.md":          "# A\n",
		"sub/b.md":      "# B\n[[A]]\n",
		"_meta/log.txt": "hello",
	})

	dst := parity.LoadFixture(t, src)

	if dst == src {
		t.Fatalf("LoadFixture returned the source dir: %s", dst)
	}
	for _, rel := range []string{"a.md", "sub/b.md", "_meta/log.txt"} {
		b, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil {
			t.Errorf("read %s in dst: %v", rel, err)
			continue
		}
		want, _ := os.ReadFile(filepath.Join(src, rel))
		if string(b) != string(want) {
			t.Errorf("%s: content drift", rel)
		}
	}
}

func TestLoadFixture_WritesToDestinationDoNotLeakIntoSource(t *testing.T) {
	src := t.TempDir()
	writeTree(t, src, map[string]string{"only.md": "original"})

	dst := parity.LoadFixture(t, src)
	if err := os.WriteFile(filepath.Join(dst, "only.md"), []byte("mutated"), 0o644); err != nil {
		t.Fatalf("write dst: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(src, "only.md"))
	if err != nil {
		t.Fatalf("read src: %v", err)
	}
	if string(b) != "original" {
		t.Fatalf("source mutated by destination write: got %q", string(b))
	}
}

func TestLoadFixtureInto_MissingSourceReturnsSentinel(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T) string // returns src path
	}{
		{
			name: "src does not exist",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nope")
			},
		},
		{
			name: "src is a regular file, not a directory",
			setup: func(t *testing.T) string {
				p := filepath.Join(t.TempDir(), "f.txt")
				if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
					t.Fatalf("seed file: %v", err)
				}
				return p
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := tc.setup(t)
			_, err := parity.LoadFixtureInto(src, t.TempDir())
			if !errors.Is(err, parity.ErrFixtureMissing) {
				t.Fatalf("err: got %v, want wrap of ErrFixtureMissing", err)
			}
		})
	}
}

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
}
