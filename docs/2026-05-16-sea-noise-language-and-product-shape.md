# Sea Noise Language and Product Shape Assessment

Date: 2026-05-16

## Context

This note captures the current assessment of:
- Go vs TypeScript vs hybrid for `Sea Noise`
- how `Sea Noise` fits relative to `Tempor`, `Steel Cars`, and `Chops`
- whether iOS should be prioritized now
- how the companion product / monetization shape should work

This is written after recognizing that some earlier language decisions were influenced by the period where `chops`, `ZeroClaw`, and larger orchestration ambitions were taking up more conceptual space.

## Summary

`Sea Noise` should still be Go for the core.

Not because the studio should standardize on Go.
Not because it needs to match `chops`.
Not because orchestration requires it.

It should be Go because the actual product is:
- local-first
- file-heavy
- deterministic
- scheduler-oriented
- subprocess-oriented
- installable as a single trusted tool
- meant to disappear into daily infrastructure

That is a Go-shaped product.

The rationale should change from:

> Sea Noise is Go because the ecosystem is becoming orchestration-heavy.

To:

> Sea Noise is Go because it is a personal cognition appliance with a deterministic shell.

## Go vs TypeScript vs Hybrid

## Go for the Core

Go should own:
- vault abstraction
- markdown/frontmatter manipulation
- deterministic ingest
- pipeline orchestration
- scheduler integration
- local config/runtime
- export and membrane logic
- HTTP clients
- subprocess orchestration for Claude or other model backends

Why:
- single-binary distribution
- stable local tooling
- low maintenance burden
- strong fit for file/process/network orchestration
- better long-term "boring infrastructure" posture

## TypeScript for Adjacent Intelligence Infrastructure

TypeScript is still the right home for parts of the stack that already have gravity there:
- `Tempor`
- `Steel Cars`
- routing experiments
- model inference policy logic
- evals
- MCP- and SDK-adjacent tooling

TS is especially good for:
- rapid model/tooling experimentation
- fast iteration on prompt-adjacent logic
- code sharing with existing TS systems
- richer MCP/web/SDK ecosystem

But TS should not own the core `Sea Noise` loop if the goal is a quiet, trusted, low-friction personal runtime.

## Hybrid Is the Right Overall Studio Pattern

The right conclusion is not Go versus TypeScript.

It is:

> Go for the runtime. TypeScript for adjacent intelligence infrastructure where it already has gravity.

That means the studio does not need one language. It needs clean boundaries.

## Router / Inference Kernel Implication

The model inference router emerging out of `Steel Cars` is probably worth extraction, but not as a TS-only product.

Because `Sea Noise` is Go, the best shape is:
- a language-neutral routing contract
- shared fixtures and expected decisions
- TS implementation for `Tempor` and `Steel Cars`
- Go implementation for `Sea Noise`

The real standalone is the routing spec and policy kernel, not "yet another TS library."

## iOS Assessment

The current iOS capture story is already doing its real job:
- share-sheet capture via Shortcuts
- voice memo routing
- Todoist handoff when an event/task is detected

That means iOS is currently functioning correctly as an ingress surface.

This strongly suggests:
- do not prioritize a native iOS app yet
- keep the mobile layer relatively dumb for now
- focus on the post-capture loop

Right now the leverage is in what happens after capture:
- triage
- synthesis
- signal
- bloom
- reflect
- plunge

The current Shortcuts setup should be replaced only when it becomes the bottleneck through:
- unreliability
- poor metadata preservation
- maintenance pain
- degraded UX around failures/retries
- inability to support new capture modes

Until then, it is correctly scoped.

## Product and Monetization Shape

The current setup suggests a very clean product split:

### 1. Sea Noise Core

Free / open source / local-first.

This is the trust anchor:
- vault logic
- reflective loop
- deterministic runtime
- personal cognition engine

It benefits from:
- inspectability
- portability
- credibility
- alignment with studio values

### 2. Sea Noise Capture

Paid iOS companion.

This is a strong candidate because it sells:
- convenience
- polish
- reliability
- ingress ergonomics

Without selling:
- captivity
- lock-in
- SaaS dependence
- proprietary control over the user's data

Users would be paying for the handle, not for ownership of the engine.

That is a very honest product boundary.

### 3. Later: Warm Interface Layer

Potentially paid, once it is real:
- more ambient review
- richer signal consumption
- reflection surfaces
- visual archive / graph / timeline / plunge interface

But only if it becomes a distinct product surface rather than ornamental wrapping.

## Recommended Positioning

The cleanest business shape is:
- open core for thought
- paid companion for capture

Or more sharply:

> The engine is free. The handle is for sale.

That is one of the strongest monetization models available here because it does not distort the core product or force cloud behavior onto a local-first system.

## Strategic Implication

The priority should remain:
1. make `Sea Noise` core excellent
2. make the reflection loop compelling
3. only then replace Shortcuts with a first-class mobile companion

The mobile companion is a strong future product, but it should not drag core development away from the thing that actually compounds judgment.

## Final Judgment

`Sea Noise` should remain Go for the core.

The reason is no longer orchestration or ecosystem alignment.
The reason is product shape:
- deterministic shell
- local cognition runtime
- trusted personal infrastructure

TypeScript should continue to own adjacent experimentation and shared intelligence infrastructure where it already has real momentum.

The iOS layer is currently "good enough" as ingress.
Its long-term value is real, but it should come after the desktop/core reflective loop is strong enough to deserve a polished paid companion.

The cleanest product line is:
- `Sea Noise Core` — FOSS engine
- `Sea Noise Capture` — paid iOS ingress companion
- optional later warm layer — paid reflective interface

That feels aligned with the studio's values and with the actual leverage points in the product.
