package vernacular

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sea-noise/internal/vault"
)

func TestAttachExposesLiveOverviewAndDetachPreservesOriginals(t *testing.T) {
	root := t.TempDir()
	checkout := t.TempDir()
	canon := filepath.Join(checkout, "canon")
	apparatus := filepath.Join(checkout, "apparatus")
	working := filepath.Join(checkout, "working")
	for _, dir := range []string{canon, apparatus, working} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	installVernStub(t, canon, apparatus, working)

	project, err := Attach(context.Background(), root, "alpha", checkout)
	if err != nil {
		t.Fatal(err)
	}
	if project.Overview != "vernacular/alpha/overview.md" {
		t.Fatalf("overview = %q", project.Overview)
	}
	original := filepath.Join(apparatus, "overview.md")
	if err := os.WriteFile(original, []byte("# edited original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	throughBridge, err := os.ReadFile(filepath.Join(root, "vernacular", "alpha", "overview.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(throughBridge) != "# edited original\n" {
		t.Fatalf("bridge did not expose original edit: %q", throughBridge)
	}

	if _, err := Attach(context.Background(), root, "alpha", checkout); err != nil {
		t.Fatalf("identical attach: %v", err)
	}
	sources, err := Sources(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := vault.WalkSourcesContext(context.Background(), root, nil, sources)
	if err != nil {
		t.Fatal(err)
	}
	sameFile := func(got, want string) bool {
		gotInfo, gotErr := os.Stat(got)
		wantInfo, wantErr := os.Stat(want)
		return gotErr == nil && wantErr == nil && os.SameFile(gotInfo, wantInfo)
	}
	found := false
	for _, note := range notes {
		if note.ID == project.Overview {
			found = true
			if !sameFile(note.PhysicalPath, original) {
				t.Fatalf("overview physical path = %q, want %q", note.PhysicalPath, original)
			}
		}
	}
	if !found {
		t.Fatalf("overview %q not indexed", project.Overview)
	}

	entry := filepath.Join(root, "vernacular", "alpha")
	wingReadme := filepath.Join(root, "vernacular", "README.md")
	if info, err := os.Lstat(wingReadme); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("wing README is not a regular file: %v", err)
	}
	entryReadme := filepath.Join(entry, "README.md")
	entryContents, err := os.ReadFile(entryReadme)
	if err != nil {
		t.Fatalf("entry README: %v", err)
	}
	if !strings.Contains(string(entryContents), "[[vernacular/alpha/overview.md]]") {
		t.Fatalf("entry README lacks qualified overview link: %q", entryContents)
	}
	userFile := filepath.Join(entry, "user-note.md")
	if err := os.WriteFile(userFile, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Detach(root, "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(original); err != nil {
		t.Fatalf("original overview removed: %v", err)
	}
	if _, err := os.Stat(userFile); err != nil {
		t.Fatalf("user file removed: %v", err)
	}
	if _, err := os.Stat(entryReadme); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unchanged generated README remained or failed to stat: %v", err)
	}
	if _, err := Sources(context.Background(), root); err != nil {
		t.Fatalf("sources after detach: %v", err)
	}
}

func TestAttachReusesPrivateReadmeOverview(t *testing.T) {
	root := t.TempDir()
	checkout := t.TempDir()
	canon := filepath.Join(checkout, "canon")
	apparatus := filepath.Join(checkout, "apparatus")
	working := filepath.Join(checkout, "working")
	for _, dir := range []string{canon, apparatus, working} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	original := filepath.Join(apparatus, "README.md")
	const content = "# Existing private project\n"
	if err := os.WriteFile(original, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	installVernStub(t, canon, apparatus, working)

	project, err := Attach(context.Background(), root, "alpha", checkout)
	if err != nil {
		t.Fatal(err)
	}
	if project.Overview != "vernacular/alpha/overview.md" {
		t.Fatalf("overview = %q", project.Overview)
	}
	if got, err := os.ReadFile(original); err != nil || string(got) != content {
		t.Fatalf("private README changed: %q, %v", got, err)
	}
	if _, err := os.Lstat(filepath.Join(apparatus, "overview.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("generated duplicate overview exists or failed to stat: %v", err)
	}
	bridgeOverview := filepath.Join(root, "vernacular", "alpha", "overview.md")
	gotInfo, gotErr := os.Stat(bridgeOverview)
	wantInfo, wantErr := os.Stat(original)
	if gotErr != nil || wantErr != nil || !os.SameFile(gotInfo, wantInfo) {
		t.Fatalf("bridge overview does not point to private README: %v, %v", gotErr, wantErr)
	}
}

func TestDetachPreservesEditedNavigationAndWingReadme(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "vernacular"), 0o755); err != nil {
		t.Fatal(err)
	}
	wingReadme := filepath.Join(root, "vernacular", "README.md")
	if err := os.WriteFile(wingReadme, []byte("user wing notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	checkout := t.TempDir()
	canon := filepath.Join(checkout, "canon")
	apparatus := filepath.Join(checkout, "apparatus")
	working := filepath.Join(checkout, "working")
	for _, dir := range []string{canon, apparatus, working} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	installVernStub(t, canon, apparatus, working)
	if _, err := Attach(context.Background(), root, "alpha", checkout); err != nil {
		t.Fatal(err)
	}
	entryReadme := filepath.Join(root, "vernacular", "alpha", "README.md")
	if err := os.WriteFile(entryReadme, []byte("user edited navigation\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Detach(root, "alpha"); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(wingReadme); err != nil || string(got) != "user wing notes\n" {
		t.Fatalf("wing README changed: %q, %v", got, err)
	}
	if got, err := os.ReadFile(entryReadme); err != nil || string(got) != "user edited navigation\n" {
		t.Fatalf("edited entry README changed: %q, %v", got, err)
	}
}

func TestDetachRefusesReplacedLink(t *testing.T) {
	root := t.TempDir()
	checkout := t.TempDir()
	canon := filepath.Join(checkout, "canon")
	apparatus := filepath.Join(checkout, "apparatus")
	working := filepath.Join(checkout, "working")
	for _, dir := range []string{canon, apparatus, working} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	installVernStub(t, canon, apparatus, working)
	if _, err := Attach(context.Background(), root, "alpha", checkout); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "vernacular", "alpha", "working")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), link); err != nil {
		t.Fatal(err)
	}
	if err := Detach(root, "alpha"); !errors.Is(err, ErrOwnership) {
		t.Fatalf("detach error = %v, want ErrOwnership", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "vernacular", "alpha", "apparatus")); err != nil {
		t.Fatalf("detach removed links after mismatch: %v", err)
	}
}
func TestOverviewAndProjectEntrySymlinkBoundaries(t *testing.T) {
	root := t.TempDir()
	checkout := t.TempDir()
	canon := filepath.Join(checkout, "canon")
	apparatus := filepath.Join(checkout, "apparatus")
	working := filepath.Join(checkout, "working")
	for _, dir := range []string{canon, apparatus, working} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("private outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(apparatus, "overview.md")); err != nil {
		t.Fatal(err)
	}
	installVernStub(t, canon, apparatus, working)
	if _, err := Attach(context.Background(), root, "alpha", checkout); !errors.Is(err, ErrDestinationConflict) {
		t.Fatalf("symlink overview attach error = %v, want destination conflict", err)
	}
	if got, err := os.ReadFile(outside); err != nil || string(got) != "private outside\n" {
		t.Fatalf("outside overview changed: %q, %v", got, err)
	}

	if err := os.Remove(filepath.Join(apparatus, "overview.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := Attach(context.Background(), root, "alpha", checkout); err != nil {
		t.Fatal(err)
	}
	overview := filepath.Join(apparatus, "overview.md")
	if err := os.Remove(overview); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, overview); err != nil {
		t.Fatal(err)
	}
	if _, err := Sources(context.Background(), root); !errors.Is(err, ErrStaleProject) {
		t.Fatalf("replaced overview Sources error = %v, want stale project", err)
	}
	if got, err := os.ReadFile(outside); err != nil || string(got) != "private outside\n" {
		t.Fatalf("outside overview changed after replacement: %q, %v", got, err)
	}

	entry := filepath.Join(root, "vernacular", "alpha")
	if err := os.RemoveAll(entry); err != nil {
		t.Fatal(err)
	}
	externalEntry := t.TempDir()
	sentinel := filepath.Join(externalEntry, "keep.md")
	if err := os.WriteFile(sentinel, []byte("do not delete\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalEntry, entry); err != nil {
		t.Fatal(err)
	}
	if err := Detach(root, "alpha"); !errors.Is(err, ErrOwnership) {
		t.Fatalf("symlink project entry detach error = %v, want ownership", err)
	}
	if got, err := os.ReadFile(sentinel); err != nil || string(got) != "do not delete\n" {
		t.Fatalf("external project-entry file changed: %q, %v", got, err)
	}
}

func TestAttachRejectsTargetContainingVault(t *testing.T) {
	root := t.TempDir()
	checkout := t.TempDir()
	canon := filepath.Join(checkout, "canon")
	working := filepath.Join(checkout, "working")
	if err := os.MkdirAll(canon, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(working, 0o755); err != nil {
		t.Fatal(err)
	}
	installVernStub(t, canon, filepath.Dir(root), working)
	if _, err := Attach(context.Background(), root, "alpha", checkout); !errors.Is(err, ErrTargetConflict) {
		t.Fatalf("target containing vault error = %v, want target conflict", err)
	}
}

func installVernStub(t *testing.T, canon, apparatus, working string) {
	t.Helper()
	bin := t.TempDir()
	stub := filepath.Join(bin, "vern")
	contents := "#!/bin/sh\ncase \"$2\" in\ncanon) printf '%s\\n' \"" + canon + "\" ;;\napparatus) printf '%s\\n' \"" + apparatus + "\" ;;\nworking) printf '%s\\n' \"" + working + "\" ;;\n*) exit 1 ;;\nesac\n"
	if err := os.WriteFile(stub, []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}
