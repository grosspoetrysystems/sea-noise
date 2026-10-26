# Brownfield Compress — Protocol Phases

Detailed phase-by-phase investigation protocol. See [SKILL.md](../SKILL.md) for when/why to use this skill.

## Phase 0: Detect (< 30 seconds)

Quick triage — is compression even worth it? Also ensure output directory exists.

```
1. Check if .tempor/ exists.
   - If NO: create .tempor/compression/ (mkdir -p handles both levels).
     Note: .tempor/ normally exists when Tempor has been initialized in filesystem mode.
     This skill creates only the compression subdirectory — it does not run full Tempor init.
     The compression artifact is standalone and does not require the Tempor runtime.
   - If YES: ensure .tempor/compression/ exists (mkdir -p).
2. Count source files (excluding node_modules, .git, dist, build, vendor)
3. Check git log --oneline | wc -l (commit depth)
4. Check for existing .tempor/compression/*.json artifacts
5. Check for .tempor/beacon.json (fingerprint)
   - If present: use its commit SHA for repo_fingerprint field
   - If absent: use `git rev-parse HEAD`
6. ls top-level for language/framework signals (package.json, Cargo.toml, go.mod, pyproject.toml, etc.)
```

Decision matrix:
- **<50 files, <6 months history**: Skip compression. Report "repo is small enough for raw context" and exit.
- **Existing fresh artifact**: Report "compression artifact exists at <path>, fingerprint matches" and exit. Suggest passing it as a `context_ref`.
- **Otherwise**: Proceed to Phase 1.

## Phase 1: Scope (< 2 minutes)

Narrow the investigation target. The goal is to NOT analyze the whole repo.

**If ticket context is available:**

```
1. Read the ticket (Linear MCP, GitHub MCP, or user-provided text)
2. Extract: mentioned files, modules, subsystems, prior art references
3. grep/glob for those references in the codebase
4. Identify the directory subtree(s) that contain the relevant code
5. That subtree is your scope
```

**If no ticket context (exploring blind):**

```
1. Read top-level README, CLAUDE.md, AGENTS.md, package.json (or equivalent)
2. ls src/ (or main source directory) — identify major modules by directory name
3. Read entry points (index.ts, main.py, etc.) to understand module relationships
4. Scope = the full src/ tree, but with depth-limited investigation (major modules only, not every file)
```

**Output of this phase:** a `scope` value — one or more directory paths that will be investigated.

## Phase 2: Investigate (2-10 minutes, scales with scope)

Structured investigation of the scoped area. Follow this order — it's designed to build understanding incrementally.

### 2a. Structure map
```
1. glob the scope for all source files (*.ts, *.py, *.rs, etc.)
2. Identify entry points (index files, main files, exports)
3. Read entry points to understand the public API / module interface
4. Identify key types/interfaces (grep for 'export type', 'export interface', 'pub struct', etc.)
5. Map: module -> entry point -> key exports
```

### 2b. Dependency map
```
1. Read import/require statements in entry points
2. Identify internal dependencies (module A imports from module B)
3. Identify external dependencies (third-party packages used)
4. Note: which modules are imported by many others? (high fan-in = high coupling)
```

### 2c. Risk map
```
1. Find large files (>500 lines) — these are coupling hotspots
2. Find files with high import fan-in — these are blast-radius amplifiers
3. Check for: shared mutable state, global config, singleton patterns
4. Check for: migration files, schema files, config files that constrain behavior
5. grep for TODO, FIXME, HACK, WORKAROUND — these are known debt markers
```

### 2d. Invariants
```
1. Read test files — what do they assert? Test assertions are implicit invariants.
2. Read config/schema files — what constraints do they enforce?
3. Read CI config if present — what gates exist?
4. Check for: linting rules, type strictness settings, pre-commit hooks
```

### 2e. Unknowns (the hardest and most valuable step)

Do NOT skip this. Do NOT produce an empty list. Actively probe for gaps:

```
1. For each directory in scope: does the structure map have an entry? If not -> unknown.
2. For each module boundary: is the interface contract clear from reading exports? If not -> unknown.
3. For each external dependency: do you understand why it's used? If not -> unknown.
4. For each large/high-fan-in file: do you understand the coupling reason? If not -> unknown.
5. Are there directories with no tests? -> unknown (untested behavior).
6. Are there config values you can't trace to their effect? -> unknown.
```

## Phase 3: Produce artifact (< 1 minute)

Write the compression artifact to `.tempor/compression/<scope_slug>.json`.

Schema:

```json
{
  "schema": "tempor.brownfield_compression.v1",
  "repo_fingerprint": "<HEAD commit SHA>",
  "generated_at": "<ISO 8601>",
  "compression_version": 1,
  "scope": ["src/beacon/"],
  "scope_slug": "beacon",
  "source": {
    "ticket_ref": "LIN-123 or null",
    "generation_method": "manual-skill",
    "agent_model": "claude-opus-4-6"
  },
  "system_map": [
    {
      "module": "src/beacon/emitter.ts",
      "role": "Event emitter — writes structured JSONL to beacon sink",
      "exports": ["emit", "createEmitter"],
      "fan_in": 5,
      "notes": null
    }
  ],
  "risk_map": [
    {
      "path": "src/services/deliberation.ts",
      "risk": "2455-line monolith, highest fan-in in codebase",
      "category": "high-coupling",
      "severity": "high"
    }
  ],
  "change_surface": [
    {
      "feature_class": "Add new beacon event type",
      "affected_paths": ["src/beacon/schemas/", "src/beacon/emitter.ts", "src/beacon/taxonomy.ts"],
      "blast_radius": "low",
      "notes": "Additive, schema-first, no shared state"
    }
  ],
  "invariants": [
    {
      "rule": "All beacon events must pass Zod schema validation before write",
      "source": "test assertions + emitter implementation",
      "confidence": 0.95
    }
  ],
  "unknowns": [
    {
      "area": "src/beacon/sink.ts",
      "question": "What happens when disk is full? No error handling visible for ENOSPC.",
      "severity": "medium",
      "investigation_hint": "Check if Node fs.appendFile throws or silently fails on ENOSPC"
    }
  ],
  "planning_guidance": {
    "recommended_phases": null,
    "checkpoint_sensitive_paths": ["src/services/deliberation.ts", "src/services/schemas.ts"],
    "notes": "Schema changes should land before implementation — schema-first invariant."
  },
  "confidence": {
    "overall": 0.75,
    "system_map": 0.85,
    "risk_map": 0.70,
    "invariants": 0.80,
    "unknowns": 0.60
  }
}
```

Rules:
- `confidence.overall` below 0.5 -> emit a warning that this artifact needs follow-up investigation before planning.
- Any section below 0.5 -> that section MUST have corresponding `unknowns` entries explaining what's missing.
- `compression_version` starts at 1, increments on refresh.

## Phase 4: Feedback log (< 30 seconds)

Append to `.tempor/compression/RUN_LOG.md`. This is the learning loop.

Format per entry:

```markdown
## Run: <ISO timestamp>
- **Scope:** <scope paths>
- **Ticket:** <ref or "none">
- **Agent model:** <model>
- **Time spent:** <approx minutes>
- **Files read:** <count>
- **Confidence:** <overall score>
- **What worked:** <1-2 sentences on what investigation steps produced the most value>
- **What was hard:** <1-2 sentences on where the protocol struggled>
- **Missed in hindsight:** <anything discovered later that should have been caught — update this after planning/execution if applicable>
---
```

This log is the raw material for improving the skill protocol over time. After 5+ runs, review the log for patterns and update Phase 2 investigation steps accordingly.
