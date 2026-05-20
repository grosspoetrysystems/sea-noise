# Sea Noise Retrieval Recommendation for Plan Iteration

## Decision summary

Deep research converged on the same retrieval architecture from two independent long-form reports.

The rewrite should **not** center on classical embedding-heavy RAG.

The recommended shape is:

- **Go-native runtime**
- **SQLite-first retrieval substrate**
- **FTS5/BM25 as primary retrieval**
- **first-class structural signals**: frontmatter, tags, links/backlinks, note types, recency, entities, assertions
- **workflow-specific context builders**
- **optional semantic layer** behind an interface
- **no Python in shipped output**
- **no MCP requirement**
- **no CGO / dylib-heavy retrieval foundation**

This is best understood as a **local retrieval and context-assembly runtime over a Markdown vault**, not a generic vector RAG app.

## Primary recommendation

Ship:

- `modernc.org/sqlite`
- SQLite `FTS5` + `bm25()`
- derived SQL tables for:
  - notes
  - chunks
  - tags
  - entities
  - links / backlinks
  - assertions
  - timestamps / note types / source metadata
- application-level retrieval router:
  - direct lookup
  - FTS search
  - graph/link expansion
  - deterministic context builders
  - optional semantic expansion
- optional embeddings stored as a **derived cache** in SQLite
- semantic search as **exact brute-force cosine in Go** over `[]float32`
- application-level **RRF** only where hybrid retrieval is proven useful

Fallback:

- `github.com/ncruces/go-sqlite3` if `modernc` fails real-corpus FTS5/build/runtime checks

## Explicit non-recommendations

Do **not** make any of these foundational in this plan:

- Python compatibility bridge for retrieval
- `sqlite-vec`
- `modernc.org/sqlite/vec`
- LanceDB Go
- Bleve vector search / FAISS path
- local ONNX runtime as a ship blocker
- Ollama as a required dependency
- standalone vector DB
- generic embedding-first RAG architecture

## Why this is the right shape

1. **Parity-first does not require Python-component parity**
   For retrieval, parity should be judged by **workflow behavior on a golden fixture harness**, not by reproducing the Python embedding stack component-for-component.

2. **The vault already has structure**
   Wikilinks, backlinks, tags, frontmatter, note types, weekly notes, assertions, and recency are stronger signals than embeddings for many real product surfaces.

3. **CGO is the real macOS risk**
   The biggest distribution hazard is not Go itself; it is any retrieval stack that drags in CGO, ONNX runtimes, native dylibs, or daemon/process dependencies.

4. **Personal-vault scale does not require ANN-first design**
   Exact cosine in Go is practical at this scale and keeps the system simpler and more deterministic than immature vector extensions or external stores.

## What changes in the plan

### Replace the current RAG framing

Do not frame T09 as “RAG replacement”.

Replace it with something like:

- **SQLite-first retrieval engine**
- **FTS5 + structural retrieval + optional semantic layer**
- **no Python in shipped output**

### Recommended plan shape for retrieval work

1. **Golden fixture + eval harness**
   Build first. This is the gating dependency.
   - ~200 representative notes across real note types
   - queries tagged by type:
     - exact keyword
     - phrase
     - title/path/entity/tag
     - paraphrase/conceptual
     - related-note
     - synthesis/research-pack
   - metrics:
     - Recall@1/5/10
     - MRR
     - latency
     - broken out by query type

2. **Driver-swappable FTS5 + metadata/link engine**
   - implement once against `database/sql`
   - start with `modernc`
   - verify `ncruces` as fallback only
   - modernc vs ncruces is a verification spike, not an architecture fork

3. **Semantic layer behind an interface**
   - optional
   - vectors stored in SQLite
   - exact cosine in Go
   - provider interface for embeddings
   - only retained if it materially improves the conceptual / paraphrase / related-note subset

## Planning implications

### T09 should be rewritten to something like

- Build SQLite-derived retrieval index over the vault
- Implement FTS5/BM25 note/chunk retrieval
- Implement metadata/link/backlink/entity/assertion indexes
- Implement retrieval router and deterministic context builders
- Add optional exact-cosine semantic retrieval behind an embedding provider interface
- Prove parity against the Python reference on the golden fixture harness
- Ship with no Python runtime dependency

### T09 acceptance criteria

- No Python subprocess dependency remains for retained retrieval surfaces
- FTS-only and deterministic retrieval surfaces meet parity targets on the fixture harness
- Semantic layer is only included if it improves the targeted conceptual query subset enough to justify itself
- Retrieval remains:
  - local-first
  - single-binary-compatible
  - CGO-free
  - MCP-free

## Driver decision

Default:
- `modernc.org/sqlite`

Fallback:
- `github.com/ncruces/go-sqlite3`

Decision rule:
- if `modernc` passes FTS5 behavior, build, and runtime checks on the real corpus, use it
- if it fails, fall back to `ncruces`
- do not turn this into a broad driver bake-off

## Embedding decision

Embedding strategy should be **staged and optional**.

- first shipped semantic provider may be remote
- local embeddings should not block the rewrite
- local ONNX / dylib-heavy packaging is specifically discouraged for this plan
- if embeddings do not materially improve the right query subset, ship without them

## Open questions now narrowed

1. Does `modernc` pass real-corpus FTS5 behavior cleanly?
2. Does the semantic layer materially improve the conceptual / paraphrase / related-note subset?
3. Is exact cosine already fast enough at expected vault scale?
   - likely yes
   - if no, consider a pure-Go ANN path later
   - do not jump to `sqlite-vec`

## Final recommendation

Revise the plan so retrieval is built as:

**SQLite FTS5 + structural SQL indexes + workflow-specific context assembly first**
with
**optional exact-cosine semantic retrieval second**
and
**no Python in shipped output**.

This keeps the Sea Noise rewrite aligned with its actual goal:
a durable, coherent Go runtime over a Markdown vault, not a more elegant version of the glued prototype.
