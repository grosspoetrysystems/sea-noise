---
name: post-council-handoff
description: Post-council execution anchor for consuming a freshly decomposed Tempor plan and turning it into a kickoff-ready execution frame. Use immediately after `tempor.decompose_plan` or any plan response that returns `execution_handoff` / lifecycle bridge metadata. This skill establishes the active phase and wave, confirms critical path and parallel tracks, identifies required artifacts and HITL gates, and then hands execution off to `plan-execution-lifecycle`. It is the explicit entry point; downstream recovery or re-entry skills should load lazily from execution state rather than at bootstrap.
---

# Post-Council Handoff

This skill is the execution entry-point anchor after council work is done.

Use it when a Tempor plan has already been created, promoted, and decomposed, and execution now needs a disciplined starting frame.

Boundary:
- Tempor generated the planning judgment and decomposition artifact.
- This skill translates that artifact into an executor-ready starting contract.
- `plan-execution-lifecycle` then governs ongoing execution.

Do not use this skill to:
- re-plan strategy
- reinterpret the approved plan from scratch
- replace `plan-execution-lifecycle`

## Trigger Conditions

Use this skill when any of these are true:

1. `tempor.decompose_plan` just completed.
2. A response includes `quality_signal`, `lifecycle_bridge`, or `execution_handoff` style metadata.
3. A decomposed plan exists, but the next execution move is ambiguous.
4. The decomposition is structurally weak and needs an explicit kickoff framing before work starts.

## Goal

Produce a short execution anchor that answers:

1. What is the active phase and first wave?
2. What is the critical path?
3. What can run in parallel?
4. What artifact(s) must exist before execution starts?
5. What triggers a return to Tempor vs normal lifecycle continuation?

## Inputs

Load only what is needed:

1. The decomposed plan artifact or `tempor.fetch_plan` output.
2. Any `quality_signal`, `lifecycle_bridge`, or `execution_handoff` metadata returned by Tempor.
3. The active `PLAN_INDEX*` only if execution docs already exist.

If the decomposition is weak or flat, read:
- `../plan-execution-lifecycle/references/ceremony-crosswalk.md`

If ongoing execution docs already exist, also read:
- the active kickoff
- the active lookahead

## Workflow

1. Confirm the boundary.
Tempor owns the approved plan and decomposition. This skill is only establishing the execution starting frame.

2. Identify the execution entry point.
Determine:
- active phase
- first executable wave
- blocking tracks
- safe parallel tracks

3. Assess execution shape.
If the decomposition is weak, call that out directly. Typical signals:
- one phase
- one wave
- many parallel tracks
- no meaningful dependencies

4. Produce the kickoff-ready handoff.
State:
- active `Plan / Phase / Wave`
- critical path
- parallelizable work
- required HITL gates
- required artifacts
- the first recommended execution action

5. Hand off to `plan-execution-lifecycle`.
The expected next step is kickoff / wave execution under that skill, not more ad hoc planning.

6. Declare re-entry triggers.
Say explicitly when execution should come back to Tempor:
- sustained drift across a phase
- repeated gate failures without convergence
- partial/failing outcomes that require plan mutation
- phase exhaustion or closeout that requires a fresh planning loop

## Output Contract

Keep the output compact and operational. It should include:

1. `Execution anchor`
2. `Critical path`
3. `Parallel tracks`
4. `Required artifacts`
5. `Return-to-Tempor triggers`
6. `Next step`

The handoff should be strong enough that a literal executor can proceed without inventing its own ceremony model.

## Lazy Fan-Out Rule

This skill is the anchor, not the whole methodology tree.

After the initial handoff:

1. Continue with `plan-execution-lifecycle` for normal execution.
2. Load drift- or re-entry-oriented skills only when execution state requires them.
3. Do not preload future recovery skills at bootstrap; let execution evidence reveal when they are needed.

## Good Outcomes

Good handoff behavior:
- the executor knows exactly where to start
- the lifecycle skill takes over cleanly
- return-to-Tempor moments are explicit
- no hidden second planning pass is needed

Bad handoff behavior:
- “here is the plan, good luck”
- silently treating a flat decomposition as execution-ready
- mixing planning judgment and execution ceremony back together
