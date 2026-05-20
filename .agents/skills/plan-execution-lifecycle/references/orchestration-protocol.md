# Parallel Orchestration Doctrine (Worktree-First)

Planning and decomposition artifacts must treat Git worktrees as a first-class execution primitive for parallel subagent delivery.

Required in kickoff/lookahead/decomposition/gate artifacts:
1. Define per-track workspace isolation strategy (`worktree` path/branch naming or explicit rationale if not used).
2. Define integration boundary for each track (expected merge surface, shared contracts, conflict hotspots).
3. Define deterministic evidence chain per track:
- commit refs
- changelog entries
- gate evidence
- drift/eval signals
4. Define orchestrator handoff contract per wave (inputs, outputs, promotion/gate criteria, rollback posture).

## Agent Execution Mode Decision Framework

Every track in a wave must be assigned an execution mode at kickoff time. The mode is determined by three factors: **task type**, **complexity**, and **execution expectation** (what the agent is expected to produce).

### Task Type -> Valid Modes

| Task type | Description | Valid modes |
|---|---|---|
| **Research / read-only** | Explore codebase, search, gather context, answer questions — no file writes | `background-parallel` (preferred), `foreground-sequential` |
| **Analysis / report** | Inspect code, produce a review or summary, no commits | `background-parallel` (preferred), `foreground-sequential` |
| **Implementation** | Write or modify `src/` files, tests, docs — produces a commit | See complexity tiers below |
| **Integration / merge** | Merge worktree branches, resolve conflicts, run gates | `orchestrator-direct` only |
| **Commit / ceremony** | Stage files, write CHANGELOG, produce gate artifacts | `orchestrator-direct` only |

### Complexity Tier -> Mode Preference for Implementation Tasks

| Tier | Signals | Preferred mode |
|---|---|---|
| **Simple** | <=3 files, fully-specified contract, no shared file risk, additive only | `background-parallel` if permissions allow; else `foreground-sequential` |
| **Moderate** | 4-8 files, clear spec but implementation choices remain, low shared file risk | `foreground-sequential` |
| **Complex** | >8 files, exploratory or uncertain scope, modifies shared contracts | `foreground-sequential` or `orchestrator-direct` |
| **High-stakes** | Modifies load-bearing shared files (pipeline, schemas, storage, index) | `orchestrator-direct` |

### The Two Orthogonal Dimensions

Execution mode and checkpoint tier are independent. Background is the preferred mode for fanned-out parallelism — it is not inherently unsafe. The checkpoint tier governs when output is reviewed and when drift is caught, regardless of how the agent ran.

```
Execution mode  x  Checkpoint tier
---------------------------------------------
background      x  post-task    <- default for simple implementation
background      x  mid-task     <- for complex scope or sensitive contracts
foreground      x  per-call     <- for high-stakes, requires interactive approval
orchestrator    x  continuous   <- for integration, ceremony, shared files
```

### Checkpoint Tiers

**Post-task (default):** Agent completes its full scope and commits to a worktree branch. Orchestrator inspects the commit (`git show --stat`, diff review) before merging. Catches scope creep, unrelated patches, and contract drift at the merge boundary. This is the standard worktree merge review already practiced in this repo.

**Mid-task:** Agent is given an explicit pause point in its prompt — after producing a plan, spec, or draft diff — at which it surfaces output for orchestrator review before writing files or committing. Preserves background execution for the research phase while adding approval before destructive/additive writes. Use when the task involves uncertain scope or touches shared contracts.

**Per-call (foreground only):** Every Write/Bash call is approved interactively. Use only when the task is high-stakes and per-action review is worth the sequential cost. Rarely needed if post-task and mid-task checkpoints are well-designed.

**Continuous (orchestrator-direct):** The orchestrator is the agent. No delegation. Use for integration, CHANGELOG, ceremony artifacts, and fallback.

### Sensitive Area Flags

Certain files and scopes trigger mandatory checkpoint escalation regardless of complexity tier:

| Scope | Minimum checkpoint tier |
|---|---|
| `src/pipeline/index.ts` | mid-task |
| `src/services/schemas.ts`, `tool-registry.ts` | mid-task |
| `src/storage/plan-store/`, `src/storage/policy/` | mid-task |
| New exported API surfaces (index.ts re-exports) | mid-task |
| CHANGELOG.md, gate artifacts, kickoff docs | post-task (orchestrator reviews before commit) |
| Any file also modified by another in-flight track | mid-task (coordination required) |

### Execution Expectation Rules

1. **Background is the default for fanned-out parallelism.** Do not downgrade to foreground just because a task writes files — use background + post-task checkpoint instead. Background agents that need Write/Bash must have those tools auto-approved; if not, declare this in the kickoff and use foreground as a fallback, not as the default.

2. **Checkpoints catch drift early — they are not optional for implementation tracks.** Every implementation track must have at least post-task review before the worktree is merged. Mid-task checkpoints are required when sensitive area flags apply.

3. **Foreground agents block the orchestrator.** Reserve for tasks where per-call visibility is required (e.g. uncertain scope touching sensitive areas where mid-task prompting is impractical).

4. **Isolation is only preserved by worktree modes.** `orchestrator-direct` works on the current branch — no per-track isolation. If the audit trail requires per-track branch history, this is a limitation to record in the gate.

### Pre-Launch Checklist (required for each wave)

Before launching any implementation agent:
```
[] Task type classified
[] Complexity tier assessed
[] Sensitive area flags checked -> checkpoint tier set
[] Execution mode declared in kickoff artifact (background preferred)
[] If background: Write + Bash auto-approved? Y/N -> if N, use foreground as fallback
[] Checkpoint tier declared: post-task / mid-task / per-call
[] Mid-task pause point defined in agent prompt (if mid-task tier)
[] Fallback mode defined if primary mode fails
```

### Fallback Recording (required)

If the declared mode cannot be used and a fallback is applied, record in the wave gate artifact:
- Declared mode and checkpoint tier, and why they failed
- Actual mode used
- Whether worktree isolation was preserved
- Impact on audit trail (commit lineage, branch history)

Rationale:
1. Worktree isolation improves deterministic parallelism and auditability in multi-agent swarms.
2. This complements existing DDD/TDD/RAG/eval-harness and ceremony-handoff practices.
3. This aligns with observed orchestrator patterns (`openclaw`, `tmux`, `dmux`) while staying tool-agnostic.

Boundary:
1. Local workflow doctrine only; do not treat this as a mandatory Tempor `src/` runtime feature.
2. Experimental symbolic abstractions (e.g., future zvec/symbolic markdown standards) remain independent and out-of-scope unless explicitly promoted into product requirements.
