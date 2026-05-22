# Sea Noise — Architecture

Parity-first Go port of the Python "mind-palace" prototype: a local retrieval and context-assembly runtime over an Obsidian markdown vault. This document specifies architectural commitments and interface contracts. It does not specify implementation details — those are decided in subsequent waves.

## 1. Scope

This document is the architectural anchor for the rewrite. Every later phase has questions that resolve here.

In scope:
- Retrieval substrate decision and rationale
- Vault-derived SQL schema (tables and their purpose)
- Retrieval router interface (mode enumeration)
- Embedder interface (the optional semantic layer contract)
- Inference eval router seam (cross-reference; full contract lives elsewhere)
- Hard binary constraints
- Fallback chain
- Boundaries with external products in the suite

Out of scope:
- Implementation details (driver choice mechanics, package layout, file boundaries)
- Acceptance metrics on the running system (Recall@K, MRR, latency are explicitly excluded — see §11)
- Inference routing logic (decision rules, classifier internals)
- Provider transport mechanics (`claude -p` invocation patterns, error handling)
- UI / control surface (handled by a separate product in the suite — see §12)

## 2. Retrieval substrate

**Primary substrate: SQLite (`modernc.org/sqlite`) with FTS5/BM25.**

The rewrite is not centered on classical embedding-heavy RAG. It is a local retrieval and context-assembly runtime over a markdown vault. The substrate that fits that shape is SQLite with FTS5/BM25 for keyword retrieval, augmented by first-class structural signals derived from the vault.

Structural signals (each is a first-class SQL-derived feature, not a vector-space artifact):

| Signal | Source in vault | Role |
|--------|-----------------|------|
| Tags | Frontmatter and inline `#tag` | Filter and routing |
| Links / backlinks | Wikilink syntax `[[Note]]` | Graph expansion |
| Entities | Extracted by signal generation passes | Cross-note alignment |
| Assertions | Extracted statements with provenance | Cross-note alignment |
| Recency | File mtime, frontmatter dates | Temporal weighting |
| Note types | Frontmatter `type` field, conventions | Routing, retrieval mode selection |
| Frontmatter | YAML metadata | All of the above |

**Driver decision:**

- Default: `modernc.org/sqlite` (pure-Go, CGO-free)
- Fallback: `github.com/ncruces/go-sqlite3` if modernc fails real-corpus FTS5/build/runtime checks
- Decision rule: if modernc passes the corpus check, use it; otherwise switch to ncruces. This is a verification spike, not an architecture fork. Both drivers expose `database/sql`; the application code is driver-swappable.

**Semantic layer:**

The semantic layer is optional and lives behind an interface (see §5). Vectors are stored as a derived cache in SQLite. Exact cosine similarity is computed in Go over `[]float32`. There is no ANN-first design; personal-vault scale does not require it.

Explicit non-substrate choices (forbidden as foundational):

- `sqlite-vec` — pulls CGO and immature extension surface
- `modernc.org/sqlite/vec` — same hazard class
- LanceDB Go — external storage layer that defeats the substrate goal
- Bleve vector search / FAISS — wrong shape for this product
- Local ONNX runtime as a ship blocker
- Ollama as a required dependency
- Any standalone vector DB
- Any Python compatibility bridge for retrieval (see §8)

## 3. Vault-derived SQL schema

The schema is a derived index over the markdown vault. The vault remains the source of truth — Sea Noise never edits notes; it reads them. The SQL index is rebuildable from the vault state at any time.

Tables:

| Table | Purpose | Key columns |
|-------|---------|-------------|
| `notes` | One row per markdown note in the vault | `id`, `path`, `type`, `mtime`, `frontmatter_json` |
| `chunks` | Chunked note bodies for FTS5 retrieval at sub-note granularity | `id`, `note_id`, `ordinal`, `body` |
| `tags` | One row per (note, tag) pair | `note_id`, `tag` |
| `entities` | Extracted entities, linked to notes | `id`, `note_id`, `surface`, `canonical_form` |
| `links` | Outgoing wikilinks from notes | `from_note_id`, `to_note_path`, `to_note_id_nullable` |
| `assertions` | Extracted statements with provenance | `id`, `note_id`, `statement`, `confidence_nullable` |
| `timestamps` | Temporal signals (frontmatter dates, mtime) | `note_id`, `kind`, `value` |
| `chunks_fts` | FTS5 virtual table backing `chunks` | per FTS5 contract |
| `notes_fts` | FTS5 virtual table backing note titles + frontmatter text | per FTS5 contract |

Backlinks are a query, not a stored table — derived from `links` via the inverse direction.

Schema exact column types, indexing strategy, and migration plumbing are deferred to T10 (SQLite-first retrieval engine, Phase 4). This document fixes only the table set, the responsibility of each table, and the FTS5 boundary.

## 4. Retrieval router interface

The retrieval router is the single entry point for any caller that needs notes or chunks from the vault. It does not orchestrate; it selects a retrieval mode and dispatches.

```go
// Package: internal/retrieval
type Router interface {
    Retrieve(ctx context.Context, req RetrieveRequest) (RetrieveResponse, error)
}
```

`RetrieveRequest` carries the query, optional filters (tags, note types, time window), the requested mode (or `Auto`), and a `Limit`. `RetrieveResponse` carries result note IDs and chunk IDs, with mode-specific provenance fields.

**Modes:**

| Mode | Description | When used |
|------|-------------|-----------|
| `DirectLookup` | Resolve a note by path, ID, or unique title match | Caller already knows the target |
| `FTSSearch` | BM25 over chunks (and/or notes_fts) | Keyword / phrase queries |
| `GraphExpansion` | Walk wikilinks/backlinks from a seed note set | Related-notes, neighborhood queries |
| `DeterministicContextBuild` | Apply a workflow-specific context builder (e.g., daily catchup, weekly bloom, signal queue) | Caller has a workflow ID |
| `OptionalSemanticExpansion` | Cosine-rank from the embedded layer, if present | Conceptual / paraphrase queries |

The router resolves `Auto` by inspecting the request shape:
- Path or ID present → `DirectLookup`
- Workflow ID present → `DeterministicContextBuild`
- Seed note set present and no query string → `GraphExpansion`
- Query string present → `FTSSearch`, with optional `OptionalSemanticExpansion` reranking iff the semantic layer is enabled

The router never invokes an LLM. Routing decisions are deterministic and inspectable. The inference eval router is a separate seam (§6).

## 5. Embedder interface (optional semantic layer)

The semantic layer is optional. It exists behind an interface so the runtime can ship with no embeddings at all.

```go
// Package: internal/ai/embedder
type Embedder interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    Dimension() int
    ProviderID() string
}
```

Properties:
- Embeddings are stored as a derived cache in SQLite (`embeddings` table — schema deferred to T10).
- Exact cosine is computed in Go over `[]float32`. No ANN index in the initial ship.
- The first provider may be remote (e.g., an HTTP embedding API). Local embeddings are not a ship blocker. ONNX/dylib-heavy packaging is explicitly excluded.
- The semantic layer is included in the shipped runtime only if it materially improves retrieval quality on conceptual/paraphrase/related-note queries against the golden fixture vault, judged by inspection of expected-output diffs. If it does not, the runtime ships without it. The include/exclude decision is recorded in `KNOWN-DELTA.md` as text — never as metric thresholds.

The Embedder interface is defined in Phase 2 (T04). Concrete implementation is Phase 4 (T10).

## 6. Inference eval router seam

The **inference eval router** is a separate seam from the retrieval router. It does not retrieve anything; it decides how an AI command should dispatch an LLM inference. AI command wrappers obtain an inference decision from the router before invoking any provider transport.

```go
// Package: internal/ai/inferencerouter
type Router interface {
    Route(ctx context.Context, req InferenceRequest) (InferenceDecision, error)
}
```

This document fixes only the seam's structural properties. The full `InferenceRequest` / `InferenceDecision` shape is defined in the interface task (Phase 3) and committed there as the seam's canonical contract.

Structural properties of the seam:

- The router is **harness-agnostic**. It is not designed around any specific external routing product, present or anticipated. No imports, vocabulary, or interface choices in this package exist to anticipate replacement by an external system.
- The router is **deterministic Go decision logic**. It does not call an LLM to decide what LLM to call.
- Zero `exec.Command` in this package.
- Zero outbound `net/http` in this package.
- Zero provider SDK imports in this package.
- Zero string literals for model/provider IDs anywhere in this package (or in any caller). Provider/model IDs are resolved at the transport layer from config.

The first implementation is a local router (`LocalRouter`) returning sensible defaults. The transport that consumes its decision — `claude -p` invocation in the current shipped pipeline — lives in a separate package and is downstream of the routing call.

CI grep gate:
```
grep -r 'claude-3\|gpt-4\|gemini\|text-embedding' --include='*.go' \
    | grep -v -E '(config|tests?)/' \
    | wc -l                                        # must be 0
```

## 7. Hard binary constraints

The shipped binary obeys these constraints. Each is enforced by a CI gate.

| Constraint | Verification |
|-----------|--------------|
| No Python runtime in the shipped binary | No `python`/`python3` subprocess invocations in the runtime code path; CI grep gate over `internal/` and `cmd/` |
| `CGO_ENABLED=0` builds and runs | CI builds with `CGO_ENABLED=0 go build ./...`; failure blocks merge |
| Static single binary on macOS and Linux | `go build` produces a binary with no dynamic dylib links; CI inspects `otool -L` (macOS) / `ldd` (Linux) |
| No MCP requirement | No MCP server, no MCP transport imports; CI grep gate |
| No ONNX runtime as a ship blocker | No `onnxruntime` imports in retrieval/embedder paths |
| No Ollama dependency | No `ollama` references in retrieval/embedder paths |
| No vector DB external store | No `pinecone`, `weaviate`, `qdrant`, `lance`, `chroma` (other than chromem-go in-process exact-cosine, if used at all) imports |
| No `sqlite-vec` / `modernc.org/sqlite/vec` | CI grep gate; explicit exclusion list in CI config |
| No string literals for model/provider IDs outside config | CI grep gate per §6 |

The vault path is never hardcoded. It is resolved from `mind-palace.toml` (at repo root or the project's config directory) or the `MIND_PALACE_VAULT` environment variable, with explicit precedence documented at T02 (Phase 1).

## 8. Fallback chain

Retrieval modes fall back along a fixed chain when their preferred substrate is unavailable.

```
chromem / exact-cosine (semantic, optional)
        │  available iff Embedder is configured AND vault has cached embeddings
        ▼
FTS5 / BM25 (keyword, primary)
        │  available iff SQLite index is built
        ▼
disabled (caller receives an explicit "no retrieval substrate available" error)
```

The chain is descending: the optional semantic layer is the most powerful but most expensive; FTS5 is the default; disabled is the explicit no-substrate state. The runtime never silently degrades — if the semantic layer is unavailable mid-request, the router emits a structured warning and proceeds with FTS5.

## 9. Parity-first discipline

This rewrite is parity-first. Until parity against the Python prototype is demonstrated on the golden fixture vault, the rewrite does not sharpen, redesign, or simplify the prototype's behavior.

Authority precedence when sources conflict:
1. Current Python runtime behavior (the canonical specification)
2. Prompts in `references/prompts/`
3. Docs in `references/docs/`

Parity is judged behaviorally: Go output diffs zero against captured Python prototype output on the golden fixture vault. There are no metric thresholds (see §11). There is no harness — only `go test` invocations comparing committed expected outputs against current runtime output.

## 10. Boundaries with adjacent tooling

Sea Noise is a runtime. The HITL control surface (UI control plane for autonomous-agent oversight) is a separate forthcoming product — Sea Noise does not build UI, and approval / oversight / reporting-back primitives are not in this runtime.

The inference router seam (§6) is harness-agnostic: it is not designed to be replaced by any particular external system. If a future caller wants to consume the seam, the contract is what is defined in §6 — nothing in this architecture anticipates a specific replacement.

## 11. Excluded from acceptance language

Throughout the rewrite, acceptance criteria for the product itself never use:

- Recall@K (any K)
- MRR (Mean Reciprocal Rank)
- Precision, F1, nDCG, or any IR metric
- Latency thresholds (`p95 ≤ N ms`, etc.)
- Throughput thresholds
- Any quantitative gate

Parity is judged behaviorally on the fixture vault. Metric language belongs to evaluation tooling that is not built in this rewrite.

This is not a stylistic preference — it is an architectural commitment. If a metric threshold appears in a later acceptance criterion or doc, that is drift and must be corrected.

## 12. Authoritative references

For every architectural question that this document does not explicitly answer:

- **Parity behavior:** the Python prototype in `references/scripts/` is the canonical specification. Run it against the golden fixture vault to see what the Go port must reproduce.
- **Inference router contract:** the `internal/ai/inferencerouter` package defines `Router`, `InferenceRequest`, `InferenceDecision`, and the stub's behavior. This document fixes only the seam's structural properties (§6).

If a later doc or task acceptance criterion contradicts this document, this document is the lower-precedence source. The Python prototype's behavior outranks all docs. This doc is the durable architectural anchor for the rewrite — but where the Python prototype's behavior settles a question, the prototype wins.
