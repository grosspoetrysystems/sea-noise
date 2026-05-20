# Plan Index Template

Use this template when initializing a new plan-index version for lifecycle execution.

```md
# Plan Index V{N} — {plan_id}

Meta plan: v{N} ({short-description})
Deliberation source: `{source-docs}`

## Present

**Active kickoff:** `{kickoff-doc}`
**Active lookahead:** `{lookahead-doc}`

## Past

### Phase 1: {name} — {OPEN|CLOSED}
Tasks: {task-list}
Closeout commit: `{commit-or-tbd}`
Archived: {archive-doc-list}
Deferred: {optional-deferred-summary}

### Phase 2: {name} — {OPEN|CLOSED}
Tasks: {task-list}
Closeout commit: `{commit-or-tbd}`
Archived: {archive-doc-list}
Deferred: {optional-deferred-summary}

## Future

### Phase {P}: {name} — NEXT
Tasks/areas: {task-list}
See: `{next-kickoff-or-lookahead}`

### Phase {P+1}: {name} — LOOKAHEAD
Task areas: {task-area-list}
See: `{lookahead-doc}`

## Ceremony Notes

- Keep canonical terms and IDs: `Plan`, `Phase`, `Wave`, `Track`, `Gate`, `T-*`, `P*-W*`.
- Apply disambiguation from:
  - `.agents/skills/plan-execution-lifecycle/references/ceremony-crosswalk.md`
- Index invariants:
  - exactly one active kickoff + one active lookahead in `Present`
  - no phase duplicated across `Present` / `Past` / `Future`
  - active phase must not also appear in `Future` as `NEXT`
  - `Future` starts at the next not-yet-active phase
- Required per phase:
  - active kickoff exists
  - next-horizon lookahead exists
- At phase transition:
  - prior lookahead rebaselined into kickoff
  - subsequent lookahead created/updated
- Worktree-first parallel orchestration:
  - each active track defines worktree/branch isolation (or explicit exception)
  - each track includes merge/integration boundary notes
  - evidence chain links commits/changelog/gates/drift-eval artifacts
  - orchestrator handoff notes include promotion criteria + rollback posture

## Source Plans (Tempor-generated)

- `{v1}`
- `{v2}`
- `{v3}`
- `{v4}`
```

## Checklist For New Index Version

1. Fill `Present/Past/Future` sections with current pointers.
2. Verify kickoff/lookahead artifacts satisfy required phase rules.
3. Verify index invariants (single active phase, no cross-section duplicates, `Future` starts next).
4. Verify archive pointers are accurate and not stale.
5. Cross-check with `.agents/EVALS.md` ceremony scenarios.
