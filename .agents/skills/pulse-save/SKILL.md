---
name: pulse-save
description: Create or refresh the handoff save point at `.tempor/PULSE_CHECK.md` for the next agent/context window. Use when ending a work slice or before switching agents.
---

# Pulse Save

Creates an overwriteable fresh-context handoff checkpoint so another agent can resume with minimal reconstruction after a context switch, machine switch, or interrupted session.

Treat `.tempor/PULSE_CHECK.md` like a game save point. The primary path is deliberate: cap the current session and start again in a fresh context window. The recovery path is the bare-minimum guarantee: if the session dies, the next agent should fall back to a less catastrophic, well-labeled state.

## Command

```bash
bash .agents/skills/pulse-save/scripts/pulse-save.sh
```

Optional:

```bash
bash .agents/skills/pulse-save/scripts/pulse-save.sh <plan_dir> [output_file]
```

## Contract

1. Overwrite `.tempor/PULSE_CHECK.md` on each run.
2. Keep output compact, pointer-heavy, lifecycle-aware, and handoff-oriented.
3. Treat `pulse-save` as the final step in the normal lifecycle cadence:
- gate/HITL evidence packaged
- build/test/coverage thresholds checked
- commit completed when the slice includes tracked product changes
- then pulse-save
4. Also use `pulse-save` as a manual eject checkpoint when:
- the context window is bloated
- execution drift is high enough that a fresh window is safer
- a session must stop before the ideal gate/commit point
5. Include a clear `Immediate Next Agent Action` section with:
- the first file(s) to read
- the next bounded action
- the explicit stop condition
- what not to touch
6. Include the last known durable state:
- branch and commit
- active phase kickoff/lookahead
- active wave kickoff/lookahead
- latest gate artifact and latest committed wave when inferable
- repo dirty-state summary
7. Do not mutate plan artifacts while generating.
8. If any lifecycle field is unknown, keep the unknown visible and add enough surrounding pointers for handoff/recovery.

## Placement In The Lifecycle

Use `pulse-save` in two cases.

Normal lifecycle checkpoint:

1. Wave or phase gate evidence is written.
2. Required quality gates have run: build/check, tests, coverage threshold when applicable.
3. Tracked product changes are committed or explicitly identified as uncommitted carryover.
4. `.tempor/PULSE_CHECK.md` is refreshed with next action, scope boundary, and dirty-state signal.

Manual eject checkpoint:

1. The context window is bloated, stale, or too drifted to continue confidently.
2. The current state is saved before starting a fresh context window.
3. The pulse must label incomplete gates, uncommitted changes, and the safest next action plainly.

If a session must stop before the ideal gate/commit point, still run `pulse-save`; the output should make the incomplete state obvious rather than letting the next agent infer it.

## Quality Bar

Before ending a session, inspect `.tempor/PULSE_CHECK.md` and make sure a fresh agent can answer:

1. Where do I resume?
2. What is already safely committed?
3. What exact action comes next?
4. What is the scope boundary?
5. What files should I avoid staging or reverting?
