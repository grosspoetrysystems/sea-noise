# Ceremony Crosswalk

Canonical disambiguation for execution terms used in this repository.

## Purpose

Use this reference to avoid mixing terminology when discussing execution flow.
It does not introduce extra naming layers or aliases.

## Execution Hierarchy (4 levels)

```
Plan → Phase → Wave → Track
```

| Level | What it is | Agile analog | Owner |
|---|---|---|---|
| Plan | Full delivery arc from deliberation through closeout | Initiative | Human |
| Phase | Sequential objective grouping related waves | Epic | Human (gates) |
| Wave | Gated execution window within a phase | Sprint | Orchestrator |
| Track | Parallel work unit within a wave | Story/workstream | Subagent |

Rules:
1. Parallelism is expressed at exactly one level: tracks within a wave.
2. Track naming must be descriptive (`T-DB-BOUNDARY`), not opaque (`SA-02d`).
3. One agent owns one track. Internal sequencing is the agent's concern, not a plan-level concept.
4. Gates check across all tracks at the wave level before the next wave starts.

## Orchestrator Role

The orchestrator is the primary agent session coordinating wave execution. It is not a separate agent or track.

Orchestrator responsibilities at wave boundaries:
1. Sequence track execution and enforce dependencies between tracks.
2. Spawn and coordinate subagents for each track.
3. Run quality gates (`pnpm check`, `pnpm test`, `pnpm test:coverage:check`).
4. Package gate evidence for HITL review.
5. Present gate outcome and next-wave recommendation to the human.

Do not create dedicated integrator or QA agents (e.g., `SA-00`, `SA-QA`). These are orchestrator responsibilities, not tracks.

## Ceremony Disambiguation

| Term in this repo | Agile analog | Meaning here |
|---|---|---|
| HITL Gate (`P*-W*`) | Sprint review / milestone checkpoint | Human approval to continue or rework |
| Lookahead | Backlog grooming artifact | Draft scope for the upcoming phase |
| Kickoff | Planning ceremony | Rebaselined commitment document for active phase |
| Closeout | Retrospective ceremony | Outcome capture, deferrals, and archive transition |
| Rotation | Rolling planning cadence | Promote next lookahead to kickoff, create another lookahead |

## Usage Rule

Keep canonical IDs and names as-is (`Phase`, `Wave`, `Gate`, `T-*`).
Use agile terms only to clarify meaning, not to rename artifacts.
