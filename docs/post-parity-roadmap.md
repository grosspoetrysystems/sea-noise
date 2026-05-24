# Post-parity roadmap

What comes after parity ships. Reference material for the development arc beyond v1; **not** in scope for v1 itself.

Parity-first discipline is unchanged: until the Go port matches the Python prototype's behavior on the golden fixture vault, the rewrite does not sharpen, redesign, or simplify the prototype. Anything in this roadmap requires parity to land first.

## Reference documents

Two long-form notes live under `docs/post-parity/`:

- [`docs/post-parity/embeddings.md`](./post-parity/embeddings.md) — when embeddings earn their keep, provider strategy (remote-first via `Embedder` interface, Voyage / OpenAI default, Ollama as privacy opt-in), exact-cosine storage in Go, Matryoshka dimensionality truncation, model versioning + re-indexing, cross-encoder substitution question, evaluation discipline.
- [`docs/post-parity/reranking-and-shaping.md`](./post-parity/reranking-and-shaping.md) — engagement telemetry as the first post-parity build, heuristic reranking with per-workflow weight profiles, adaptive query classifier, vault shaping (stable UIDs, canonical metadata, body normalization, block anchors), output shaping (templated assembly, deduplication, provenance through every output).

## Sequencing intent

Both documents treat post-parity work as **eval-fixture-gated**: every stage advance requires a measured improvement on the conceptual-query subset of the eval fixture. No stage gets added because it is the trend.

Rough ordering (each gated by evidence):

1. Engagement telemetry table (prereq for the strongest heuristic features)
2. Heuristic reranker with per-workflow weight profiles
3. Vault shaping pass (UIDs, canonical metadata, body normalization, block anchors) — runs alongside reranking; durable across tool changes
4. Output shaping (templated assembly, dedup, provenance) — one workflow at a time
5. Cross-encoder reranker — when heuristic ceiling is measured
6. Embeddings (related-notes workflow first) — only when cross-encoder cannot help
7. Specialist reranker APIs — only when local cross-encoder hits its own ceiling

The two reference documents specify each stage in detail.

## Out of scope for this document

- Implementation details — see the linked references.
- Concrete v1 work — see `PARITY.md` and the v10 plan acceptance criteria.
- Distribution / packaging — intentionally excluded from v1; see `KNOWN-DELTA.md`.
