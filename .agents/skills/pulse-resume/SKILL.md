---
name: pulse-resume
description: Read `.tempor/PULSE_CHECK.md` and print a handoff-ready resume/start plan for the next agent. Use when entering a fresh context window or taking over from another agent.
---

# Pulse Resume

This is the fresh-context handoff start button. It reads the save point and outputs immediate next steps from the pulse itself before falling back to older kickoff-local start orders.

## Command

```bash
bash .agents/skills/pulse-resume/scripts/pulse-resume.sh
```

Optional:

```bash
bash .agents/skills/pulse-resume/scripts/pulse-resume.sh <pulse_file>
```

## Contract

1. Default input: `.tempor/PULSE_CHECK.md`.
2. Default output format: canonical JSON (`PULSE_RESUME`) to stdout.
3. No bootstrap file is written.
4. Output includes `resume_plan` with extracted `Immediate Next Agent Action` from the pulse when available.
5. Fall back to extracted immediate-start order from the active kickoff only when the pulse does not include a next-action block.
6. Output includes active wave/gate fields, handoff safety notes, hard boundaries, and command-first resume steps.
7. Keep output short, actionable, and command-first.
