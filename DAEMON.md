# Daemon

Sea Noise ships as a resident background daemon. This document is the contract: lifecycle, IPC surface, filesystem watcher, and how users start the daemon without delegating that decision to the binary.

## Why a daemon

The original prototype relied on external schedulers and multiple scripts. Sea Noise keeps scheduling in one static binary so the runtime has one observable lifecycle and one place to configure retries, markers, and shutdown.

The daemon owns the schedule internally. It owns the vault-change reaction. It owns the dispatch surface that any agent can call into. There is no separate scheduler, no install step, no plist generation.

## Lifecycle

```sh
sea-noise serve
```

Starts the resident daemon. Runs indefinitely until killed (`SIGINT`, `SIGTERM`) or
the process exits. Clean shutdown stops the watcher and IPC listener, then waits
for in-flight pipeline steps to finish before exiting. There is no configurable
shutdown grace period or rollback of vault writes.

A second `sea-noise serve` against the same socket path refuses to start and
exits non-zero with a message identifying the existing process. There is no
`--force` flag.

The configuration file currently supports only `vault.path`. Socket, dry-run,
watcher, inference, and index settings are command-line flags; `MIND_PALACE_VAULT`
overrides `vault.path`.

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

There is intentionally no `sea-noise schedule install` subcommand and no auto-generated launchd plist. Registering with launchd would reintroduce the exact TCC and approval churn this design escapes.

## Internal scheduler

- **Inbox** — fires every 30 minutes at minute `00` and `30`; if the
  daemon wakes after a boundary, the next scheduler check reconciles the
  current inbox. Startup watcher scans also catch notes already present.
  Step order: `hydrate-stubs → process-inbox → youtube-transcript`.

- **Daily** — fires at `00:05 UTC`; a startup or post-sleep check catches
  the current missed window:
  `rotate-logs → retry-crawls → summarize → index → daily-synthesis (deferred) → sync-consumed (deferred) → signal → plunge (deferred) → home (deferred) → todoist-push → todoist-pull`.

- **Weekly** — fires on Sunday at `01:00 UTC`; startup or post-sleep
  checks catch the current missed window:
  `retry-crawls → weekly-synthesis (deferred) → index → signal → audit-links → enrich-topics → profile-entities`.

Marker files (`_meta/.daily-last-run`, `_meta/.weekly-last-run`) gate
re-execution within the same period. A second tick within the period skips the
chain.

The same step lists are exposed as one-shot CLI subcommands
(`sea-noise run --dry-run inbox`, `sea-noise run daily`) for interactive
operation. Both paths call into the same daemon registry.

A `--dry-run` flag logs every step that would run without invoking any step
function. It still appends a processing log.

Processing logs (`_meta/processing-logs/YYYY-MM-DD.md`) record step name,
start time, duration, and exit status for each run.

## Filesystem watcher

The daemon watches the vault path via `fsnotify` for new inbox files.

- **New note in `inbox/`** — triggers the inbox pipeline.
- **Modified or removed notes** — no per-note retrieval update is currently
  wired; run `sea-noise index` to rebuild the full index.

Events debounce at the configured interval (default 200ms). The watcher scans
existing `inbox/*.md` files when it starts.

## Configuration

The configuration file currently supports only the vault path:

```toml
[vault]
path = "/absolute/path/to/vault"
```

The runtime accepts these command-line controls:

- `serve --socket PATH` or `SEA_NOISE_SOCKET` for IPC;
- `serve --no-ipc` to disable IPC;
- `serve --no-watch` to disable the filesystem watcher;
- `serve --watch-debounce-ms N` to set watcher debounce;
- `serve --dry-run` to log steps without invoking them;
- `serve --index-db PATH` to select the retrieval index;
- `serve --cli-command NAME` and `serve --cli-args ...` for inference.

`MIND_PALACE_VAULT` overrides `vault.path` if set. Paths must be absolute
existing directories.

## IPC surface

Unix domain socket at `--socket PATH`, then `$SEA_NOISE_SOCKET`, then
`~/.sea-noise/daemon.sock`. Use `--no-ipc` to disable the listener. JSON-RPC
dispatch uses `encoding/json` and the standard library's `net` package.

### Verbs

| Verb | Purpose |
|------|---------|
| `skill.run` | Execute a named embedded inference skill through the configured transport. |
| `pipeline.trigger` | Fire `inbox`, `daily`, or `weekly`; marker gating applies. `force` is reserved and currently ignored. |
| `vault.query` | Run a retrieval query against the SQLite index. |
| `index.refresh` | Re-index the entire vault. |
| `status` | Report protocol version, vault root, uptime, and dry-run state. |

All verbs accept and return newline-delimited JSON objects with deterministic
schemas. Any same-user process that can open the socket can call the verbs.


### Neutrality

The IPC surface is harness-agnostic. The daemon does not know which agent or CLI is calling. There is no per-agent configuration, no harness-specific behavior, no negotiation. Any process that can connect to a Unix socket and speak JSON-RPC drives the daemon identically.


## What the daemon does not do

- Does not register itself with launchd.
- Does not write cron entries.
- Does not auto-start on system boot.
- Does not bind a TCP port. The IPC is local-only via Unix domain socket.
- Does not call out to remote services beyond what individual workflows do (Todoist sync, YouTube transcript fetch, Cloudflare crawl).
- Does not know about Claude Code, Claude CLI, or any specific agent harness. The IPC is neutral.
