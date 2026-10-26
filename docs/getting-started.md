---
type: guide
status: stable
sources:
  - resource: README.md
  - resource: sea-noise.toml.example
  - resource: scripts/ci.sh
  - resource: cmd/sea-noise/main.go
---
# Getting started

Build the single binary, point it at a Markdown vault, and run one read-only
command before any command that writes vault files.

## Requirements

- Go 1.26 or newer (`CGO_ENABLED=0` for the shipped binary).
- A Markdown vault directory (kept outside this repository).
- An inference CLI on `PATH` for AI-backed commands.

Cloudflare and Todoist credentials are optional; commands that need them fail
clearly or fall back to local-only behavior. Source: `README.md`.

## Build

```sh
go build -trimpath -o sea-noise ./cmd/sea-noise
./sea-noise --version
```

For a full repository build-and-test pass, run `scripts/ci.sh`.
The binary is self-contained: it does not invoke Python and does not need the
checkout once built.

## Configure a vault

```sh
cp sea-noise.toml.example sea-noise.toml
# set vault.path to an existing Markdown vault
```

`vault.path` can also come from the `MIND_PALACE_VAULT` environment variable;
the environment variable wins when both are set. Neither the config file nor the
vault belongs in this repository. Full precedence and credentials live in
[Configuration and integrations](configuration.md).

## First commands

Build the index before using search, unless a previously-built index already
exists:

```sh
./sea-noise index                  # build the SQLite retrieval index
./sea-noise search "some phrase"   # read-only; queries that index
```

Commands dispatch from `cmd/sea-noise/main.go`. Many
subcommands write vault files — see the write column in
[CLI commands and workflows](cli.md). When learning a new write path, prefer a
copy of the vault and a report-producing or `--dry-run` invocation.
