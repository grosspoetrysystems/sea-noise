---
okf_version: "0.2"
---
# Sea Noise documentation

Sea Noise is a local-first Go runtime for retrieval, context assembly, and
small automation pipelines over a Markdown vault. The vault is the source of
truth; Sea Noise reads it and writes derived artifacts, never editing source
notes through its retrieval path.

This corpus is for agents and contributors operating the two surfaces the
runtime actually exposes:

- A **CLI** (`sea-noise <subcommand>`) for one-shot workflows and queries.
- A resident **daemon** (`sea-noise serve`) with a local Unix-socket JSON-RPC
  dispatch surface and a filesystem watcher.

There is no web/UI surface in this runtime. The HITL control plane is a
separate forthcoming product (see [Architecture](architecture.md) §
boundaries).

## Start here

- [Getting started](getting-started.md) — build the binary, configure a vault,
  run your first command.
- [Configuration and integrations](configuration.md) — vault-path resolution,
  `sea-noise.toml`, credentials, and failure boundaries.

## Surfaces

- [CLI commands and workflows](cli.md) — every product-facing subcommand,
  which ones write vault files, and safe-learning guidance. The test/parity-only
  `parity-discover` command is intentionally outside this workflow guide.
- [Daemon](daemon.md) — lifecycle, internal scheduler, IPC verbs, and the
  filesystem watcher.

## Design

- [Architecture](architecture.md) — retrieval substrate, SQL schema, the two
  router seams, the optional semantic layer's current limits, and hard binary
  constraints.
- [Maintainer operating modes](maintainer-modes.md) — branch-protection posture
  and pull-request expectations.

## Scope and gaps

- Pipeline internals, skill-prompt wording, and parity-fixture mechanics live in
  source and `testdata/`, not in this corpus.
- Acceptance metrics (Recall@K, MRR, latency) are deliberately absent — the
  runtime is judged behaviorally against fixtures, not by IR metrics.
