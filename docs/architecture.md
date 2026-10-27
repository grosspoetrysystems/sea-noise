---
type: reference
status: stable
sources:
  - resource: ARCHITECTURE.md
  - resource: ROUTER-SEAM.md
  - resource: internal/retrieval/router.go
  - resource: internal/retrieval/schema.go
  - resource: internal/ai/inferencerouter/types.go
  - resource: internal/ai/embedder/embedder.go
---
# Architecture and boundaries

Sea Noise is a local retrieval and context-assembly runtime over a Markdown
vault. The vault remains the source of truth; SQLite is a rebuildable derived
index. `ARCHITECTURE.md` is the authoritative design narrative; this page is an agent-oriented map.

## Retrieval path

The retrieval package uses pure-Go SQLite with FTS5/BM25 as the primary
substrate. Its schema stores notes, chunks, tags, entities, links, assertions,
timestamps, and FTS5 tables. Backlinks are derived by inverting `links`, not
stored separately.

The router accepts a query, filters, mode, and limit, then dispatches. Three
modes are implemented today:

- direct lookup by note ID or path (no title lookup);
- FTS search for keyword/phrase queries; and
- graph expansion from seed notes.

Two further modes are declared but not wired: deterministic context building for
a workflow ID and optional semantic expansion both return an unsupported-mode
error until later waves land them.

Routing is deterministic and does not invoke an LLM. `sea-noise index` builds or
refreshes the SQLite index, and `sea-noise search` queries it.

## Router seam versus prototype parity

`ROUTER-SEAM.md` is an important boundary:

- New retrieval-shaped surfaces use `internal/retrieval`.
- Prototype-derived workflows preserve Python-compatible vault walking,
  substring matching, and regular expressions. They do not consume the router.

Using BM25 in a parity workflow would change its candidate set and can silently
diverge from fixture behavior. Do not modernize that boundary without
re-capturing the golden fixture outputs.

## Optional semantic and inference layers

The embedder interface and optional semantic expansion are design seams, but
the retrieval package does not wire them in this wave. There is no current
embedding cache or cosine-ranking path in `SQLiteRouter`.

The inference router is a separate, harness-agnostic decision seam. It does not
retrieve, call `exec.Command`, make outbound HTTP requests, or import provider
SDKs. The transport layer consumes its decision; the current CLI transport can
invoke a configured local inference executable.

## Current retrieval limits

No semantic-to-FTS fallback is active. `ModeDeterministicContextBuild` and
`ModeOptionalSemanticExpansion` return `ErrUnsupportedMode`, and
`RetrieveResponse` contains only the mode and hits—there is no structured
degradation-warning field. A semantic fallback and warning contract remain
future design, not shipped behavior.

## Binary constraints

The shipped binary is intended to be CGO-free and static, with no Python
runtime, MCP requirement, ONNX ship blocker, Ollama dependency, external vector
database, or `sqlite-vec` dependency. These are repository constraints, not
claims that every optional integration is available on every host.

## Product boundary

This repository has no web server or UI. The HITL control plane is a separate
forthcoming product. The daemon's agent-facing surface is local Unix-socket
JSON-RPC, not HTTP; see [Daemon](daemon.md).

Open boundary: the architecture names a future optional semantic provider but
does not select a required provider or guarantee a particular remote service.
