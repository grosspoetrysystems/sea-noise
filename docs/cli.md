---
type: reference
status: stable
sources:
  - resource: README.md
  - resource: cmd/sea-noise/main.go
  - resource: cmd/sea-noise/run.go
  - resource: cmd/sea-noise/index.go
  - resource: cmd/sea-noise/search.go
  - resource: ROUTER-SEAM.md
---
# CLI commands and workflows

The binary entry point is `cmd/sea-noise/main.go`.
Run `sea-noise --help` for the current usage text; `--version` prints the
binary version. The vault path must resolve before commands that need a vault
can run.

## Command map

| Command | Purpose | Writes outside the repository? |
| --- | --- | --- |
| `audit-links` | Scan for broken `[[wikilinks]]` and write `_meta/broken-links.md`. | Yes, vault |
| `index` | Rebuild the SQLite retrieval index. `--db` can override its path. | Yes, index DB |
| `search` | Run FTS5 keyword search against the index. | No |
| `generate-signal` | Assemble the rolling queue from unconsumed notes. | Yes, vault |
| `summarize-transcripts` | Add research context to transcript notes through the configured inference CLI. | Yes, vault |
| `profile-entities` | Generate profiles from repeated mentions. | Yes, vault |
| `enrich-topics` | Write a topic research brief; Cloudflare crawl is optional. | Yes, vault |
| `youtube` | Hydrate YouTube-source notes with a transcript section. | Yes, vault/network |
| `fetch` | Hydrate inbox URL stubs; `--retry` retries eligible failed crawls. | Yes, vault/network |
| `process-inbox` | Triage inbox notes and update topic/atlas outputs. | Yes, vault |
| `sync todoist` | Push todo notes, or pull completions with `--completions`. | Yes, vault/external API |
| `run inbox|daily|weekly` | Fire one pipeline once and exit. | Usually, vault |
| `serve` | Run the resident scheduler, IPC server, and watcher. | Usually, vault |

Use a disposable vault while learning a write path. Prefer report-producing
commands or `--dry-run` where supported. The runtime does not promise that all
commands are read-only merely because they print a summary.

## Safe retrieval workflow

1. Configure and validate the vault path (see [Configuration](configuration.md)).
2. Build or refresh the derived index:

   ```sh
   sea-noise index
   ```

3. Query it:

   ```sh
   sea-noise search "phrase to find"
   ```

`index` builds the derived SQLite index directly by migrating and populating it;
`search` opens that index and queries it through the retrieval router.
Prototype-derived commands keep the direct vault-walk behavior documented in
`ROUTER-SEAM.md` for parity.

## One-shot pipelines

`run` requires one of `inbox`, `daily`, or `weekly`; it uses the same step lists
as the daemon. Only successful non-dry-run `daily` and `weekly` runs write
period markers (`_meta/.daily-last-run` and `_meta/.weekly-last-run`); `inbox`
has no marker. `--force` clears one of those daily/weekly markers before the
run and has no effect for `inbox`. `--dry-run` skips pipeline step functions and
marker writes, but still creates or appends
`_meta/processing-logs/YYYY-MM-DD.md`.

Flags must precede the pipeline name; arguments after it are not parsed as flags:

```sh
sea-noise run --dry-run daily
```

Inference-related options include `--cli-command`, `--cli-args`, `--skills-dir`,
and `--timeout`; `--index-db` selects the retrieval database. The default
`--cli-args` value is `-p,{PROMPT},--dangerously-skip-permissions` (the same
default is used by `serve`). This passes a flag that disables the configured
inference CLI's permission gate; it does not grant OS-level permissions.
Review or override `--cli-args` before enabling vault writes or scheduled
execution.

## External dependencies and failures

- Inference steps are optional until a `--cli-command` is configured; inspect
  the command and its permissions before enabling writes.
- Cloudflare crawl enrichment needs `CF_ACCOUNT_ID` and `CF_API_TOKEN`; without
  them, topic enrichment has a vault-only path.
- Todoist synchronization needs `TODOIST_API_TOKEN`; optional inbox/work/personal
  IDs customize routing.
- YouTube and URL hydration depend on network access and can leave retry records.

Missing vault state is a hard configuration error. Optional credentials are
soft dependencies for their specific workflows, not a reason to put secrets in
this repository or in the vault.
