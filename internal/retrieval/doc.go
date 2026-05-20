// Package retrieval holds the retrieval router and the SQLite-backed
// retrieval engine.
//
// The router exposes Route(ctx, RetrieveRequest) → RetrieveResponse with
// modes DirectLookup, FTSSearch, GraphExpansion, DeterministicContextBuild,
// and OptionalSemanticExpansion (see ARCHITECTURE.md §4).
//
// At this scaffold wave (T02) the package exists only as a placeholder
// so the layout is committed before implementation lands. The router
// interface lands in subsequent waves; the SQLite engine lands in T10
// (Phase 4).
package retrieval
