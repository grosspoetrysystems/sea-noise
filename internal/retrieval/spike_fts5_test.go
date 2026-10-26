package retrieval

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestFTS5Available is the FTS5 verification spike per ARCHITECTURE.md §2.
// If modernc.org/sqlite cannot create and query an FTS5 virtual table on
// this platform, T10 must fall back to github.com/ncruces/go-sqlite3.
func TestFTS5Available(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "spike.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE VIRTUAL TABLE docs USING fts5(title, body, tokenize = 'porter unicode61')`); err != nil {
		t.Fatalf("create fts5 table (driver does not support FTS5; fall back to ncruces): %v", err)
	}
	if _, err := db.Exec(`INSERT INTO docs(title, body) VALUES (?, ?), (?, ?), (?, ?)`,
		"Alpha", "the quick brown fox jumps",
		"Beta", "lazy dogs sleep in the sun",
		"Gamma", "quick foxes prefer brown leaves",
	); err != nil {
		t.Fatalf("insert: %v", err)
	}

	rows, err := db.Query(`SELECT title, bm25(docs) FROM docs WHERE docs MATCH ? ORDER BY rank LIMIT 3`, "quick brown")
	if err != nil {
		t.Fatalf("fts5 match query: %v", err)
	}
	defer rows.Close()

	var titles []string
	for rows.Next() {
		var title string
		var score float64
		if err := rows.Scan(&title, &score); err != nil {
			t.Fatalf("scan: %v", err)
		}
		titles = append(titles, title)
	}
	if len(titles) < 2 {
		t.Fatalf("expected ≥2 matches for 'quick brown', got %d: %v", len(titles), titles)
	}
	if titles[0] != "Alpha" && titles[0] != "Gamma" {
		t.Fatalf("expected Alpha or Gamma top hit, got %s", titles[0])
	}
	t.Logf("FTS5 spike OK on modernc.org/sqlite: top hits %v", titles)
}
