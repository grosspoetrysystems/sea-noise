package retrieval_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"sea-noise/internal/retrieval"
	"sea-noise/internal/vernacular"
)

func TestIndexVaultResolvesQualifiedAndLocalSourceLinks(t *testing.T) {
	root := t.TempDir()
	checkoutAlpha := t.TempDir()
	checkoutBeta := t.TempDir()
	alphaCanon := filepath.Join(checkoutAlpha, "canon")
	alphaApparatus := filepath.Join(checkoutAlpha, "apparatus")
	alphaWorking := filepath.Join(checkoutAlpha, "working")
	betaCanon := filepath.Join(checkoutBeta, "canon")
	betaApparatus := filepath.Join(checkoutBeta, "apparatus")
	betaWorking := filepath.Join(checkoutBeta, "working")
	for _, dir := range []string{alphaCanon, alphaApparatus, alphaWorking, betaCanon, betaApparatus, betaWorking} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("personal README\n[[README.md]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes", "from.md"), []byte("[[README.md]] [[unique]] [[vernacular/alpha/apparatus/detail.md]] [[vernacular/alpha/apparatus/overview.md]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(alphaApparatus, "overview.md"), []byte("[[detail]] [[README.md]] [[vernacular/beta/apparatus/detail.md]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(alphaApparatus, "detail.md"), []byte("alpha detail\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(alphaApparatus, "README.md"), []byte("alpha README\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(alphaApparatus, "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(alphaApparatus, "deep", "unique.md"), []byte("unique alpha note\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(betaApparatus, "overview.md"), []byte("beta overview\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(betaApparatus, "detail.md"), []byte("beta detail\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(betaApparatus, "README.md"), []byte("beta README\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	installMultiVernStub(t, map[string]map[string]string{
		checkoutAlpha: {"canon": alphaCanon, "apparatus": alphaApparatus, "working": alphaWorking},
		checkoutBeta:  {"canon": betaCanon, "apparatus": betaApparatus, "working": betaWorking},
	})
	if _, err := vernacular.Attach(context.Background(), root, "alpha", checkoutAlpha); err != nil {
		t.Fatal(err)
	}
	if _, err := vernacular.Attach(context.Background(), root, "beta", checkoutBeta); err != nil {
		t.Fatal(err)
	}

	db := openDB(t)
	if _, err := retrieval.IndexVault(context.Background(), db, root, nil); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`SELECT from_note_id, to_note_path, COALESCE(to_note_id, '') FROM links`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	links := make(map[string]string)
	for rows.Next() {
		var from, raw, to string
		if err := rows.Scan(&from, &raw, &to); err != nil {
			t.Fatal(err)
		}
		links[from+"\x00"+raw] = to
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	assertLink := func(from, raw, want string) {
		t.Helper()
		if got := links[from+"\x00"+raw]; got != want {
			t.Fatalf("link %s -> %s = %q, want %q", from, raw, got, want)
		}
	}
	assertLink("notes/from.md", "README.md", "")
	assertLink("notes/from.md", "unique", "vernacular/alpha/apparatus/deep/unique.md")
	assertLink("notes/from.md", "vernacular/alpha/apparatus/detail.md", "vernacular/alpha/apparatus/detail.md")
	assertLink("notes/from.md", "vernacular/alpha/apparatus/overview.md", "vernacular/alpha/overview.md")
	assertLink("vernacular/alpha/overview.md", "detail", "vernacular/alpha/apparatus/detail.md")
	assertLink("vernacular/alpha/overview.md", "README.md", "vernacular/alpha/apparatus/README.md")
	assertLink("vernacular/alpha/overview.md", "vernacular/beta/apparatus/detail.md", "vernacular/beta/apparatus/detail.md")
}

func installMultiVernStub(t *testing.T, targets map[string]map[string]string) {
	t.Helper()
	bin := t.TempDir()
	stub := filepath.Join(bin, "vern")
	contents := "#!/bin/sh\ncase \"$(pwd -P):$2\" in\n"
	for checkout, paths := range targets {
		canonicalCheckout, err := filepath.EvalSymlinks(checkout)
		if err != nil {
			t.Fatal(err)
		}
		contents += fmt.Sprintf("\"%s:canon\") printf '%%s\\n' \"%s\" ;;\n", canonicalCheckout, paths["canon"])
		contents += fmt.Sprintf("\"%s:apparatus\") printf '%%s\\n' \"%s\" ;;\n", canonicalCheckout, paths["apparatus"])
		contents += fmt.Sprintf("\"%s:working\") printf '%%s\\n' \"%s\" ;;\n", canonicalCheckout, paths["working"])
	}
	contents += "*) exit 1 ;;\nesac\n"
	if err := os.WriteFile(stub, []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}
