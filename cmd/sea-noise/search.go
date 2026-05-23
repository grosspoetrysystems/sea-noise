package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"

	"sea-noise/internal/retrieval"
)

// runSearch implements `sea-noise search --db PATH QUERY...`.
//
// Runs FTS5 retrieval against the SQLite index built by `sea-noise index`
// and prints ranked hits in the form:
//
//   <score>  <note-id>  <snippet>
//
// One line per hit. Snippet is FTS5's snippet() output around the matched
// terms with «…» as the highlight delimiters.
func runSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	dbPath := fs.String("db", "", "path to the SQLite index file (built by `sea-noise index`)")
	limit := fs.Int("limit", 20, "max hits to return")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage: sea-noise search --db PATH QUERY...

Run FTS5 keyword search against a previously-built retrieval index.

Flags:
  --db PATH      path to the SQLite index file (required)
  --limit N      max hits to return (default 20)`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dbPath == "" {
		fs.Usage()
		return fmt.Errorf("--db is required")
	}
	query := strings.Join(fs.Args(), " ")
	if strings.TrimSpace(query) == "" {
		fs.Usage()
		return fmt.Errorf("query is required")
	}

	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		return fmt.Errorf("open db %s: %w", *dbPath, err)
	}
	defer db.Close()

	router := retrieval.NewSQLiteRouter(db)
	resp, err := router.Retrieve(context.Background(), retrieval.RetrieveRequest{
		Query: query,
		Limit: *limit,
	})
	if err != nil {
		return err
	}
	if len(resp.Hits) == 0 {
		fmt.Println("(no hits)")
		return nil
	}
	for _, h := range resp.Hits {
		fmt.Printf("%7.3f  %s  %s\n", h.Score, h.NoteID, h.Snippet)
	}
	return nil
}
