# Post-v0 — Reranking, signal weighting, and vault shaping

*Companion to the v0 retrieval stack recommendation. This document specifies what gets built after the v0 substrate (modernc.org/sqlite + FTS5 + metadata + wikilink graph) is shipped and proven on the eval fixture. Its companion piece, "Post-v0 — Embeddings, dimensionality, and the semantic question," covers the parallel decisions for the semantic side. Both documents share the eval harness defined in v0 as the gating mechanism for every staged advance.*

---

## Bottom line

After v0 ships, the next development arc has three threads that should run in parallel, not serially:

- **Reranking** progresses from metadata-weighted BM25 → heuristic reranking → (optionally) cross-encoder reranking, gated at each stage by measured improvement on the conceptual-query subset of the eval fixture
- **Engagement telemetry** is the prerequisite that unlocks the strongest heuristic signals (recently-opened, open-frequency, active-project) and should be the first thing built post-v0
- **Vault shaping** — deterministic, model-independent, non-semantic normalization passes that write durable metadata into frontmatter and standardize body conventions — runs alongside reranking work and is where the durable product differentiation actually lives

Specialist reranker APIs (ZeroEntropy, Cohere Rerank, Voyage Rerank) come last and only with an explicit measured trigger, not because they feel like the next step.

---

## The staged path, with sharpenings

The right evolution is BM25 baseline → metadata weighting → heuristic rerank → hybrid retrieval → cross-encoder rerank → specialist API. Three sharpenings turn the survey into a v1 spec.

### Stage 1 prerequisite — engagement telemetry

The strongest signals for a personal-knowledge reranker are not document features (title, body, tag) but interaction features: which notes the user has opened recently, which they open frequently, which belong to their currently active project. These signals are dramatically more predictive than created/modified timestamps because "recently engaged with" and "recently edited" are different things, and engagement correlates more directly with relevance.

Build this first. A small SQLite table — `note_uid, opened_at, surface` — logged from whatever read surfaces the runtime exposes (CLI, future tools, optionally a tiny Obsidian plugin that writes timestamps via a shared SQLite path or webhook). The data is local-only, deterministic to aggregate (open count, last-opened, decayed engagement score), and feeds every subsequent stage. Without it, heuristic reranking is missing its best features.

### Stage 3 — heuristic reranking as the actual workhorse

The right starting shape:

```
score = bm25_score
      + title_match_boost
      + tag_match_boost
      + recency_boost
      + backlink_boost
      + active_project_boost
      + recent_open_boost
      - archived_penalty
```

Two things this gets right that hosted rerankers can't compete with: the signals are personal (no generic model knows your folder structure means anything), and the system is fully debuggable (every component of every score is inspectable in SQL). For a personal vault this often beats neural rerankers on the queries that matter.

Tune the weights on the eval fixture by query type. The right weights for direct-query are not the right weights for synthesis. This is where workflow-specific context builders earn their keep — each workflow gets its own weight profile, expressed as a few constants, version-controlled, A/B-testable against the fixture.

### Adaptive retrieval — the query classifier is the mechanism

Cheap queries should skip expensive reranking; ambiguous queries should fall through to it. The classifier is the actual hard part and deserves explicit specification.

In practice it's a query-shape heuristic running a small set of deterministic tests in order: presence of explicit operators (quotes, tag prefixes, type filters); presence of an exact title-token match against the FTS index; presence of a recognized entity from the wikilink graph; query length and token-rarity statistics. Queries that pass any cheap-path test route to BM25-only or BM25+metadata. Queries that fail every test fall through to the expensive path.

This classifier is what makes the cost story work. Without it, "adaptive retrieval" is wishful thinking and you end up running the expensive path on every query.

### The measured trigger for moving to neural rerankers

The symptom is "correct note in top 50 but not top 5." The trigger should be a number, not a vibe. The right form: when **MRR on the conceptual-query subset of the eval fixture stagnates across two consecutive heuristic-tuning iterations, and Recall@50 on that same subset is materially higher than Recall@5** (say, >15 points), the heuristic reranker has hit its ceiling and a model that reads query and document together becomes worth the dependency.

Same eval harness from v0, longer time horizon. This keeps the staging honest — neural rerankers get added because the data says so, not because they're the trend.

### Cross-encoder vs hybrid — these are not sequential

A common framing puts hybrid-with-embeddings before cross-encoder reranking. For this product, decouple them and possibly invert.

A small cross-encoder over BM25-top-50 with no embeddings anywhere is a complete shippable architecture. Cross-encoders read query and document together and produce a stronger relevance signal than embedding cosine for many query types. If the v0 eval shows BM25 has good Recall@50 on conceptual queries but poor ranking precision, the cross-encoder path may be more leverage than the embeddings path. They can substitute for each other on some workflows; on others (find-related-to-this-note, fuzzy paraphrase lookup) embeddings remain irreplaceable because there is no query text to feed a cross-encoder. The eval decides, per workflow.

Practical implication: treat reranking and embeddings as independent threads with shared eval gating, not as a linear sequence. See the embeddings companion for the parallel argument.

---

## Vault shaping — where the durable differentiation lives

The shaping question is separate from reranking and arguably higher-value over the long run. Where reranking improves how retrieved candidates are ordered, shaping improves the corpus itself — making retrieval, synthesis, and downstream workflows materially better for free, durably, and in a way that survives every tool change.

The constraint isn't "don't touch the vault." It's "anything written into the vault must be model-independent, human-readable, deterministic-to-regenerate, and useful even if the runtime disappears tomorrow." Frontmatter and conventional Markdown syntax pass this test trivially.

### Input shaping — deterministic vault normalization

Highest-leverage additions, ordered by value:

**Stable UIDs in frontmatter.** A `uid:` field (ULID or short hash) generated once at first index and never changed. Lets wikilinks survive title renames, lets the derived index reference notes without filename coupling, survives folder reorganization. Convention-compatible — Obsidian, Foam, Logseq, Quartz all tolerate arbitrary frontmatter keys. Pure win: non-proprietary, non-semantic, deterministic.

**Canonicalized structural metadata.** `created:` / `modified:` in RFC 3339, `wordcount:`, `source_url:` (lifted from inline patterns to frontmatter so it's queryable), `note_type:` if derivable from folder or pattern, `incoming_links:` count maintained at index time. None model-generated; all single-pass scan; payoff is that your SQL metadata layer stops recomputing on every query.

**Body normalization passes.** Canonical wikilink syntax (`[[Note|alias]]` vs `[[Note]]` collapsed to one form), inline URLs converted to proper Markdown links where context allows, tag syntax standardized, heading levels normalized so the top-level heading is consistent, every code fence given a language hint or `text` fallback. All model-independent, readable, idempotent. The IR literature is clear: document normalization moves retrieval quality more than retriever tuning. Anserini, Pyserini, and BEIR maintainers all emphasize this.

**Stable block anchors.** Generate `^block-id` anchors at index time for every paragraph or significant block, using the standard Obsidian-compatible syntax. Turns every retrievable unit into a stably addressable target that survives the user editing surrounding content. Retrieval can return "Note X, block ^a1b2" and that anchor stays valid across edits.

What to avoid even though it's tempting: auto-tagging that writes tags into frontmatter (semantic, model-dependent, not durable), auto-summaries inserted as note sections (mutates user content, locks in a model version), semantic IDs replacing human filenames (breaks portability), maintained "smart" frontmatter fields the runtime owns (turns the vault into a database with markdown skin). Anything Obsidian-proprietary — Dataview inline fields, callout syntax `> [!note]`, `%%comments%%`, canvas files — stays out of anything the runtime writes. The convention floor is CommonMark + YAML frontmatter + wikilinks + tags + block anchors; that's the portable substrate and the only thing your shaping passes should produce.

### Output shaping — workflow-specific assembly with provenance

This is the place where context engineering actually applies and where most products under-invest.

**Templated assembly per workflow.** A research-pack output is not "top-10 BM25 results concatenated." It's a templated document: source query, scope (date range, tags, folders), per-result entry with title, breadcrumb path, snippet around the hit term, surrounding heading context, outgoing wikilinks, frontmatter metadata. The template is the product; retrieval is the substrate. Synthesis wants a different template than research-pack which wants a different template than fleeting-triage. Each workflow gets its own.

**Deduplication at assembly time.** Web clips especially: multiple notes covering the same article from different reading sessions are common. A MinHash or shingled-trigram pass over retrieved candidates collapses near-duplicates before output. Deterministic, cheap, non-semantic, makes outputs feel curated rather than dumped.

**Provenance through every output.** Every snippet carries its source: note UID, block anchor if available, character offset, retrieval signals that surfaced it (BM25 score, tag match, link distance, rerank score). Free auditability for downstream model-using workflows, and clickable backlinks for the user reading the output.

---

## What to ship in v1 vs defer

V1 (post-v0, in order):

1. Engagement telemetry table and aggregation queries
2. Heuristic reranker with per-workflow weight profiles, gated by eval fixture
3. Query-shape classifier for adaptive retrieval
4. Vault shaping pass: UIDs, canonical metadata, body normalization, block anchors — idempotent so it can run on every index
5. Output shaping: templated assembly + provenance preservation + dedup, one workflow at a time starting with research-pack

V2+ (gated by measured triggers):

- Cross-encoder reranker (when conceptual MRR stagnates and Recall@50 ≫ Recall@5)
- Hybrid retrieval with embeddings (when the embeddings companion's eval gates trip)
- Specialist reranker APIs (when local cross-encoder hits its own ceiling)

The discipline that makes this maintainable: every stage advance requires a measured improvement on the eval fixture for the workflow it targets. No stage gets added because it's the trend. The boring substrate stays boring; the intelligence accumulates in composable, debuggable, swappable layers above it.
