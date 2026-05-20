# Sea Noise

Fresh rewrite workspace for the `Sea Noise` runtime.

## Current stance

- This repository is the runtime, not the vault.
- The existing iCloud Obsidian directory remains the live vault.
- Migration should begin by replacing the current operational loop, not by importing the vault into this repo.

## Immediate goals

1. Establish the rewrite as a clean Go project outside the vault.
2. Preserve the current vault path as an external runtime target.
3. Rebuild only the minimum daily dogfood loop first.
4. Avoid premature productization, packaging, or ecosystem glue.

## Docs

- [Migration Plan](./docs/MIGRATION.md)
- [Studio One-Pager](./docs/README.md)
- [Go Rewrite Re-evaluation](./docs/2026-05-16-sea-noise-go-rewrite-reevaluation.md)
- [Language and Product Shape](./docs/2026-05-16-sea-noise-language-and-product-shape.md)

## Working assumption

The right first move is:

- keep `/Users/kd-m2air/Library/Mobile Documents/iCloud~md~obsidian/Documents/mind-palace` as the live vault
- build `Sea Noise` here as a separate runtime
- point the runtime at that vault path via config

That preserves the original product boundary:

runtime here, vault there.

## Build, run, test

The runtime is pure Go, CGO disabled, single static binary.

```sh
./scripts/build.sh          # CGO_ENABLED=0 go build ./...
./scripts/test.sh           # go test ./...
./scripts/grep-gates.sh     # enforce architectural invariants
```

Then point the binary at a vault:

```sh
# Either set an env var:
MIND_PALACE_VAULT=/path/to/vault ./sea-noise --help

# Or copy mind-palace.toml.example to mind-palace.toml and edit it:
cp mind-palace.toml.example mind-palace.toml
# uncomment and set vault.path = "/path/to/vault"
```

The env var wins if both are set.

## Architectural invariants (enforced by `scripts/grep-gates.sh`)

- No provider/model name string literals in Go source outside `internal/config` and tests.
- No Python subprocess invocations.
- No MCP server / transport imports.

See [`ARCHITECTURE.md`](./ARCHITECTURE.md) for the substrate decision, package layout, and seam contracts.
