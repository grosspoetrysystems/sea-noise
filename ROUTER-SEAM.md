# Router seam

The retrieval router under `internal/retrieval/` is for **new surfaces only**. Prototype-derived workflows do not consume it.

## What lives behind the seam

`internal/retrieval/` is a SQLite-backed engine (modernc.org/sqlite, FTS5 / BM25, graph expansion, FTS filters). It exposes a `Router` type with indexing + search modes.

CLI surfaces:

- `sea-noise index` — builds / refreshes the SQLite index from the vault
- `sea-noise search` — queries the index

## What does **not** consume the router

Every workflow ported from the Python prototype scans the vault directly via `internal/vault.Walk` plus substring and regex matching. That includes (current state):

| Workflow | Method |
|----------|--------|
| `audit-links` | `vault.Walk` + wikilink regex (see `internal/auditlinks/`) |
| `generate-signal` | `vault.Walk` + frontmatter parsing (see `internal/signal/`) |
| `summarize-transcripts` | Single-note read (see `internal/summarizer/`) |
| `profile-entities` | `vault.Walk` + substring match (see `internal/entityprofile/`) |
| `enrich-topics` | `vault.Walk` + tag-substring match (see `internal/enrichtopics/`) |

Future ports of remaining prototype scripts follow the same convention.

## Why the boundary exists

BM25 tokenization does not replicate Python substring matching. Wiring the router into a prototype-derived workflow would change the candidate set the workflow operates on, and behavioral diff against the golden fixture would fail. Parity is the gate. The router is for surfaces the prototype never had (`index`, `search`, and future retrieval-shaped surfaces).

## What this means for future contributors

- Adding a new retrieval-shaped surface? Use the router.
- Porting another prototype script? Use `vault.Walk` + substring/regex. Match what the Python source does, byte-for-byte where the goldens enforce it.
- Tempted to "modernize" a prototype-derived workflow to use the router? Don't — without re-capturing every golden, the workflow diverges silently from the prototype.

This boundary is codified in agent-facing memory (`feedback_retrieval_router_boundary.md`) and is the architectural decision behind v10's T08 closing as seam-only.
