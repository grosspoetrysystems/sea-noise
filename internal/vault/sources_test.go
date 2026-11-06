package vault

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWalkSourcesUsesLogicalIDsAndDeduplicatesOverview(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ordinary.md"), []byte("ordinary"), 0o644); err != nil {
		t.Fatal(err)
	}
	wing := filepath.Join(root, "vernacular", "alpha")
	if err := os.MkdirAll(wing, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wing, "ignored.md"), []byte("not automatic input"), 0o644); err != nil {
		t.Fatal(err)
	}
	apparatus := filepath.Join(external, "apparatus")
	if err := os.MkdirAll(apparatus, 0o755); err != nil {
		t.Fatal(err)
	}
	overview := filepath.Join(apparatus, "overview.md")
	if err := os.WriteFile(overview, []byte("# original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(apparatus, "detail.md"), []byte("# detail"), 0o644); err != nil {
		t.Fatal(err)
	}

	notes, err := WalkSourcesContext(context.Background(), root, nil, []Source{
		{LogicalRoot: "vernacular/alpha/overview.md", PhysicalRoot: overview},
		{LogicalRoot: "vernacular/alpha/apparatus", PhysicalRoot: apparatus},
	})
	if err != nil {
		t.Fatal(err)
	}
	ids := make(map[string]Note, len(notes))
	for _, note := range notes {
		ids[note.ID] = note
	}
	sameFile := func(got, want string) bool {
		gotInfo, gotErr := os.Stat(got)
		wantInfo, wantErr := os.Stat(want)
		return gotErr == nil && wantErr == nil && os.SameFile(gotInfo, wantInfo)
	}
	if note, ok := ids["ordinary.md"]; !ok {
		t.Fatal("ordinary vault note missing")
	} else if !sameFile(note.PhysicalPath, filepath.Join(root, "ordinary.md")) {
		t.Fatalf("ordinary physical path = %q", note.PhysicalPath)
	}
	if _, ok := ids["vernacular/alpha/ignored.md"]; ok {
		t.Fatal("automatic root walk followed vernacular wing")
	}
	if note, ok := ids["vernacular/alpha/overview.md"]; !ok {
		t.Fatal("overview logical ID missing")
	} else if !sameFile(note.PhysicalPath, overview) {
		t.Fatalf("overview physical path = %q, want %q", note.PhysicalPath, overview)
	}
	if _, ok := ids["vernacular/alpha/apparatus/overview.md"]; ok {
		t.Fatal("overview was duplicated under directory source")
	}
	if _, ok := ids["vernacular/alpha/apparatus/detail.md"]; !ok {
		t.Fatal("apparatus detail missing")
	}
	if got, ok := ResolvePath(root, "vernacular/alpha/apparatus/detail.md", []Source{{LogicalRoot: "vernacular/alpha/apparatus", PhysicalRoot: apparatus}}); !ok || !sameFile(got, filepath.Join(apparatus, "detail.md")) {
		t.Fatalf("ResolvePath = %q, %v", got, ok)
	}
	if _, ok := ResolvePath(root, "../outside.md", nil); ok {
		t.Fatal("ResolvePath accepted path escape")
	}
}
func TestWalkSourcesRejectsSymmetricVaultOverlap(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "external")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Dir(root)
	for _, physical := range []string{inside, parent} {
		_, err := WalkSourcesContext(context.Background(), root, nil, []Source{{
			LogicalRoot:  "vernacular/alpha/apparatus",
			PhysicalRoot: physical,
		}})
		if !errors.Is(err, ErrSourcePath) {
			t.Fatalf("source %q overlap error = %v, want ErrSourcePath", physical, err)
		}
	}
}
