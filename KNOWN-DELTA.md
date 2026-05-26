# Known deltas

Decisions taken in flight that diverge from earlier plan revisions, codified here so future contributors do not re-litigate them. This document is the durable record; the rationale lives alongside each delta.

## Scaffold simplifications (T02)

The earlier plan called for Kong CLI framework, koanf TOML+env config, a Makefile, and GitHub Actions CI. The repo ships:

- Hand-rolled `flag` + switch dispatcher in `cmd/sea-noise/main.go`
- Custom TOML loader in `internal/config/`
- Shell scripts (`scripts/build.sh`, `scripts/test.sh`, `scripts/grep-gates.sh`) in place of Makefile and GitHub Actions
- Binary name `sea-noise` (an earlier plan used a different name, since dropped)

Rationale: minimize first, layer up after proof. The simpler scaffold passes its own tests, builds clean, and keeps the dependency surface small. Restoration of any of these substitutions becomes a future track only if the simpler shape proves inadequate.

## Client cuts restored (T04)

The earlier plan listed several clients that were dropped silently during initial T04 work. v10 restores them as explicit scope:

- `internal/crawl/` — Cloudflare crawler extracted from `internal/enrichtopics/` into a standalone reusable client
- `internal/todoist/` — Todoist REST v2 + Sync v9 client with exponential backoff via `retryablehttp`
- `internal/youtube/` — YouTube timedtext transcript client
- `retryablehttp` added to `go.mod`

The `Embedder` interface in `internal/ai/embedder/` stays seam-only. No concrete implementations ship in v1 (no Voyage, OpenAI, Ollama, or Noop embedder). Embeddings are a feature flag, not a foundation; concrete implementations enter when the post-parity eval shows BM25 plus heuristics cannot close a workflow gap.

## Subcommand gaps restored (T05)

The earlier plan called for `fetch`, `fetch --retry`, `sync todoist push`, `sync todoist --completions`, and `youtube` subcommands. v10 restores all five. They register in the same `flag` + switch dispatcher as the already-shipped subcommands.

Two prototype behaviors inside `fetch-stub-content.py` are intentionally out of v10 scope:

- **Platform-API pre-fetch path.** The Python script imports a local-only `platform_apis` module (`fetch_via_platform_api`, `crawl_upgrade_queue`) that is not in `references/scripts/`. Without it as authority for parity comparison, the Go `internal/fetchstub/` port calls Cloudflare via `internal/crawl/` directly. Result: same end-state for plain web pages; Substack-Notes-style multi-platform unwrapping is deferred.
- **Crawl-upgrade post-pass.** Runs only when `platform_apis` has populated `crawl_upgrade_queue`. With the platform path deferred, the post-pass has no input and is also deferred.

Both deferrals are recoverable: if the platform-API module surfaces in a future `references/` refresh, the post-pass slots back in as a second call to `internal/crawl/` over the queued URLs. The manifest-backed retry surface is fully ported and provides the safety net for the missing path.

## Retrieval router scope (T08)

The retrieval router under `internal/retrieval/` (SQLite FTS5 + BM25 + graph expansion) was originally planned to back AI-workflow context assembly. Investigation showed every prototype-derived workflow scans the vault directly via `vault.Walk` plus substring or regex matching. Wiring the router into them would change candidate sets and break parity.

The router stays an architectural seam, exposed only through the `sea-noise index` and `sea-noise search` surfaces. See `ROUTER-SEAM.md` for the full boundary statement.

## Export sequencing (T12)

`sea-noise export research-packs` consumes `_meta/topic-frequencies.md`. The Python prototype does not have a script that writes this file — instructions in `references/prompts/CLAUDE.md` direct the LLM to maintain it during inbox processing.

The Go port preserves this shape: T07 ships an inbox-process skill that writes `_meta/topic-frequencies.md`, `_meta/rejected-connections.md`, and `atlas/*` MOC updates. T11-watch keeps the file fresh by triggering the inbox-skill on new notes. T12 then ports as a consumer with no prerequisites of its own beyond T07 + T11-watch.

## Distribution actively excluded (T14)

The earlier plan included a distribution track: `sea-noise init` wizard, `sea-noise schedule install/remove/status`, goreleaser config, Homebrew tap, release notes. **None of this ships in v1, and reintroducing it would actively harm the product.**

Rationale: the Python prototype is invoked by external schedulers (cron, launchd plists firing the catchup shell scripts). On macOS this triggers a constant churn of TCC approvals and firewall prompts — every script invocation looks like new external software to the OS, and iCloud volume mounts plus venv path shifts compound the problem. The primary motivation for the Go rewrite is escaping this churn by running as a single static binary as a resident process: approved once, stays approved.

Generating launchd plists or installing cron entries on the user's behalf would reintroduce exactly the surface area the rewrite is escaping. v1 ships `sea-noise serve` and example shell invocations for starting it from a login shell or terminal. Users register the daemon however they prefer; the product does not own that decision.

See `DAEMON.md` for the daemon lifecycle and IPC contract.

## Cosmetic naming

- The inference router type is named `LocalRouter` (`internal/ai/inferencerouter/`). An earlier draft used `StubInferenceRouter`. Shape is identical.
- Config file is `sea-noise.toml`. Earlier code referenced an older filename; the rename follows the binary rename. The `MIND_PALACE_VAULT` environment variable still works as a fallback for vault path resolution and intentionally retains its historical name for compatibility with the Python prototype's runtime convention.

## Scoping note for grep-based scrubs

The Python prototype's source name appears in `references/` (parity authority), `testdata/fixture-vault/` (golden fixture), dated `docs/2026-05-16-*` (historical artifacts), and real disk-path documentation in `README.md` and `docs/MIGRATION.md` (the user's Obsidian vault folder is named after the prototype). These instances are intentionally preserved — modifying parity-authority content breaks the parity-first discipline, and disk-path references describe real filesystem state. Any future scrub gate must carve these out explicitly.
