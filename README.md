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
