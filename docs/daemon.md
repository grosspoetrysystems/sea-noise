---
type: guide
status: stable
sources:
  - resource: DAEMON.md
  - resource: cmd/sea-noise/serve.go
  - resource: cmd/sea-noise/run.go
  - resource: cmd/sea-noise/ipc_handler.go
  - resource: internal/ipc/protocol.go
  - resource: internal/watcher/watcher.go
  - resource: internal/daemon/daemon.go
---
# Resident daemon

`sea-noise serve` keeps scheduling, vault-change reaction, and local dispatch in
one process. It runs until SIGINT/SIGTERM, then stops the watcher and IPC
listener before stopping the daemon. `DAEMON.md` is the complete product contract.

## Start and lifecycle

```sh
sea-noise serve
```

The daemon can run in a terminal, under a shell background job, or in a
terminal multiplexer. It does not install a launchd plist, write cron entries,
auto-start at boot, or bind a TCP port. A second daemon using the same socket
path refuses to start rather than taking over the existing process.

Useful flags include:

- `--dry-run` skips pipeline step functions and marker writes, but still creates
  or appends the vault's `_meta/processing-logs/YYYY-MM-DD.md` file;
- `--socket PATH` chooses the Unix socket (`$SEA_NOISE_SOCKET`/default when
  omitted); `--no-ipc` disables IPC;
- `--no-watch` disables the filesystem watcher;
- `--watch-debounce-ms N` tunes write-burst coalescing;
- `--cli-command`, `--cli-args`, `--skills-dir`, and `--timeout` configure the
  optional inference transport; and
- `--index-db PATH` chooses the retrieval index.

The default `--cli-args` value is
`-p,{PROMPT},--dangerously-skip-permissions`. This passes a flag that disables
the configured inference CLI's permission gate; it does not grant OS-level
permissions. Review or override it before enabling vault writes or scheduled
execution.

The vault path is still resolved from the normal configuration precedence;
`serve` does not accept an arbitrary vault argument.

## Scheduled chains

The daemon uses UTC. Its fixed defaults are daily at `00:05 UTC` and weekly on
Sunday at `01:00 UTC`; current `serve` and `run` wiring exposes no schedule
configuration surface, so these are not user-configurable through the CLI.
The registered step order is:

- **Inbox**: every 30 minutes when the scheduler clock is at minute `00` or
  `30`, and when the watcher reports a new `inbox/` file:
  `hydrate-stubs → process-inbox → youtube-transcript`. There is no explicit
  startup inbox run.
- **Daily**: `rotate-logs → retry-crawls → summarize → index →
  daily-synthesis` (deferred) `→ sync-consumed` (deferred) `→ signal →
  plunge` (deferred) `→ home` (deferred) `→ todoist-push → todoist-pull`.
- **Weekly**: `retry-crawls → weekly-synthesis` (deferred) `→ index → signal →
  audit-links → enrich-topics → profile-entities`.

`_meta/.daily-last-run` and `_meta/.weekly-last-run` gate only successful
non-dry-run `daily` and `weekly` runs; `inbox` has no marker. Executed
non-deferred steps log their name, start time, duration, and status. Deferred
steps instead log `DEFERRED (no Go port yet)`, while dry-run steps log
`[DRY-RUN] ... — would run`; both formats omit execution timing and status.
Dry-run still creates or appends the processing log. The exact same step lists
are available as `sea-noise run inbox|daily|weekly` for one-shot debugging.

## Local IPC

The IPC server uses a Unix domain socket and JSON-RPC-like JSON objects through
the standard library's `net` package. It is not HTTP and is local-only. The
handler in `cmd/sea-noise/ipc_handler.go`
bridges transport to the daemon, skill executor, and retrieval router.

The verbs are:

- `skill.run` — execute a named inference skill;
- `pipeline.trigger` — trigger `inbox`, `daily`, or `weekly`. Its `force` field
  is reserved and not wired, so it has no effect; daily/weekly marker gates
  still apply and `inbox` has no marker;
- `vault.query` — query the SQLite retrieval index;
- `index.refresh` — perform a full vault index rebuild; it has no scoped-glob
  parameter; and
- `status` — return `version`, `vault_root`, `uptime_sec`, and optional
  `dry_run` fields. It does not return PID, run markers, or watcher/queue state.

Clients need socket access and the documented JSON schemas. The daemon is
agent-agnostic: it does not know which CLI or harness connected.

## Filesystem watcher

The watcher uses `fsnotify` and a configurable debounce window. A new note in
`inbox/` triggers the inbox pipeline. Current `serve` wiring does not claim
incremental re-indexing for every modified note; the full index pipeline
remains the explicit refresh path. Removed or modified-note behavior should be
checked against the current watcher implementation before relying on it.

## Failure boundaries

A missing or invalid vault path prevents startup. Optional inference and
Todoist collaborators can be absent, while their dependent steps remain
unavailable or deferred. Invalid socket paths, an occupied socket, and startup
errors stop the daemon instead of silently switching to a different transport.
