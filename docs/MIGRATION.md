# Sea Noise Migration Plan

Date: 2026-05-16

## Decision

Treat this rewrite as a fresh migration, not a transplant.

That means:

- do not move the existing iCloud vault into this repository
- do not copy `_scripts/`, `_rag/`, or other runtime machinery into the vault-facing repo
- keep the current vault as the live dataset
- build the new runtime here and target the existing vault path

## Why

The product boundary is clearer this way.

- The vault is content.
- This repo is runtime.
- The rewrite should prove it can operate against the live vault without entangling itself with the vault’s historical implementation details.

This also avoids needing special auth or invasive migration steps just to begin.

## What does not need to move

The following should stay where they are for now:

- the iCloud Obsidian vault
- current notes and folder structure
- current captures and daily/weekly artifacts
- current Shortcuts-based iOS ingress

The rewrite can point at the vault path directly by config.

## What should move conceptually

The following should be reimplemented here over time:

- vault abstraction
- markdown/frontmatter manipulation
- deterministic ingest/hydrate logic
- Claude subprocess integration
- daily/weekly orchestration
- signal generation
- logging and marker handling

## First migration target

The first meaningful milestone is not “port everything.”

It is:

> run the minimum daily dogfood loop from this repo against the live vault without degrading the current experience.

Concretely, that means a first usable cut should support:

1. reading config with a vault path
2. walking the vault safely
3. reading and writing markdown/frontmatter
4. running one deterministic command against the live vault
5. logging its work

## Recommended implementation order

1. Project scaffold
2. Config with external vault path
3. Markdown / frontmatter engine
4. Vault read/write abstraction
5. Deterministic command port
6. Claude wrapper
7. Inbox / daily / weekly orchestration

## Suggested first deterministic command

Start with one of these:

- `signal`
- `audit links`
- `fetch --retry`

Best first choice:

- `signal`

Why:

- it is useful every day
- it is deterministic
- it is fast
- it touches core product value
- it is less risky than hydration or vault mutation-heavy promotion logic

## Config direction

Use a runtime config outside the vault.

Suggested shape:

```toml
[vault]
path = "/Users/kd-m2air/Library/Mobile Documents/iCloud~md~obsidian/Documents/mind-palace"

[claude]
command = "claude"
timeout = "5m"
```

The config should live with the runtime, not in the vault.

## Auth / access question

You likely do not need any new “auth” for the migration itself if:

- the runtime reads and writes the vault path locally
- model/API credentials come from environment or a runtime config outside the vault

The main decision is operational, not authentication-related:

- whether the runtime is allowed to mutate the live vault immediately
- or whether early commands should run in dry-run / report-only mode

Recommended:

- deterministic commands can operate directly once trusted
- anything that mutates note structure significantly should start with dry-run support

## Near-term repo goal

This repo should become the clean runtime workspace for `Sea Noise`.

That means the next useful artifact is probably:

- a small Go module scaffold
- plus a first command that can be pointed at the live vault

Not:

- package distribution
- a mobile app
- search backend redesign
- public OSS polish

## Success criterion

This migration is on track when:

- the runtime here can act on the live vault
- the vault remains external and clean
- the operator can begin replacing one real part of the old loop with the new one

That is the real start of the rewrite.
