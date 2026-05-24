# Parity status

Current state of the Go port against the Python prototype in `references/scripts/`. Authority precedence when sources conflict: (1) Python runtime behavior on the fixture vault, (2) prompts in `references/prompts/`, (3) docs in `references/docs/`.

Acceptance is **behavioral diff** against captured prototype output on the golden fixture vault (`testdata/fixture-vault/` → `testdata/expected/`). There are no metric thresholds.

## Ported subcommands

| Go subcommand | Python prototype | Status | Golden |
|---------------|------------------|--------|--------|
| `sea-noise audit-links` | `audit-links.py` | shipped | `testdata/expected/audit-links/report.md` |
| `sea-noise generate-signal` | `generate-signal.py` | shipped | `testdata/expected/signal/queue.md` |
| `sea-noise summarize-transcripts` | `summarize-transcripts.py` | shipped | (per-note diff) |
| `sea-noise profile-entities` | `profile-entities.py` | shipped, single-entity invocation only — vault-wide path pending T07 |
| `sea-noise enrich-topics` | `enrich-topics.py` | shipped, single-topic invocation only — vault-wide path + `--note` path pending T07 |
| `sea-noise index` | `_rag/` indexer | shipped (new surface, no prototype golden) |
| `sea-noise search` | `_rag/` retrieval | shipped (new surface, no prototype golden) |

## Not yet ported

| Python prototype | Status | Notes |
|------------------|--------|-------|
| `fetch-stub-content.py` | pending T05 | Hydrates URL stubs; restores `sea-noise fetch` + `sea-noise fetch --retry` subcommands. Needs `internal/crawl/` standalone client (T04). |
| `fetch-youtube-transcript.py` | pending T05 | Restores `sea-noise youtube` subcommand. Needs `internal/youtube/` (T04). |
| `sync-todoist.py` | pending T05 | Restores `sea-noise sync todoist push`. Needs `internal/todoist/` (T04). |
| `sync-todoist-completions.py` | pending T05 | Restores `sea-noise sync todoist --completions`. Needs `internal/todoist/` (T04). |
| `export-research-packs.py` | pending T12 | Sequenced after T07 inbox-skill and T11-watch. Consumes `_meta/topic-frequencies.md` maintained by the inbox-skill. |
| `run-daily-catchup.sh` | pending T11 | Becomes the internal daemon scheduler's daily chain. |
| `run-weekly-catchup.sh` | pending T11 | Becomes the internal daemon scheduler's weekly chain. |

## Inbox-process skill

The Python prototype maintains `_meta/topic-frequencies.md`, `_meta/rejected-connections.md`, and `atlas/*` MOCs via instructions in `references/prompts/CLAUDE.md` executed during inbox processing. No Python script writes these files — they are LLM-maintained.

The Go port restores this via an inbox-process skill (pending T07). The skill writes the same files in the same shape so downstream consumers (`generate-signal`, `enrich-topics`, `export-research-packs`) parse them without format negotiation.

## Capture mechanism

`testdata/scripts/capture-parity.py` runs the Python prototype against the fixture vault and writes golden outputs into `testdata/expected/`. It pins report dates (`AUDIT_REPORT_DATE`, `SIGNAL_REPORT_DATE`) for determinism. Goldens regenerate when the prototype changes; the Go port is the diff target.

## What is enforced as architectural invariant

`scripts/grep-gates.sh` polices three invariants:

1. No provider / model-name string literals outside `internal/config/` and tests.
2. No Python subprocess invocations in shipped Go binary.
3. No MCP imports.

CGO is disabled (`CGO_ENABLED=0`); the binary is a single static Go artifact.
