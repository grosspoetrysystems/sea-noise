# Daemon

Sea Noise ships as a resident background daemon. This document is the contract: lifecycle, IPC surface, filesystem watcher, and how users start the daemon without delegating that decision to the binary.

## Why a daemon

The Python prototype is invoked by external schedulers — cron entries and launchd plists firing `run-daily-catchup.sh` and `run-weekly-catchup.sh`. On macOS this triggers a constant churn of TCC approvals and firewall prompts. iCloud volume mounts shift, the Python venv path moves, the script paths change, and the OS treats each new state as new external software. The Go rewrite escapes this by running as a single static binary as a resident process: approved once, stays approved.

The daemon owns the schedule internally. It owns the vault-change reaction. It owns the dispatch surface that any agent can call into. There is no separate scheduler, no install step, no plist generation.

## Lifecycle

```sh
sea-noise serve
```

Starts the resident daemon. Runs indefinitely until killed (`SIGINT`, `SIGTERM`) or the process exits. Clean shutdown:

1. Stops accepting new IPC requests
2. Waits for in-flight pipeline steps to complete (bounded by a configurable grace period)
3. Removes the IPC socket file
4. Flushes any pending processing-log entries
5. Exits 0

A second `sea-noise serve` against the same socket path refuses to start and exits non-zero with a message identifying the existing process. There is no `--force` flag.

`sea-noise.toml` configures: vault path, socket path, daily schedule time, weekly schedule day + time, dry-run default, watcher debounce interval.

## How to start the daemon

The daemon does not install itself. Pick one:

```sh
# Foreground in a terminal:
sea-noise serve

# Background from a login shell, output to a log file:
sea-noise serve >> ~/.sea-noise/daemon.log 2>&1 &

# From your shell rc (zshrc / bashrc) on login:
pgrep -f 'sea-noise serve' >/dev/null || (sea-noise serve >> ~/.sea-noise/daemon.log 2>&1 &)

# From a terminal multiplexer (tmux, screen):
tmux new-session -d -s sea-noise 'sea-noise serve'
```

There is intentionally no `sea-noise schedule install` subcommand and no auto-generated launchd plist. Registering with launchd would reintroduce the exact TCC and approval churn this design escapes. See `KNOWN-DELTA.md` for the full rationale.

## Internal scheduler

Two timed chains, both driven by Go time package primitives inside the daemon:

- **Daily** — fires at the configured daily time (default 00:05 local). Step order mirrors `references/scripts/run-daily-catchup.sh`:
  `retry-crawls → summarize-transcripts → index → synthesis → sync-consumed → generate-signal → plunge → home → todoist-push → todoist-pull`

- **Weekly** — fires at the configured weekly day + time (default Sunday 01:00 local). Step order mirrors `references/scripts/run-weekly-catchup.sh`:
  `retry-crawls → weekly-synthesis → index → generate-signal → audit-links → enrich-topics → profile-entities`

Marker files (`_meta/.daily-last-run`, `_meta/.weekly-last-run`) gate re-execution within the same period. A second tick within the period skips the chain with an INFO log message.

The same step lists are exposed as one-shot CLI subcommands (`sea-noise run inbox|daily|weekly`) for developer / debug use. Both paths call into the same `internal/pipeline/` package. The daemon path is the production path; the CLI path is for working on a single chain interactively.

A `--dry-run` flag on either path logs every step that would run without executing any of them.

Processing logs (`_meta/processing-logs/YYYY-MM-DD.md`) record step name, start time, duration, and exit status for each run.

## IPC surface

Unix domain socket at the path from `sea-noise.toml` (default `~/.sea-noise/daemon.sock`). JSON-RPC dispatch using `encoding/json` and the standard library's net package; no external RPC framework, no HTTP server.

### Verbs

| Verb | Purpose |
|------|---------|
| `skill.run` | Execute a named skill from `.agents/shared/skills/inference/` via the skill executor + transport. Caller passes skill name and arguments; daemon returns the skill output. |
| `pipeline.trigger` | Fire one of `inbox`, `daily`, `weekly` immediately, bypassing the scheduler. Honors marker files unless `force: true` is passed. |
| `vault.query` | Run a retrieval query against the SQLite index. Caller passes query string, mode (FTS / BM25 / graph), and limits. |
| `index.refresh` | Re-index the vault. Caller may pass a glob to limit the refresh scope. |
| `status` | Report daemon health: PID, uptime, last daily run, last weekly run, watcher state, queued pipeline triggers. |

All verbs accept and return JSON objects with deterministic schemas.

### Neutrality

The IPC surface is harness-agnostic. The daemon does not know which agent or CLI is calling. There is no per-agent configuration, no harness-specific behavior, no negotiation. Any process that can connect to a Unix socket and speak JSON-RPC drives the daemon identically.

## Filesystem watcher

The daemon watches the vault path via `fsnotify` for created, modified, and removed events.

- **New note in `inbox/`** — daemon triggers the inbox-process skill on the new file. This maintains `_meta/topic-frequencies.md`, `_meta/rejected-connections.md`, and `atlas/*` MOCs. Downstream consumers (`generate-signal`, `enrich-topics`, `export-research-packs`) see fresh inputs without waiting for the daily chain.
- **Modified note anywhere** — daemon re-indexes the affected note in the SQLite FTS index. Searches stay current without a manual `sea-noise index` run.
- **Removed note** — daemon removes the corresponding row from the FTS index.

Events debounce at a configurable interval (default 500ms) to absorb the burst of writes that editors emit during a single save. The watcher depends only on `github.com/fsnotify/fsnotify`; no OS-specific syscall imports.

## Configuration

`sea-noise.toml` minimal example:

```toml
[vault]
path = "/Users/<you>/iCloud/Obsidian/<vault-name>"

[daemon]
socket = "~/.sea-noise/daemon.sock"
daily = "00:05"
weekly = { day = "sunday", time = "01:00" }
watcher_debounce_ms = 500
shutdown_grace_seconds = 30
```

`MIND_PALACE_VAULT` environment variable overrides `vault.path` if set, for compatibility with the prototype's environment.

## What the daemon does not do

- Does not register itself with launchd.
- Does not write cron entries.
- Does not auto-start on system boot.
- Does not bind a TCP port. The IPC is local-only via Unix domain socket.
- Does not call out to remote services beyond what individual workflows do (Todoist sync, YouTube transcript fetch, Cloudflare crawl).
- Does not know about Claude Code, Claude CLI, or any specific agent harness. The IPC is neutral.
