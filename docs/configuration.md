---
type: guide
status: stable
sources:
  - resource: README.md
  - resource: sea-noise.toml.example
  - resource: internal/config/config.go
  - resource: cmd/sea-noise/serve.go
  - resource: internal/todoist/todoist.go
  - resource: internal/crawl/crawl.go
---
# Configuration and integrations

Keep `sea-noise.toml` and the vault outside source control. The checked-in
`sea-noise.toml.example` is only a shape example.

## Vault path resolution

`internal/config.LoadVaultPath` resolves the vault in this order:

1. `MIND_PALACE_VAULT`, when set;
2. `vault.path` in `sea-noise.toml` loaded from the process current working
   directory. The loader does not search the repository root.

The environment variable wins when both are present. The resolved path must be
an existing directory; a missing, unreadable, malformed, or non-directory path
is a hard startup/configuration error. The minimal file is:

```toml
[vault]
path = "/path/to/an existing/markdown-vault"
```

The database path is separate. `index` and daemon flags can override it with
`--db` or `--index-db`; otherwise the runtime uses its default under
`~/.sea-noise/retrieval.db`.

## Optional integrations

| Integration | Environment / flags | Boundary |
| --- | --- | --- |
| Inference CLI | `--cli-command`, `--cli-args`, `--skills-dir`, `--timeout` | Used only by inference-backed steps; inspect the executable and permissions first. |
| Cloudflare crawl | `CF_ACCOUNT_ID`, `CF_API_TOKEN` | Enrich-topic crawling is richer when configured; vault-only behavior remains available. |
| Todoist | `TODOIST_API_TOKEN`; optional `TODOIST_INBOX_ID`, `TODOIST_WORK_ID`, `TODOIST_PERSONAL_ID` | Needed only for `sync todoist`; absent configuration must not be treated as a repository secret. |
| YouTube / URL fetch | source URLs plus network access | Hydration can fail and leave retryable records. |

By default, both `run` and `serve` pass
`-p,{PROMPT},--dangerously-skip-permissions` as `--cli-args`. This asks the
configured inference CLI to disable its permission gate; it does not grant
OS-level permissions. Review or override the value before enabling vault
writes or scheduled execution.

The daemon wires optional collaborators at startup. A missing optional
credential does not make unrelated local retrieval unavailable, but a malformed
configured integration can fail its command or daemon startup. Read the command
output and its workflow docs rather than assuming a fallback happened.

## Data and safety boundaries

The vault is user data and is not part of this repository. Write-producing
commands modify it, and Todoist synchronization also changes an external
service. Use a disposable copy while learning. Keep credentials out of
`sea-noise.toml`, vault notes, fixtures, issue reports, and commits.

The runtime has no web UI or TCP listener. `serve` exposes only local Unix
socket IPC unless disabled; see [Daemon](daemon.md). Network use comes from
individual integrations, not a general web surface.

## Honest gaps

This corpus documents the configuration keys and environment variables exposed
by the current source. It does not prescribe deployment tooling, launchd, cron,
secret-manager setup, or a required inference provider; the daemon intentionally
does not install itself or choose those policies.
