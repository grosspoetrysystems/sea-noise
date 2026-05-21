# v8 — Phase 4 Lookahead — SQLite-first retrieval engine

**Plan / Phase:** `plan_gjt5io5gp71d` / `phase4_sqlite_retrieval`
**Phase name:** Phase 4 — SQLite-first retrieval engine
**Status:** Lookahead (rebaselined into Phase 4 kickoff at Phase 3 closeout).

## Objective

Stand up the retrieval substrate Sea Noise needs to replace the Python RAG layer with no harness, no metric gates, and no MCP / ONNX / Ollama / dylib dependencies. Per `tempor/RETRIEVAL-RECOMMENDATION.md`: SQLite-first via `modernc.org/sqlite`, FTS5/BM25 over notes and chunks, derived structural tables (tags, entities, links, assertions, timestamps). Retrieval router with five modes (direct lookup, FTS, graph expansion, deterministic context builders, optional semantic). Embedder concrete impl decided at Phase-4 kickoff based on whether semantic layer materially helps.

## Scope (proposed)

| Wave | Track | Title |
|------|-------|-------|
| P4-W1 | T10 | SQLite-first retrieval engine |
| P4-W2 | T08 | AI workflows consume retrieval router outputs |

## Dependency order

```
(Phase 3 complete: router seam, Claude integration)
            │
            ▼
          T10 ──→ T08
```

T10 builds the retrieval engine; T08 rewires AI workflows from whatever ad-hoc retrieval they used in Phase 3 to call the new retrieval router.

## Scope sketches

### P4-W1 — SQLite-first retrieval engine (T10)

- `modernc.org/sqlite` as the default driver. Fallback to `github.com/ncruces/go-sqlite3` if modernc fails real-corpus FTS5/build checks.
- Schema build: derived SQL tables for notes, chunks, tags, entities, links/backlinks, assertions, timestamps. Schema migration plumbing.
- FTS5/BM25 over `chunks_fts` and `notes_fts`.
- Retrieval router with all five modes (`DirectLookup`, `FTSSearch`, `GraphExpansion`, `DeterministicContextBuild`, `OptionalSemanticExpansion`).
- Optional semantic layer behind `Embedder` interface (interface already exists from T04). Concrete impl decided at wave kickoff based on whether semantic materially improves retrieval quality on the fixture vault. If it doesn't, runtime ships without semantic; record decision in `KNOWN-DELTA.md`.
- Exact cosine in Go over `[]float32` if semantic ships; no ANN index initially.
- Parity tests: golden expected outputs for representative queries against the T09 fixture vault, captured deterministically from the Python prototype.

### P4-W2 — AI workflows consume retrieval (T08)

- Rewire the AI command wrappers from Phase 3 to consume the retrieval router instead of any ad-hoc per-command vault scanning.
- Update parity tests to verify retrieval-router-mediated outputs match the prototype.

## Entry criteria

- [ ] Phase 3 closeout passed.
- [ ] Inference router + Claude integration stable (AI commands working end-to-end).
- [ ] Fixture vault (T09) still pinned; expected retrieval outputs to be captured during P4-W1.

## Key unknowns

- **modernc.org/sqlite vs ncruces fallback**: verify real-corpus FTS5 behavior on the fixture vault during P4-W1 kickoff. If modernc fails, swap drivers (one-line `import` change since both expose `database/sql`).
- **Semantic layer ship decision**: include only if FTS5 + structural alone misses too many parity tests. Decision recorded in `KNOWN-DELTA.md`.
- **Schema migration strategy**: in-place or rebuild-from-vault on startup? Lean rebuild-from-vault (vault is source of truth, SQL index is derived; rebuild is cheap at personal-vault scale).
- **Chunking strategy**: paragraph-level, fixed-byte windows, or per-note no-chunking? Match the Python prototype's chunking shape.

## Drift signals

- Reaching for ANN (`sqlite-vec`, lance, etc.) — explicitly excluded by ARCHITECTURE.md §2.
- Adding CGO via a SQLite driver — must be pure-Go.
- Phase 4 introducing AI calls (drift back into Phase 3 territory). Retrieval is deterministic.
- Metric thresholds creeping into acceptance criteria — explicitly forbidden by ARCHITECTURE.md §11.

## Rebaseline notes (at promotion to kickoff)

Promote into `v8-phase4-kickoff.md` once Phase 3 closes. Rebaseline against:
- Final InferenceRequest/Decision shape from T06.
- Actual AI command wrappers and what retrieval they need (T07).
- Reference commit at Phase 3 closeout.
- Updated unknowns from Phase 3 outcomes.
