# Sea Noise Go Rewrite Re-evaluation

Date: 2026-05-16

## Context

This note re-evaluates the existing `mind-palace` Go rewrite plan in light of current studio priorities, dogfooding needs, and the clearer product framing that has since emerged around `Sea Noise`.

Primary reference documents reviewed:
- `.agents/go-rewrite-plan.md`
- `.agents/product-vision.md`
- `.agents/system-architecture.md`

## Summary

The rewrite plan is still worth keeping, but it should no longer be treated as a linear execution roadmap.

It is strongest as:
- a source inventory of the current system
- a record of the core architecture thesis
- a migration reference for deterministic pipeline behavior

It is weaker as:
- a complete implementation sequence to execute as written
- a reliable prioritization of what matters most for dogfooding now

The main issue is not that the plan is wrong. The issue is that it tries to solve too many layers at once: local dogfood tool, public product, install/distribution, search backend migration, export membrane, agent sync, and scheduler management. That was useful for thinking, but it is too broad for the current moment.

## What Still Feels Foundational

### 1. Runtime / Vault Separation

This still looks exactly right.

The strongest idea in the whole rewrite is that the runtime owns:
- business logic
- scheduling
- orchestration
- export
- sync

While the vault remains:
- markdown
- portable
- Obsidian-compatible
- cleanly separable from execution machinery

This is the correct product boundary.

### 2. Single Binary

The single Go binary still feels like the right center of gravity.

Not because "single binary" is intrinsically elegant, but because it creates the right discipline:
- deterministic shell around probabilistic behavior
- low operational burden
- local-first ergonomics
- less glue and less ambient setup complexity

### 3. Claude as Reasoning, Go as Harness

This still feels right.

Keeping Claude as the reasoning engine while moving deterministic orchestration into Go matches the current product thesis well. It preserves leverage while reducing fragility.

### 4. Audience of One / Mirror Not Assistant

The product vision remains strong and should be treated as canonical.

The important part of `Sea Noise` is not that it summarizes notes or automates intake. The important part is that it accumulates what the user has been attending to, finds structure in it, and reflects it back in a way that produces new thought.

That remains the real product.

### 5. Temporal Structure

The strongest differentiated part of the system is the rhythm:
- inbox
- daily synthesis
- weekly synthesis
- signal
- bloom
- reflect
- plunge

This temporal architecture matters more than specific implementation details like vector backend selection.

## What Now Looks Stale, Overbuilt, or Premature

### 1. The Plan Is Too Complete

The current rewrite plan tries to carry too much at once:
- rewrite all glue into Go
- replace search/indexing infrastructure
- introduce export and agent sync
- provide installer and scheduler UX
- package for public distribution
- preserve migration fidelity

That breadth made sense as an architecture capture, but it is too much for a live dogfooding roadmap.

### 2. There Is Internal Drift in the Document

The doc contains signs that it froze at an intermediate point.

Examples:
- earlier sections choose `Kong` and `koanf`
- later dependency sections still list `cobra` and `viper`

This does not invalidate the plan, but it means it should not be trusted as an execution checklist without curation.

### 3. Config Location Needs Reconciliation

The plan says `mind-palace.toml` should live outside the vault, which seems right.

But the later installer phase says `mind-palace init` writes `mind-palace.toml` to the vault root.

That should be resolved before implementation starts. The stronger choice is to keep runtime config outside the vault.

### 4. Search Is Overweighted Too Early

Search matters, but the current plan gives the vector/search rewrite more architectural attention than the core reflective loop deserves.

The product's emotional and practical differentiation is:
- synthesis
- provocation
- accumulation of judgment
- reflective feedback loops

Not embedded semantic retrieval by itself.

A simpler search baseline would likely be enough for dogfooding early.

### 5. Export + Agent Sync Are Strategically Interesting but Not Core

The export membrane is a good idea and likely still the right direction.

But export packs and agent sync are not what proves that `Sea Noise` is a great daily product for the operator. They should not sit this early in the practical build order unless they are needed for immediate dogfood loops.

### 6. Installer / Distribution / Homebrew Are Productization Layers

These are downstream concerns.

They matter once the tool is sharp enough that distribution becomes meaningful. They are not the thing that makes the product good for daily use.

## What the Rewrite Is Really About

The rewrite should not be understood primarily as:
- port Python to Go
- replace LanceDB
- wrap launchd
- make a CLI product

It should be understood as:
- preserve and strengthen the capture -> synthesis -> provocation -> reflection loop
- reduce operational drag around that loop
- make the system local, stable, portable, and composable
- create a deterministic shell for a reflective cognition tool

The product is the loop, not the rewrite.

## Recommended Reframe

The current plan should be demoted from "execution roadmap" to:
- architecture thesis
- migration reference
- source inventory

A new execution plan should be built around dogfooding leverage.

## Proposed Rewrite Plan v2

### Milestone 1: Sea Noise Core

Goal: run the existing core loop from a Go binary on the real vault.

Keep scope to:
- markdown engine
- vault abstraction
- deterministic ingest
- Claude subprocess wrapper
- inbox / daily / weekly orchestration
- signal generation
- basic logging and markers

Defer:
- vector backend redesign
- agent sync
- export packs
- installer wizard
- distribution
- scheduler UX beyond what is minimally needed

Success criterion:
- the Go runtime can replace the current daily operational loop for the real vault
- without making the product worse

### Milestone 2: Sea Noise Reflection

Goal: improve the actual product experience, not just the substrate.

Focus on:
- stronger reflect flows
- better weekly provocations
- sharper signal behavior
- better assertion tracking
- real plunge behavior
- better visibility into what the system is surfacing and why

This is where the product becomes more alive, not just more rewritten.

Success criterion:
- the operator prefers living inside the new loop
- not just because it is cleaner technically, but because it produces better thinking

### Milestone 3: Sea Noise Extraction

Goal: make the tool cleanly installable and more portable after the loop is already excellent.

Then bring back:
- installer
- scheduler management
- public packaging
- broader runtime config UX
- export membrane if it is paying rent
- search upgrades if clearly needed

Success criterion:
- the tool is good enough that distribution is desirable
- not merely possible

## Recommended Near-Term Priorities

If work resumes now, the first priorities should be:

1. Re-scope the rewrite around the minimum daily dogfood loop.
2. Resolve architectural contradictions in the doc.
3. Define the smallest viable Go runtime that can replace the current shell/Python orchestration for real usage.
4. Keep search simple at first.
5. Focus product energy on reflection, bloom, signal, and plunge rather than packaging.

## Specific Keeps

These parts of the existing plan should survive with minimal change:
- runtime / vault split
- single binary harness
- Claude stays Claude
- local-first config and execution
- markdown as primary authoring surface
- export membrane as a future interface boundary
- dogfood-first milestone sequencing

## Specific Cuts or Deferrals

These should be cut from the first active implementation pass or explicitly deferred:
- full search/backend redesign as a first-class milestone
- early Homebrew/distribution work
- installer polish before loop quality
- full agent sync before daily product sharpness
- treating script parity as the primary success metric

## Main Risk to Avoid

The main risk is rebuilding the current machine too faithfully.

The current system is valuable because it has real rhythms and real output, not because every current behavior deserves preservation. Some of the system should survive because it is good. Some only survives because it exists.

The rewrite should sharpen the product, not merely compress the implementation.

## Final Judgment

The existing rewrite plan is still valuable and should not be discarded.

But it should be used differently now:
- preserve it as architecture memory
- mine it for constraints and migration details
- extract the still-correct principles
- write a smaller execution plan centered on dogfooding leverage

The key question is no longer:

> How do we finish the rewrite plan?

The better question is:

> What is the minimum Go rewrite that lets us live inside Sea Noise every day and feel the product getting sharper?

That is the version of the rewrite most worth doing.
