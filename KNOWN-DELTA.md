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

## AI workflow shapes (T07)

The Claude-integration workflows split into two execution shapes:

- **Go-orchestrated workflows** (vault-wide `enrich-topics --auto`, `enrich-topics --note`, vault-wide `profile-entities --auto`). The Go process walks the vault, picks candidates, calls the inference skill per-candidate, and writes the results. Tests use `t.TempDir` + a mocked `MockTransport` to assert against the per-candidate prompts and post-write vault state. This matches the existing single-note shape (`enrich-topics <topic>`, `profile-entities <entity>`, `summarize-transcripts`).
- **LLM-orchestrated workflows** (`process-inbox`). The Go process loads the skill, fires it through the inference CLI once, and reports back whatever summary the LLM emits. The LLM uses its own file tools (via `claude -p --dangerously-skip-permissions` or equivalent) to read inbox/, write `_meta/topic-frequencies.md`, `_meta/rejected-connections.md`, and `atlas/*` MOC updates, and to promote notes out of inbox/. Tests can only assert that the skill is wired and the prompt receives the vault root — runtime behavior depends on the LLM and is not parity-asserted from Go.

`_meta/topic-frequencies.md` is the contract between `process-inbox` (writer) and `internal/signal/` (reader): a single 4-column markdown table `| tag | count | last_seen | trend |`. The inbox-process skill is pinned to this column layout. The prototype's older 5-column shape `| topic | first_seen | last_seen | count | trend |` is read tolerantly by `internal/enrichtopics/auto.go` for backwards compatibility but is not what fresh `process-inbox` runs emit.

## Daemon shell (T11) + deferred catchup steps

The resident-process daemon shell (`internal/daemon/`) ships with the daily + weekly + inbox pipeline shape, marker-file gating, processing-log writing, `--dry-run`, and step-failure isolation. The step lists pin the prototype's catchup-script step names byte-for-byte. Each step is either bound to an in-process Go workflow or marked **deferred** with `Fn: nil`, in which case the daemon logs `DEFERRED (no Go port yet)` and continues.

Deferred steps as of T11 merge:

| Step | Prototype script | Status |
|------|------------------|--------|
| `daily-synthesis` | `claude -p "daily synthesis" --dangerously-skip-permissions` | LLM-orchestrated; no skill file yet. Future port adds a `daily-synthesis` skill mirroring the prompt. |
| `weekly-synthesis` | `claude -p "weekly synthesis" --dangerously-skip-permissions` | Same shape as daily-synthesis; same future port. |
| `sync-consumed` | `_scripts/sync-consumed.py` | Not in `references/scripts/`. The prototype keeps it locally only; the parity-authority gap means no Go port until the script surfaces. |
| `plunge` | `_scripts/generate-plunge.py` | Same — not in `references/scripts/`. |
| `home` | `_scripts/generate-home.py` | Same — not in `references/scripts/`. |

The daemon's `Server.RunPipeline` is the plug-in seam for IPC (T11-IPC) and the fsnotify watcher (T11-watch). Both subsystems ship as their own tracks and reach into the daemon via this single entry point — no further refactor of `internal/daemon/` is required to add them.

Time source is `time.Now().UTC()` for scheduling boundaries; the prototype uses local time. UTC was chosen for determinism (DST transitions, multi-timezone vault syncs). If proof demands local-time scheduling, swap the clock implementation; the `Clock` interface in `internal/daemon/` makes that a one-line change.

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

## IPC socket path budget

The daemon's JSON-RPC IPC listens on a Unix domain socket. Unix domain sockets are bound by `sockaddr_un.sun_path`, which is capped per platform: 104 bytes on Darwin, 108 on Linux (the final byte is reserved for the null terminator on most platforms). We use a conservative cross-platform budget of 103 characters in `internal/ipc.ValidateSocketPath`, and `sea-noise serve` fails fast with `ipc: socket path exceeds sun_path limit` rather than letting the bind syscall return a cryptic error at runtime.

Default path is `$SEA_NOISE_SOCKET` if set, otherwise `~/.sea-noise/daemon.sock`. The 103-byte budget is enough headroom for typical macOS home directories (`/Users/<short-name>/.sea-noise/daemon.sock` is ~40 chars). Users with long usernames or relocated home directories who hit the limit can point `$SEA_NOISE_SOCKET` at `/tmp/sea-noise.sock` or another short path.

The IPC integration tests construct ephemeral sockets under `/tmp/sn-ipc-XXXXXX/s` rather than `t.TempDir()` because `t.TempDir()` under macOS lives at `/var/folders/...` which routinely exceeds the budget by itself.

## Scoping note for grep-based scrubs

The Python prototype's source name appears in `references/` (parity authority), `testdata/fixture-vault/` (golden fixture), dated `docs/2026-05-16-*` (historical artifacts), and real disk-path documentation in `README.md` and `docs/MIGRATION.md` (the user's Obsidian vault folder is named after the prototype). These instances are intentionally preserved — modifying parity-authority content breaks the parity-first discipline, and disk-path references describe real filesystem state. Any future scrub gate must carve these out explicitly.
