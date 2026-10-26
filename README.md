# Sea Noise

Sea Noise is a local-first Go runtime for retrieval, context assembly, and small automation pipelines over a Markdown vault.

The repository contains executable code, public documentation, and synthetic test data only. A user's vault remains outside the repository.

## Status

The current release target is an installable, single-binary baseline. The runtime is still evolving; commands that write vault files should be tested against a disposable vault before being used with important data.

## Requirements

- Go 1.26 or newer
- CGO disabled for the shipped binary
- A Markdown vault directory
- An inference CLI on `PATH` for AI-backed commands
- Python 3.12+ only for optional parity-fixture capture
- Node.js only for repository hook tests

Cloudflare and Todoist credentials are optional integrations. Commands that need them fail clearly or fall back to local-only behavior as documented by their command help.

## Build and install

Build from source:

```sh
git clone https://github.com/grosspoetrysystems/sea-noise.git
cd sea-noise
go build -trimpath -o sea-noise ./cmd/sea-noise
./sea-noise --version
```

For a repository-wide build and test pass:

```sh
./scripts/ci.sh
```

The binary is self-contained. It does not invoke Python and does not require a checkout after it has been built.

## Configure a vault

Copy the example configuration and set the path to an existing Markdown vault:

```sh
cp sea-noise.toml.example sea-noise.toml
# edit vault.path
```

`vault.path` may also be supplied through the `MIND_PALACE_VAULT` environment variable for compatibility with earlier Sea Noise development. The environment variable wins when both sources are set. Neither the configuration file nor the vault belongs in this repository.

## Commands

Run `./sea-noise --help` for the current flags. The main surfaces are:

| Command | Effect | Writes vault files |
| --- | --- | --- |
| `audit-links` | Report broken wikilinks | Yes |
| `index` | Rebuild the SQLite retrieval index | Yes, outside the vault by default |
| `search` | Query the retrieval index | No |
| `generate-signal` | Build the signal queue | Yes |
| `summarize-transcripts` | Add research context to transcript notes | Yes |
| `profile-entities` | Create entity profiles from repeated mentions | Yes |
| `enrich-topics` | Create topic research briefs | Yes |
| `youtube` | Hydrate YouTube transcript notes | Yes |
| `fetch` | Hydrate inbox URL stubs | Yes |
| `process-inbox` | Classify and promote inbox notes | Yes |
| `sync todoist` | Push or pull Todoist tasks | Yes and external API |
| `run` | Run one named pipeline | Usually |
| `serve` | Run scheduled pipelines as a resident process | Usually |

Prefer a copy of the vault and a dry-run or report-producing command when learning a new write path.

## Credentials and integrations

- `CF_ACCOUNT_ID` and `CF_API_TOKEN`: optional Cloudflare Browser Rendering crawl access for richer topic and stub context.
- `TODOIST_API_TOKEN`: required only for Todoist synchronization.
- `TODOIST_INBOX_ID`, `TODOIST_WORK_ID`, and `TODOIST_PERSONAL_ID`: optional Todoist routing overrides.
- The configured inference CLI may have its own authentication and permission model. Sea Noise passes through the command configured for the local runtime; review that command before enabling scheduled writes.

Do not put credentials in `sea-noise.toml`, the vault, fixtures, issue reports, or commits.

## Development checks

```sh
./scripts/build.sh
./scripts/test.sh
./scripts/grep-gates.sh
./scripts/lint.sh                 # optional full report
node --test .agents/shared/hooks/*.test.mjs .claude/shared/hooks/*.test.mjs
```

The fixture vault under `testdata/fixture-vault/` is synthetic. It contains no copy of a user's notes. Parity captures and their regeneration procedure are documented in [`testdata/CAPTURING-FIXTURES.md`](testdata/CAPTURING-FIXTURES.md).

## Architecture

- [`ARCHITECTURE.md`](ARCHITECTURE.md) — package boundaries and runtime contracts
- [`DAEMON.md`](DAEMON.md) — scheduled pipeline behavior
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — contribution and verification expectations
- [`SECURITY.md`](SECURITY.md) — private vulnerability reporting
- [`RELEASING.md`](RELEASING.md) — version and tag procedure

## License

MIT. See [`LICENSE`](LICENSE).
