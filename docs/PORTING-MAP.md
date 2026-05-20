# Sea Noise Porting Map

Date: 2026-05-16

## Purpose

This document answers a narrow migration question:

> What, if anything, needs to be ported from the current iCloud vault/repo into the fresh `sea-noise` runtime repo?

The answer may be:
- copy nothing and rebuild from docs
- copy a few reference files
- copy selected prototypes as temporary scaffolding

This map assumes:
- the iCloud Obsidian directory remains the live vault
- `sea-noise` is the new runtime repo
- files can be copied manually later only if they earn their keep

## Default Rule

Start from:

> Port as little as possible.

The burden of proof is on copying.

A file or folder should move only if it is one of:
- canonical reference material
- active implementation scaffold that saves real time
- irreplaceable prompt/spec content

Everything else should stay in the vault and be treated as:
- content
- historical implementation
- runtime exhaust

## High-Level Split

## Keep in the Vault

These are part of the live `Sea Noise` dataset, not the new runtime repo:

- `inbox/`
- `notes/`
- `atlas/`
- `bloom/`
- `signal/`
- `daily/`
- `plunge/`
- `templates/` if they are user-facing content templates rather than runtime templates
- `.obsidian/`
- `HOME.md`
- `HOME.canvas`
- content artifacts in `_meta/` that represent operational history

These should not be copied as source code.

## Candidate Runtime Reference Material

These are the current best candidates to copy into `sea-noise`:

- `.agents/go-rewrite-plan.md`
- `.agents/product-vision.md`
- `.agents/system-architecture.md`
- `.agents/skills/SKILL_MANIFEST.json`
- `.agents/skills/*/SKILL.md`
- `.claude/CLAUDE.md`
- `.claude/commands/reflect.md`
- `.claude/commands/research.md`
- `.claude/commands/signal.md`

These are valuable because they encode:
- architecture
- product thesis
- pipeline specs
- prompt and synthesis behavior
- command surfaces

This material is the most likely "must port" set.

## Historical Implementation Material

These are useful references, but should not be copied automatically:

- `_scripts/`
- `_rag/`
- LaunchAgent/plist assumptions described in docs

Treat them as:
- old implementation references
- source material for selective rewrite
- lookup targets when rebuilding a specific command

Do not import them wholesale unless there is a specific reason.

## Folder-by-Folder Mapping

## `inbox/`

Role:
- live capture input

Action:
- do not copy
- runtime should target this folder externally through config

## `notes/`

Role:
- primary knowledge corpus

Action:
- do not copy
- runtime reads and mutates this externally

## `atlas/`

Role:
- structural orientation / MOCs

Action:
- do not copy
- runtime reads and updates externally

## `bloom/`

Role:
- weekly synthesis
- provocations
- reflect companion pieces
- assertion tracker

Action:
- do not copy
- runtime reads and writes externally

## `signal/`

Role:
- generated read queue surface

Action:
- do not copy
- runtime should regenerate externally

## `daily/`

Role:
- daily synthesis artifacts

Action:
- do not copy
- runtime reads/writes externally

## `plunge/`

Role:
- emerging product surface / stochastic reflection mode

Action:
- do not copy as code
- keep as product/content surface in vault
- reference conceptually when implementing later

## `_meta/`

Role:
- operational metadata
- retry manifests
- topic frequencies
- logs
- discarded items
- health surfaces

Action:
- do not copy wholesale
- preserve externally in the vault
- selectively model the structures in the new runtime

Important sub-items to preserve conceptually:
- `_meta/.daily-last-run`
- `_meta/.weekly-last-run`
- `_meta/crawl-failures.json`
- `_meta/topic-frequencies.md`
- `_meta/pipeline-health.md`
- `_meta/rejected-connections.md`

These are runtime data formats, not repo code.

## `_scripts/`

Role:
- current Python/shell implementation

Action:
- do not copy wholesale
- use as lookup only

Port selectively by command.

Recommended treatment:
- when implementing `signal`, inspect only `generate-signal.py`
- when implementing hydration, inspect only `fetch-stub-content.py`
- when implementing topic enrichment, inspect only `enrich-topics.py`

This avoids dragging old structure into the new repo.

## `_rag/`

Role:
- current search/indexing implementation

Action:
- do not copy by default

Reason:
- search backend is not the first migration priority
- current RAG is implementation detail, not product essence
- new runtime can reintroduce search later with a fresh design

Possible exceptions:
- copy only if specific query/result behavior needs to be preserved exactly

## `.agents/`

Role:
- architecture and rewrite memory
- skills manifest
- planning documents

Action:
- selectively copy

Priority copy set:
- `go-rewrite-plan.md`
- `product-vision.md`
- `system-architecture.md`
- `skills/SKILL_MANIFEST.json`
- `skills/*/SKILL.md`

Low-priority copy:
- transient planning artifacts
- stale scratchpads
- generated analyses that no longer shape execution

## `.claude/CLAUDE.md`

Role:
- canonical behavioral spec for triage and synthesis

Action:
- copy

Why:
- this is one of the most important behavior specs in the current system
- it defines how the product actually thinks in operation

Treat it as canonical source material, even if later rewritten into more structured forms.

## `.claude/commands/`

Role:
- explicit command surfaces for `reflect`, `research`, `signal`

Action:
- copy

Why:
- these are product surfaces, not merely implementation details
- especially `reflect`, which is central to the product identity

## `AGENTS.md`

Role:
- concise project overview and system framing

Action:
- optional copy

Why:
- useful as orientation
- but much of its value is already contained in the architecture docs

If copied, treat it as a lightweight index rather than the main source of truth.

## `ALERT.md`

Role:
- vault-facing health display

Action:
- do not copy as runtime source

Can be referenced later if the runtime needs to reproduce the health surface.

## What Probably Should Be Copied First

If doing a minimal manual import into `sea-noise`, copy only:

1. `.agents/go-rewrite-plan.md`
2. `.agents/product-vision.md`
3. `.agents/system-architecture.md`
4. `.agents/skills/SKILL_MANIFEST.json`
5. `.agents/skills/`
6. `.claude/CLAUDE.md`
7. `.claude/commands/reflect.md`
8. `.claude/commands/research.md`
9. `.claude/commands/signal.md`

That is the highest-signal reference corpus.

## What Should Explicitly Not Be Copied Yet

- `inbox/`
- `notes/`
- `atlas/`
- `bloom/`
- `signal/`
- `daily/`
- `_meta/`
- `_rag/`
- all of `_scripts/`

Those should remain external runtime targets or old implementation references.

## If the Next Agent Uses Tempor

This document should be enough to support a targeted lookup workflow:

1. choose one command to rebuild
2. inspect the matching old script only
3. inspect the matching skill spec only
4. inspect `.claude/CLAUDE.md` only if behavior depends on prompt logic
5. avoid broad copying

That is the intended use.

## Recommended Immediate Follow-Up

Create a simple import checklist in the new repo:

- `reference/architecture/`
- `reference/skills/`
- `reference/prompts/`

And copy only the canonical files above into those folders.

Everything else stays in place and is consulted on demand.

## Bottom Line

The likely correct answer is:

> Port almost none of the old repo physically.

Keep the vault where it is.
Copy only the canonical docs and prompt/spec material.
Treat the old scripts as lookup references while rebuilding commands one at a time.

That is the cleanest migration shape.
