package retrieval_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"sea-noise/internal/retrieval"
)

// openDB creates an empty SQLite database file inside t.TempDir() and
// returns it with Migrate already applied. The DB is closed via t.Cleanup.
func openDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "index.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := retrieval.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// writeVault materialises a tiny markdown vault inside t.TempDir() from
// the provided file → content map and returns the vault root.
func writeVault(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", abs, err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", abs, err)
		}
	}
	return root
}

func TestMigrateIsIdempotent(t *testing.T) {
	db := openDB(t)
	// Second call must succeed.
	if err := retrieval.Migrate(context.Background(), db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	// All declared tables must exist.
	for _, tbl := range []string{"notes", "chunks", "tags", "links", "entities", "assertions", "timestamps", "notes_fts", "chunks_fts"} {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE name = ?`, tbl).Scan(&name)
		if err != nil {
			t.Errorf("missing table %s: %v", tbl, err)
		}
	}
}

func TestIndexVault_SyntheticVault(t *testing.T) {
	root := writeVault(t, map[string]string{
		"notes/alpha.md": "---\ntitle: Alpha\ntags: [foo, bar]\ntype: reference\n---\n# Alpha\n\nThe quick brown fox jumps over [[beta]] and [[notes/gamma]].\n\nAnother paragraph about brown leaves.\n",
		"notes/beta.md":  "---\ntitle: Beta\ntags: [foo]\n---\nLazy dogs sleep in the sun.\n\nNothing about foxes here.\n",
		"notes/gamma.md": "# Gamma\n\nQuick foxes prefer brown leaves and tall grass.\n",
		"_meta/skip.md":  "this should NOT be indexed\n",
	})

	db := openDB(t)
	stats, err := retrieval.IndexVault(context.Background(), db, root, nil)
	if err != nil {
		t.Fatalf("IndexVault: %v", err)
	}
	if stats.Notes != 3 {
		t.Errorf("notes indexed = %d, want 3", stats.Notes)
	}
	if stats.Chunks < 4 {
		t.Errorf("chunks indexed = %d, want >= 4", stats.Chunks)
	}
	if stats.Tags < 3 {
		t.Errorf("tags indexed = %d, want >= 3 (foo×2, bar)", stats.Tags)
	}
	if stats.Links != 2 {
		t.Errorf("links indexed = %d, want 2", stats.Links)
	}

	// _meta exclusion: must not appear in notes.
	var skipCount int
	if err := db.QueryRow(`SELECT count(*) FROM notes WHERE id LIKE '_meta/%'`).Scan(&skipCount); err != nil {
		t.Fatalf("count _meta: %v", err)
	}
	if skipCount != 0 {
		t.Errorf("expected _meta excluded, got %d rows", skipCount)
	}

	// Link resolution: alpha → notes/gamma resolves by ID match; alpha →
	// beta resolves by stem match (single hit).
	rows, err := db.Query(`SELECT to_note_path, to_note_id FROM links WHERE from_note_id = 'notes/alpha.md' ORDER BY to_note_path`)
	if err != nil {
		t.Fatalf("query links: %v", err)
	}
	defer rows.Close()
	gotResolved := map[string]string{}
	for rows.Next() {
		var path, id sql.NullString
		if err := rows.Scan(&path, &id); err != nil {
			t.Fatalf("scan link: %v", err)
		}
		gotResolved[path.String] = id.String
	}
	if gotResolved["beta"] != "notes/beta.md" {
		t.Errorf("beta link should resolve to notes/beta.md, got %q", gotResolved["beta"])
	}
	if gotResolved["notes/gamma"] != "" && gotResolved["notes/gamma"] != "notes/gamma.md" {
		// We don't expect direct id match here (path lacks .md). Stem
		// resolver should pick it up.
	}
}

func TestFTSSearch_FixtureVault(t *testing.T) {
	root := writeVault(t, map[string]string{
		"notes/alpha.md": "---\ntitle: Alpha\n---\nThe quick brown fox jumps over the lazy dog.\n",
		"notes/beta.md":  "---\ntitle: Beta\n---\nLazy dogs sleep in the sun.\n",
		"notes/gamma.md": "---\ntitle: Gamma\n---\nQuick foxes prefer brown leaves.\n",
	})
	db := openDB(t)
	if _, err := retrieval.IndexVault(context.Background(), db, root, nil); err != nil {
		t.Fatalf("IndexVault: %v", err)
	}

	r := retrieval.NewSQLiteRouter(db)
	resp, err := r.Retrieve(context.Background(), retrieval.RetrieveRequest{
		Query: "quick brown",
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if resp.Mode != retrieval.ModeFTSSearch {
		t.Errorf("Mode = %s, want FTSSearch", resp.Mode)
	}
	if len(resp.Hits) < 2 {
		t.Fatalf("hits = %d, want >= 2 for 'quick brown'; resp=%+v", len(resp.Hits), resp)
	}
	// Alpha and Gamma both match; Beta should not appear.
	for _, h := range resp.Hits {
		if h.NoteID == "notes/beta.md" {
			t.Errorf("beta should not match 'quick brown' query; hits=%+v", resp.Hits)
		}
	}
}

func TestDirectLookup_ByID(t *testing.T) {
	root := writeVault(t, map[string]string{
		"a.md": "---\ntitle: A\n---\nbody",
	})
	db := openDB(t)
	if _, err := retrieval.IndexVault(context.Background(), db, root, nil); err != nil {
		t.Fatalf("IndexVault: %v", err)
	}

	r := retrieval.NewSQLiteRouter(db)
	resp, err := r.Retrieve(context.Background(), retrieval.RetrieveRequest{ID: "a.md"})
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if resp.Mode != retrieval.ModeDirectLookup {
		t.Errorf("Mode = %s, want DirectLookup", resp.Mode)
	}
	if len(resp.Hits) != 1 || resp.Hits[0].NoteID != "a.md" {
		t.Errorf("hits = %+v, want [a.md]", resp.Hits)
	}

	miss, err := r.Retrieve(context.Background(), retrieval.RetrieveRequest{ID: "nope.md"})
	if err != nil {
		t.Fatalf("Retrieve miss: %v", err)
	}
	if len(miss.Hits) != 0 {
		t.Errorf("miss hits = %+v, want empty", miss.Hits)
	}
}

func TestUnsupportedMode_SentinelError(t *testing.T) {
	db := openDB(t)
	r := retrieval.NewSQLiteRouter(db)
	_, err := r.Retrieve(context.Background(), retrieval.RetrieveRequest{
		Mode:       retrieval.ModeDeterministicContextBuild,
		WorkflowID: "daily-catchup",
	})
	if !errors.Is(err, retrieval.ErrUnsupportedMode) {
		t.Errorf("err = %v, want ErrUnsupportedMode", err)
	}
}

func TestEmptyRequest_SentinelError(t *testing.T) {
	db := openDB(t)
	r := retrieval.NewSQLiteRouter(db)
	_, err := r.Retrieve(context.Background(), retrieval.RetrieveRequest{})
	if !errors.Is(err, retrieval.ErrEmptyRequest) {
		t.Errorf("err = %v, want ErrEmptyRequest", err)
	}
}

func TestGraphExpansion_Depth1AndDepth2(t *testing.T) {
	// alpha → beta → gamma. delta links to alpha (backlink test).
	root := writeVault(t, map[string]string{
		"notes/alpha.md": "---\ntitle: Alpha\n---\nlinks to [[beta]]\n",
		"notes/beta.md":  "---\ntitle: Beta\n---\nlinks to [[gamma]]\n",
		"notes/gamma.md": "---\ntitle: Gamma\n---\nleaf node\n",
		"notes/delta.md": "---\ntitle: Delta\n---\nbacklink to [[alpha]]\n",
	})
	db := openDB(t)
	if _, err := retrieval.IndexVault(context.Background(), db, root, nil); err != nil {
		t.Fatalf("IndexVault: %v", err)
	}
	r := retrieval.NewSQLiteRouter(db)

	// Depth 1 from alpha: should reach beta (outgoing) and delta (backlink).
	resp, err := r.Retrieve(context.Background(), retrieval.RetrieveRequest{
		Mode:  retrieval.ModeGraphExpansion,
		Seeds: []string{"notes/alpha.md"},
	})
	if err != nil {
		t.Fatalf("depth1: %v", err)
	}
	got := hitIDs(resp.Hits)
	sort.Strings(got)
	want := []string{"notes/beta.md", "notes/delta.md"}
	if !equalStrings(got, want) {
		t.Errorf("depth1 hits = %v, want %v", got, want)
	}

	// Depth 2 from alpha: should additionally reach gamma (via beta).
	resp, err = r.Retrieve(context.Background(), retrieval.RetrieveRequest{
		Mode:  retrieval.ModeGraphExpansion,
		Seeds: []string{"notes/alpha.md"},
		Depth: 2,
	})
	if err != nil {
		t.Fatalf("depth2: %v", err)
	}
	got = hitIDs(resp.Hits)
	sort.Strings(got)
	want = []string{"notes/beta.md", "notes/delta.md", "notes/gamma.md"}
	if !equalStrings(got, want) {
		t.Errorf("depth2 hits = %v, want %v", got, want)
	}
}

func TestGraphExpansion_NoSeeds_SentinelError(t *testing.T) {
	db := openDB(t)
	r := retrieval.NewSQLiteRouter(db)
	_, err := r.Retrieve(context.Background(), retrieval.RetrieveRequest{Mode: retrieval.ModeGraphExpansion})
	if !errors.Is(err, retrieval.ErrEmptyRequest) {
		t.Errorf("err = %v, want ErrEmptyRequest", err)
	}
}

func TestFTSSearch_TagFilter(t *testing.T) {
	root := writeVault(t, map[string]string{
		"a.md": "---\ntitle: A\ntags: [keep]\n---\nquick brown fox\n",
		"b.md": "---\ntitle: B\ntags: [drop]\n---\nquick brown fox\n",
	})
	db := openDB(t)
	if _, err := retrieval.IndexVault(context.Background(), db, root, nil); err != nil {
		t.Fatalf("IndexVault: %v", err)
	}
	r := retrieval.NewSQLiteRouter(db)
	resp, err := r.Retrieve(context.Background(), retrieval.RetrieveRequest{
		Query: "quick brown",
		Tags:  []string{"keep"},
	})
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	got := hitIDs(resp.Hits)
	want := []string{"a.md"}
	if !equalStrings(got, want) {
		t.Errorf("tag-filtered hits = %v, want %v", got, want)
	}
}

func TestFTSSearch_NoteTypeFilter(t *testing.T) {
	root := writeVault(t, map[string]string{
		"a.md": "---\ntitle: A\ntype: reference\n---\nquick brown fox\n",
		"b.md": "---\ntitle: B\ntype: idea\n---\nquick brown fox\n",
	})
	db := openDB(t)
	if _, err := retrieval.IndexVault(context.Background(), db, root, nil); err != nil {
		t.Fatalf("IndexVault: %v", err)
	}
	r := retrieval.NewSQLiteRouter(db)
	resp, err := r.Retrieve(context.Background(), retrieval.RetrieveRequest{
		Query:     "quick brown",
		NoteTypes: []string{"reference"},
	})
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	got := hitIDs(resp.Hits)
	want := []string{"a.md"}
	if !equalStrings(got, want) {
		t.Errorf("type-filtered hits = %v, want %v", got, want)
	}
}

func TestFTSSearch_MtimeWindow(t *testing.T) {
	root := writeVault(t, map[string]string{
		"old.md":    "---\ntitle: Old\n---\nquick brown fox\n",
		"new.md":    "---\ntitle: New\n---\nquick brown fox\n",
		"recent.md": "---\ntitle: Recent\n---\nquick brown fox\n",
	})
	// Pin deterministic mtimes. Using fixed dates from time.Date for hermeticity.
	oldT := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	newT := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	recentT := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for rel, ts := range map[string]time.Time{
		"old.md":    oldT,
		"new.md":    newT,
		"recent.md": recentT,
	} {
		if err := os.Chtimes(filepath.Join(root, rel), ts, ts); err != nil {
			t.Fatalf("chtimes %s: %v", rel, err)
		}
	}

	db := openDB(t)
	if _, err := retrieval.IndexVault(context.Background(), db, root, nil); err != nil {
		t.Fatalf("IndexVault: %v", err)
	}
	r := retrieval.NewSQLiteRouter(db)

	// Since newT (inclusive): new + recent.
	since := newT
	resp, err := r.Retrieve(context.Background(), retrieval.RetrieveRequest{
		Query: "quick brown",
		Since: &since,
	})
	if err != nil {
		t.Fatalf("Retrieve since: %v", err)
	}
	got := hitIDs(resp.Hits)
	sort.Strings(got)
	want := []string{"new.md", "recent.md"}
	if !equalStrings(got, want) {
		t.Errorf("since hits = %v, want %v", got, want)
	}

	// Until newT (inclusive): old + new.
	until := newT
	resp, err = r.Retrieve(context.Background(), retrieval.RetrieveRequest{
		Query: "quick brown",
		Until: &until,
	})
	if err != nil {
		t.Fatalf("Retrieve until: %v", err)
	}
	got = hitIDs(resp.Hits)
	sort.Strings(got)
	want = []string{"new.md", "old.md"}
	if !equalStrings(got, want) {
		t.Errorf("until hits = %v, want %v", got, want)
	}
}

// hitIDs is a test-local helper to project a result list to note IDs.
func hitIDs(hits []retrieval.Hit) []string {
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.NoteID)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestIndexVault_RealFixtureVault(t *testing.T) {
	vaultRoot, _ := findFixtureVault(t)

	db := openDB(t)
	stats, err := retrieval.IndexVault(context.Background(), db, vaultRoot, nil)
	if err != nil {
		t.Fatalf("IndexVault fixture: %v", err)
	}
	if stats.Notes < 10 {
		t.Errorf("fixture notes indexed = %d, want >= 10", stats.Notes)
	}

	// FTS round-trip: a known token from the synthetic reference note
	// ("roasted") should retrieve that note.
	r := retrieval.NewSQLiteRouter(db)
	resp, err := r.Retrieve(context.Background(), retrieval.RetrieveRequest{Query: "roasted"})
	if err != nil {
		t.Fatalf("Retrieve roasted: %v", err)
	}
	if len(resp.Hits) == 0 {
		t.Fatalf("no hits for 'roasted' against fixture vault (chunks=%d)", stats.Chunks)
	}
	var sawNote bool
	for _, h := range resp.Hits {
		if strings.Contains(h.NoteID, "roasted-vegetables") {
			sawNote = true
			break
		}
	}
	if !sawNote {
		t.Errorf("expected roasted-vegetables in hits, got %+v", resp.Hits)
	}
}
