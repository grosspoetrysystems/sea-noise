# Post-v0 — Embeddings, dimensionality, and the semantic question

*Companion to the v0 retrieval stack recommendation and to "Post-v0 — Reranking, signal weighting, and vault shaping." This document specifies the embeddings half of post-v0 development: when they earn their keep, how to ship them, how to measure them, and when they should give way to alternatives. Shares the eval harness from v0 as the gating mechanism.*

---

## Bottom line

Embeddings remain a **feature flag, not a foundation**. The v0 stack ships useful with zero embedding infrastructure. Embeddings enter when the eval fixture shows specific workflows (find-related-to-this-note, fuzzy paraphrase lookup, semantic synthesis) where BM25 + heuristics + reranking demonstrably plateau on the conceptual-query subset.

When they enter, the shape is:

- **Note-level embeddings by default**, chunked only for genuinely long documents (transcripts, long-form articles)
- **Exact brute-force cosine in Go** over `[]float32` — never sqlite-vec at this scale
- **Remote-first via an `Embedder` interface** (Voyage `voyage-3.5-lite` or OpenAI `text-embedding-3-small`), with Ollama as the privacy opt-in
- **Matryoshka-truncated to 512 dimensions** for storage efficiency without measurable quality loss
- **Treated as a rebuildable cache**, with explicit model-version pinning so re-indexing is a known cost, not a hidden one

The single most important point: a cross-encoder reranker may **substitute for embeddings on many workflows** and should be evaluated as an alternative path, not stacked on top reflexively. The eval determines which wins per workflow.

---

## Where embeddings earn their keep, and where they don't

The workflow table from v0 still stands. The honest framing: embeddings are irreplaceable on a narrow but real set of surfaces, and using them anywhere else is malpractice.

Embeddings are the right tool when there is **no query text to feed a cross-encoder** (find notes related to *this note*), when paraphrase invariance matters more than precision (fuzzy "thing about X where I don't recall the words"), and when the user's vocabulary varies enough that BM25 misses semantic neighbors. Even on these surfaces, embeddings work best as a candidate-pool generator feeding a reranker — not as a final-ranking signal.

They are the wrong tool for direct query, title search, tag/date/folder filtering, exact-phrase lookup, link-graph traversal, and anything where the user supplied a specific term they expect to match. Aggregate-recall measurements hide this; per-workflow measurement reveals it.

---

## Provider strategy — remote-first, staged

There is no clean pure-Go local-transformer path in 2026. Every fast local option requires CGO and/or an unsigned native shared library, which breaks the single-binary distribution promise. Therefore:

**V1 default: remote API behind an `Embedder` interface.** Voyage `voyage-3.5-lite` (200M tokens/month free for individuals, 32K context, Matryoshka-truncatable) and OpenAI `text-embedding-3-small` ($0.02/M tokens) are the credible options. One-time embedding cost for a 50k-note vault is well under $1. Query-embed latency 100–300 ms is acceptable because embeddings are never on the high-frequency direct-query surface; they're on the synthesis and related-notes surfaces where 200 ms total is fine.

**Privacy opt-in: Ollama sidecar.** Same `Embedder` interface, talks to a user-installed Ollama process (not bundled). Preserves the single-binary promise for the default install while giving privacy-conscious users a fully local path without forcing you to ship an ONNX runtime.

**Deferred: in-process local embedder.** Hugot + ONNX Runtime, or llama.cpp + GGUF. Only justified if usage data shows enough users hit the API ceiling or refuse remote calls. Cost: `.app` bundle, dylib signing, notarization, vendor binary re-signing. Not v1, not v2, possibly never.

The clean property this preserves: provider choice is a config swap, not an architectural decision. The user (or you) can move between Voyage, OpenAI, Ollama, and any future provider by changing one constructor.

---

## Storage — exact cosine, not sqlite-vec

At personal scale (10k–50k vectors), brute-force exact cosine in Go is the right answer. Single-digit milliseconds at 50k vectors and 512 dimensions, 100% recall, zero tuning parameters, zero new dependencies.

The mechanics: store embeddings as `BLOB` columns in the existing SQLite file (one row per note, keyed by note UID), load them into a contiguous `[]float32` at startup, run cosine via a tight loop. Re-load on index update. The whole thing is roughly 50 lines of Go.

Why not sqlite-vec / `modernc.org/sqlite/vec`: at this scale sqlite-vec is itself brute-force, so it provides zero algorithmic advantage over the Go loop while adding the newest and least-proven dependency in the stack. The escalation path if cosine ever breaches latency budget (it won't here) is `coder/hnsw` — still pure Go, real HNSW, no native deps. sqlite-vec at this scale is a bet without a payoff.

---

## When to chunk

Default is **note-level embedding, no chunking**. Most personal-knowledge notes are atomic enough that the whole note is the right semantic unit. Chunking introduces a pipeline (boundary detection, overlap, header preservation), a fan-out (one note → multiple embeddings), and a deduplication problem at retrieval time.

Chunk when:

- A note's body exceeds the embedding model's context window (rare at 32K for `voyage-3.5-lite`)
- A note is a transcript or long-form article covering multiple distinct topics
- The eval shows long notes are systematically under-retrieved when embedded whole

Chunking strategy when you do it: **structural chunking by Markdown headings**, not fixed token windows. Each H2/H3 section becomes its own chunk, with the note title and parent heading path prepended as a contextual header (`Note: "X" / Section: "Y" / ...content...`). This is the Anthropic-style contextual chunk header pattern, but generated deterministically from the document's existing structure rather than via a model. Store chunks in a separate `chunks` table keyed by `(note_uid, chunk_id)` with the parent note's metadata propagated for filtering.

What not to do: fixed-token sliding windows, sentence-level chunking, or model-generated chunk summaries written into the chunks. The first two destroy conceptual boundaries; the third locks chunks to a model version.

---

## Dimensionality — truncate via Matryoshka

Voyage's `voyage-3.5-lite` and OpenAI's `text-embedding-3-small` are both Matryoshka-trained: the embedding can be truncated to a shorter prefix without retraining and without significant quality loss. The math at 50k notes:

- 1024 dims × 4 bytes × 50k notes = 200 MB on disk and in memory
- 512 dims × 4 bytes × 50k notes = 100 MB
- 256 dims × 4 bytes × 50k notes = 50 MB

The quality cliff on Matryoshka models is typically gentle to 512 and starts hurting around 256. **Truncate to 512 by default**; reserve 1024 for users with very large vaults where the extra recall matters. The truncation is a single `vec[:512]` slice — no model swap needed.

This also halves cosine latency and halves memory residency. Free win for Matryoshka-trained models; do not assume it for models that aren't.

---

## Model versioning and the re-indexing problem

Embedding spaces are not interoperable across model versions. Switching from `voyage-3.5-lite` to a future `voyage-4-lite` means re-embedding the entire vault. This is the hidden cost of embeddings and the reason they must be treated as a rebuildable cache.

Mechanism:

- Each embedding row stores `model_id` and `model_version` (e.g., `voyage-3.5-lite@2024-05-20`)
- The runtime refuses to mix vectors from different model versions in the same kNN query
- Model swap → mark old vectors as stale, re-embed in background, swap atomically when done
- Vault is unaffected; only the cache changes

This is one reason embeddings should not be written into the vault as frontmatter. They belong in the derived cache, where regeneration is a maintenance operation, not a vault rewrite.

---

## The cross-encoder substitution question

This is the most important architectural decision in this document and the easiest to get wrong.

A small cross-encoder reranker over BM25-top-50 — reading query and document together — produces a stronger relevance signal than embedding cosine on many query types. For workflows that have explicit query text (direct query, conceptual query, synthesis prompt), a cross-encoder may be **strictly better than embeddings** and may obsolete the embeddings dependency for those surfaces entirely.

Where embeddings remain irreplaceable: workflows with no query text. "Find notes related to this note" has no query to feed a cross-encoder; you need the source note's embedding to find neighbors. "What have I been thinking about over time" lookups, where the query is a region of vector space rather than a string, are similarly embedding-shaped.

Practical implication: embeddings and cross-encoders are not stacked reflexively. They are evaluated per workflow against the same fixture. Likely outcome:

- Direct query, exact-match, tag filter: BM25 + heuristic rerank, no embeddings, no cross-encoder
- Conceptual query, synthesis: BM25 + cross-encoder rerank, embeddings optional and gated by measured lift
- Find-related-to-this-note: embeddings + rerank (cross-encoder using note title as proxy query if needed)
- Fuzzy paraphrase: embeddings + rerank

Do not deploy embeddings on workflows where a cross-encoder over BM25 already wins. The dependency cost is real (provider API key, network call, model versioning, re-indexing) and unjustified when a model that already reads the query directly does better.

---

## Advanced techniques worth knowing

Not v1 territory but should be on the radar for when basic embeddings plateau:

**HyDE (Hypothetical Document Embeddings).** Generate a fake answer to the query with an LLM, embed that, search with the fake-answer embedding instead of the query embedding. Works because answers and documents share vector-space neighborhood structure that questions and documents often don't. Cheap to try, sometimes helps a lot on conceptual queries.

**Contextual chunk headers (model-generated).** Each chunk gets an LLM-generated 50–100 token prefix describing how it fits in the parent note. Stored in the derived index only, never in the vault. Anthropic measured a substantial recall lift from this; cost is one LLM call per chunk at index time, plus re-indexing when the prefixing model changes. Higher operational cost than structural chunk headers; sometimes worth it for transcript-heavy vaults.

**Late-interaction models (ColBERT, ColPali).** Embed per-token instead of per-document, score via MaxSim. Effectively a hybrid between embeddings and reranking — better retrieval quality than dense single-vector embeddings, more expensive storage (each note becomes many tiny vectors). Worth knowing as an alternative path that could bypass both the cosine-search and the cross-encoder discussions, though the Go-native story is currently weak.

**Query expansion via LLM.** Transform "taste" into "taste OR judgment OR aesthetics OR discernment" via an LLM call. Often beats embeddings outright for vocabulary-mismatch problems. Cheap, deterministic-enough, and the expanded query goes straight into BM25 — no embeddings needed. Worth trying before reaching for the embedding dependency.

---

## Evaluation — the conceptual subset is the only measurement that matters

The trap is measuring embedding quality on aggregate recall. Most queries against a personal vault are direct or structural; BM25 will dominate aggregate metrics whether or not embeddings help where they should. **Measure embeddings only on the conceptual/paraphrase/related-notes subset of the eval fixture.**

The right comparisons, all on that subset:

- BM25 alone (baseline from v0)
- BM25 + heuristic rerank (from the reranking companion)
- BM25 + cross-encoder rerank (the alternative path)
- BM25 + embeddings, RRF-fused
- BM25 + embeddings + cross-encoder rerank (the full hybrid)

Metrics: Recall@5, MRR, p95 latency including provider round-trip. The deployment decision per workflow is the configuration that maximizes MRR within the latency budget for that workflow. There is no single right answer across workflows.

---

## Migration path from v0

V0 ships with no embeddings. The path from there to a stable embedding-aware system:

1. **Eval fixture conceptual subset analysis.** Run v0 against the conceptual queries. Identify the workflows where Recall@5 is unacceptable.
2. **Cross-encoder spike.** Add a cross-encoder reranker over BM25-top-50. Re-measure. If this closes the gap, stop here — embeddings may not be needed.
3. **Embeddings spike, related-notes workflow only.** This is the surface where cross-encoders don't help. Add note-level embeddings via the `Embedder` interface, exact cosine in Go, Voyage or OpenAI. Measure.
4. **Embeddings expansion.** Only if the eval shows specific other workflows that embeddings help and cross-encoders don't, expand to those workflows.
5. **Chunking.** Only if transcript/long-form notes are systematically under-retrieved. Structural chunking by headings, never fixed windows.
6. **Local embedding consideration.** Only if usage data shows enough users hit limits or refuse remote calls.

The discipline: each step requires measured justification from the eval fixture. Embeddings are a feature, gated by evidence, scoped per workflow. The vault is untouched; the cache is rebuildable; the model version is explicit; the provider is swappable. That is what makes this maintainable a year out.
