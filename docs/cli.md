---
type: reference
status: stable
sources:
  - resource: README.md
  - resource: cmd/sea-noise/main.go
  - resource: cmd/sea-noise/run.go
  - resource: cmd/sea-noise/index.go
  - resource: cmd/sea-noise/search.go
  - resource: cmd/sea-noise/vernacular.go
  - resource: cmd/sea-noise/serve.go
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
| `vernacular link` / `unlink` / `list` | Manage selected private projects under vault-root `vernacular/`. | Links, navigation and bridge metadata; may create a private overview. `list` is read-only. |
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

## Link a Vernacular project

Use a persistent `vernacular/` directory at the vault root, not a folder inside
`inbox/`. Each selected project keeps its editable originals in its existing
Vernacular workspace. The code checkout is not imported or indexed through the
bridge.

Prerequisites: configure the vault, initialize the chosen project's Vernacular
workspace, and put `vern` on `PATH`. Sea Noise resolves the selected checkout's
public `vern path canon`, `apparatus`, and `working` paths; it does not read a
private Vernacular registry.

```sh
sea-noise vernacular link --key helicopter --project /absolute/path/to/helicopter
sea-noise vernacular list
sea-noise index
sea-noise search "review"
```

| Vault path | Meaning |
| --- | --- |
| `vernacular/README.md` | Orientation for the persistent project wing |
| `vernacular/helicopter/README.md` | Navigation and current project/workspace paths |
| `vernacular/helicopter/overview.md` | Alias to the private project overview |
| `vernacular/helicopter/apparatus/` | Live link to durable private project material |
| `vernacular/helicopter/working/` | Live link to the project's working notes |

Setup reuses an existing private overview or README. If none exists, it creates
a minimal private overview for you to edit, not an inferred project summary.
Repeating the same enrollment is safe. Keep the project key stable: note IDs
and qualified wikilinks use it, for example
`[[vernacular/helicopter/overview]]`. Sea Noise indexes an original overview
once under that logical ID even when another linked path exposes the same file.

With `serve` running, edits, new target directories and enrollment changes
refresh the derived index automatically. Without it, run `sea-noise index`
after changing notes or membership. A project's own `working/inbox/` is still
project scratch; it does not dispatch the vault's inbox pipeline.

Default deterministic workflow scans skip the wing. The inbox inference skill
is also instructed to leave it alone, but a configured inference executable is
trusted code, not a filesystem sandbox. Explicit single-note operations such as
`enrich-topics --note PATH` can write through a supplied project path.

Retrieval resolves qualified project links. The prototype-parity `audit-links`
report still compares bare note stems and can report a valid qualified link as
broken; it is not a validator for the indexed project graph.

To remove a project from the wing:

```sh
sea-noise vernacular unlink helicopter
```

`unlink` removes owned bridge links and metadata, not project originals.
User-edited navigation is preserved. This is different from deleting a note
inside a linked folder: normal edits and deletions there affect the original.
Vault sync and backup tools may not follow external symlinks; back up the
Vernacular originals separately.

Invalid or missing targets are reported rather than silently redirected.
Restore the target, or unlink and enroll the correct checkout again. See
[daemon failure boundaries](daemon.md#filesystem-watcher) for suspended watches
and stale-cache behavior.

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
