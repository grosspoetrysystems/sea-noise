---
name: git-commit-workflow
description: LLM-agnostic commit workflow for this Tempor repository with explicit staging, commit message discipline, hook-aware remediation, and required docs linkage. Use when any agent or human is asked to commit, stage selected files, or resolve commit blockers like index.lock and hook failures.
---

# Git Commit Workflow (Tempor Repo)

Follow this sequence for commit operations in this repository.

## 1) Inspect and Scope

1. Run `git status --short` and `git diff --name-only`.
2. Keep unrelated user changes unstaged unless explicitly requested.
3. Stage only files in the active task scope.
4. Default commit scope is Tempor product code (`src/`) and directly supporting tests/schema/config/changelog files.
5. Treat `.agents/*`, `.tempor/*`, and lifecycle ceremony docs as local process artifacts; do not make standalone commits for them unless explicitly requested.

## 1.5) Commit-Readiness Decision Gate (Required)

Before staging/committing, evaluate whether the current staged/unstaged delta is worth committing now or should wait for a more complete `src/` implementation.

Commit now when at least one is true:

1. A coherent `src/` behavior/capability slice is complete and testable.
2. The wave-level unit is decision-worthy/content-worthy (material product impact, not incidental churn).
3. Work is fragmented by blockers, and committing the current slice unblocks the next track/wave safely.

Defer commit when all are true:

1. `src/` implementation is still materially incomplete.
2. No blocker mitigation value from committing now.
3. The delta is mostly scaffolding/partial edits without clear standalone behavior change.

If committing a fragmented blocker-driven slice, record this explicitly in the commit report:

1. What blocker forced fragmentation.
2. What follow-up slice is required for completion.
3. Why this commit is still sufficient at collective wave impact level.

## 2) Stage Intentionally

1. Use explicit paths: `git add <path...>`.
2. Re-check with:
- `git diff --cached --name-only`
- `git diff --cached`

## 3) Enforce Repo Rules

This repository requires:

1. Conventional messages: `feat: ...`, `fix: ...`, `chore: ...`.
2. Commit subject style:
- **top-line only** — single subject line, no body, no co-author trailer
- use imperative tense (e.g., `implement ...`, `add ...`, `fix ...`)
- avoid past tense (`implemented`, `archived`, `added`)
- keep subject concise and scoped to one change theme
- prefer normalized lowercase naming when embedding identifiers (`beacon-fingerprint`, `gate-validation`, `archetype-routing`)
3. Commit subject intent:
- describe what `src/` does now, not which plan/wave/track produced it
- **never** use track IDs (`T-SCHEMAS`, `T-FINGERPRINT`), wave/phase/gate terms, or lifecycle language in commit subjects
- Wrong: `feat: T-SCHEMAS beacon event schemas` / `merge: T-TAXONOMY worktree`
- Right: `feat: add beacon event schemas and TypeScript types` / `feat: implement beacon subdirectory taxonomy`
4. `CHANGELOG.md` staged in same commit when non-doc files change.
5. For non-trivial product changes, update `.agents/CONTENT_NOTES.md` with cross-reference when this file is part of the current tracked workflow.
   - Invoke `.agents/skills/content-notes-build-in-public/SKILL.md` when drafting/updating content notes so entries include upside/tradeoffs and reusable build-in-public angles (Substack/YouTube/validation).
6. Quality gates before finalizing a commit series:
- `pnpm check`
- `pnpm test`
- `pnpm test:coverage:check`
7. If coverage thresholds fail, add or update tests before committing:
- write targeted tests for changed behavior and failure paths
- rerun `pnpm test:coverage:check` until thresholds pass
- do not weaken thresholds unless explicitly requested

## 4) Respect Hook Flow

Current hooks:

1. `.husky/pre-commit`
- `bash scripts/check-doc-roundup-staged.sh`
- `pnpm lint-staged`

2. `.husky/pre-push`
- `pnpm check`
- `pnpm test`
- `pnpm test:coverage:check`

If blocked, read hook output, fix root cause, restage, retry.

## 5) Commit and Confirm

1. Commit with concise, scoped message.
2. Run `git show --stat --oneline -1`.
3. Report commit hash, files, exclusions, and readiness-gate rationale (`complete slice` vs `fragmented unblock`).

## Worktree Agent Commits

When accepting a commit from a parallel subagent worktree:

1. **Inspect before merging.** Run `git show --stat HEAD` in the worktree to see exactly which files are in the commit.
2. **Strip pre-existing patches.** Worktree agents fix whatever is broken in their branch to get the build green. These patches (e.g. `plan-store/index.ts`, unrelated test files) are NOT part of the feature and must NOT be bundled.
   - If unrelated files are staged: `git reset HEAD~1` in the worktree, re-add only the feature files, recommit.
3. **Merge worktrees sequentially**, not simultaneously. `CHANGELOG.md` always conflicts when two worktrees each add an entry to the same `[Unreleased]` block. Resolve by keeping both entries and removing conflict markers — this is mechanical.
4. **Commit messages for merge commits**: use a code-first subject describing the capability added, not the worktree branch or track ID.
   - Wrong: `merge: T-SCHEMAS worktree` / `merge: beacon-fingerprint branch`
   - Right: `feat: add beacon event schemas and TypeScript types`

## Troubleshooting

### `.git/index.lock`

1. Check active git processes: `ps aux | rg "[g]it"`.
2. If none are active, remove stale lock: `rm -f .git/index.lock`.
3. Retry stage/commit.

### Sandbox denied `.git/*`

1. Re-run staging/commit with escalated permission.
2. Explain it is sandbox policy, not repository corruption.

### Docs roundup/changelog failure

1. Add concise `CHANGELOG.md` entry.
2. Add/update `.agents/CONTENT_NOTES.md` for non-trivial work.
3. Restage docs and retry commit.

### Coverage threshold failure

1. Inspect failing metric/module from `pnpm test:coverage:check`.
2. Add or expand tests for the changed behavior.
3. Re-run:
- `pnpm test`
- `pnpm test:coverage:check`
4. Commit only after coverage gates pass.
