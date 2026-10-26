// Package retrieval holds the SQLite-backed retrieval engine and the
// retrieval router that sits on top of it. See ARCHITECTURE.md §2–§4 and
// §8 for the substrate decision, vault-derived schema, router interface,
// and fallback chain.
//
// The shipped substrate is SQLite (modernc.org/sqlite, pure-Go, CGO-free)
// with FTS5/BM25 for keyword retrieval over notes and chunks, augmented
// by structural tables (tags, links, entities, assertions, timestamps).
// The vault remains the source of truth; the SQL index is rebuildable
// from the vault state at any time.
//
// Surface:
//
//   - Migrate(db) applies the schema (idempotent).
//   - IndexVault(db, root) walks the vault and populates the index.
//   - Router (interface) selects a retrieval mode and dispatches.
//   - SQLiteRouter is the concrete Router backed by *sql.DB.
//
// The optional semantic layer (Embedder, cosine ranking in Go) is
// defined in internal/ai/embedder and consumed here behind the
// OptionalSemanticExpansion mode. It is not wired in this wave.
package retrieval
