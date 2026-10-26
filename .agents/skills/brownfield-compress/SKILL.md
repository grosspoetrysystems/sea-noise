---
name: brownfield-compress
description: Investigate a codebase (or scoped subsystem) and produce a structured compression artifact for downstream Tempor planning. Use when arriving fresh at a repo, scoping work from a ticket, or when plan quality is suffering from noisy/incomplete context. Outputs artifacts and run logs to `.tempor/compression/`.
---

# Brownfield Compress

Research step for the Research -> Plan -> Implement workflow. A fresh agent is blind every time. This skill is the structured way to open your eyes before planning.

## When to use

- Cold start on any non-trivial codebase
- Scoping a feature request or ticket into a plan
- When a previous plan had low quality, high repair rate, or contributor disagreement traceable to bad context
- When `sense_pattern` returns `full-council` with low confidence (the archetype short-circuit didn't fire — you need to research)

## When NOT to use

- Greenfield repos with <50 files where raw context fits in a single contributor window
- When `sense_pattern` matched an archetype at high confidence (the pattern IS the research — just plan)
- When a fresh compression artifact already exists and the repo fingerprint hasn't changed

## Inputs

The skill accepts optional scoping context. If invoked as `/brownfield-compress`, determine scope from conversation context. If invoked with args, interpret them as:

- **Ticket reference**: Linear issue ID, GitHub issue URL, or free-text description of the work
- **Scope path**: explicit directory/module to investigate (e.g., `src/beacon/`)
- **`--full`**: skip ticket-scoped narrowing, analyze the full repo (expensive, use sparingly)

## Protocol

Five phases, each time-boxed. See [references/protocol-phases.md](references/protocol-phases.md) for detailed step-by-step procedures, decision matrices, the artifact JSON schema, and the feedback log format.

| Phase | Time | Purpose |
|-------|------|---------|
| 0. Detect | < 30s | Triage: is compression worth it? Check for existing artifacts. |
| 1. Scope | < 2m | Narrow investigation target from ticket or repo structure. |
| 2. Investigate | 2-10m | Structured analysis: structure map, dependency map, risk map, invariants, unknowns. |
| 3. Produce | < 1m | Write `.tempor/compression/<scope_slug>.json` artifact. |
| 4. Feedback | < 30s | Append to `RUN_LOG.md` for protocol improvement over time. |

Key invariants:
- `confidence.overall` below 0.5 triggers a warning — follow-up investigation needed before planning.
- Unknowns section must never be empty. Actively probe for gaps.
- `compression_version` starts at 1, increments on refresh.

## Output to user

After completion, report:
1. Artifact path: `.tempor/compression/<scope_slug>.json`
2. Confidence: overall + any low-confidence sections flagged
3. Unknowns count: how many open questions remain
4. Suggested `context_refs` for the next `create_plan` call:

```json
[
  { "type": "file", "ref": ".tempor/compression/<scope_slug>.json", "label": "brownfield-compression-<scope_slug>" }
]
```

5. If confidence is below 0.5: explicitly recommend targeted follow-up investigation before planning.

## Integration with Tempor workflow

This skill sits upstream of Tempor's planning tools:

```
/brownfield-compress -> produces artifact
     |
tempor_create_plan(context_refs: [{ type: "file", ref: ".tempor/compression/X.json" }])
     |
tempor_decompose_plan -> tracks, waves, gates
     |
execution
```

The compression artifact does not replace `context_refs` — it augments them. You can (and should) still include targeted raw refs (git diffs, specific files) alongside the compression artifact for delta context.

## Refreshing a stale artifact

If the artifact exists but the repo fingerprint has changed:

1. Read the existing artifact
2. Run Phase 2 investigation but diff against the existing system_map — focus on what changed
3. Increment `compression_version`
4. Overwrite the artifact
5. Log the refresh in RUN_LOG.md with a note on what changed

This is cheaper than a fresh investigation because you're updating, not rebuilding.

## Portability

This skill is part of the **Tempor skill kit** — the set of portable skills that complement the MCP planning tools to cover the full Research -> Plan -> Implement cycle. See `.agents/skills/SKILL_MANIFEST.json` for the complete kit.
