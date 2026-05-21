# v8 — Phase 1 Kickoff — Foundation

**Plan:** `plan_gjt5io5gp71d` — Parity-first Go port of Sea Noise prototype
**Phase:** `phase1_foundation` — Foundation
**Status:** Active (project start, no prior phase to rebaseline from)
**Reference state:** No git repo yet — filesystem-anchored at scaffolding date 2026-05-19.

## Objective

Stand up vault plumbing, repo scaffold, the markdown/vault abstraction, and the pinned golden fixture vault so every later phase has a stable foundation.

## Scope (4 tracks across 3 waves)

| Wave | Track | Title | Owner | Status |
|------|-------|-------|-------|--------|
| P1-W1 | T01 | Retrieval-first architecture alignment | unassigned | not started |
| P1-W2 | T02 | Project scaffold + CLI skeleton | unassigned | not started |
| P1-W3 | T03 | Markdown engine + vault abstraction | unassigned | not started |
| P1-W3 | T09 | Golden fixture vault | unassigned | not started |

Full track definitions: `.tempor/plans/sea-noise/go-port/plan_gjt5io5gp71d/versions/v8/decompositions/decomp_v1/plan.json` (phases[0].waves[*].tracks).

## Dependency order

```
T01 ──→ T02 ──→ ┬── T03 ──→ (Phase 2: T04)
                └── T09 ──→ (Phase 4: T10)
```

T03 and T09 are independent of each other; they run in parallel within P1-W3 via separate git worktrees.

## Proposed waves

### P1-W1 — Architecture alignment (single track)

- T01: Write `ARCHITECTURE.md` covering SQLite FTS5/BM25 retrieval substrate, structural signals, Embedder interface, hard binary constraints (CGO_ENABLED=0, no Python in shipped output, no MCP), retrieval router contract, fallback chain.
- See [v8-phase1-wave1-kickoff.md](v8-phase1-wave1-kickoff.md).

### P1-W2 — Project scaffold (single track)

- T02: Go module init, CLI skeleton (cobra or stdlib flag — TBD in scaffold), package layout, base config plumbing including vault.path resolution from `mind-palace.toml` or `MIND_PALACE_VAULT` env, CI gate stubs (CGO_ENABLED=0 build, go test, grep gates for forbidden imports).
- See [v8-phase1-wave2-lookahead.md](v8-phase1-wave2-lookahead.md) — promote to kickoff at P1-W1 gate.

### P1-W3 — Vault + fixture (parallel, 2 tracks)

- T03 (worktree `swarm/t-t03-markdown-engine-vault-abstraction`): Markdown engine, frontmatter parser, tag/link/backlink extractor, vault walker, byte-for-byte parity tests against Python prototype outputs on the fixture vault.
- T09 (worktree `swarm/t-t09-golden-fixture-vault`): Hand-author representative markdown notes covering the 7 case types from the prototype (stub clip, transcript note, assertion, fleeting note, weekly bloom, topic frequencies, signal queue) + structural variation for FTS/tags/links/entities/assertions/recency. Commit at pinned SHA under `testdata/fixture-vault/`. Capture expected retrieval outputs as fixtures.

## Entry criteria

- [ ] **Git repository initialized** (blocking — see Known prerequisites in `PLAN_INDEX_V8.md`).
- [x] Python prototype staged at `references/scripts/` and `references/scripts/_rag/`.
- [x] `tempor/RETRIEVAL-RECOMMENDATION.md` exists.
- [x] Live vault path resolution rule defined: `mind-palace.toml` or `MIND_PALACE_VAULT` env, never hardcoded (per project memory).

## Exit criteria (Phase 1 closeout)

- [ ] `ARCHITECTURE.md` committed and matches RETRIEVAL-RECOMMENDATION.md.
- [ ] Go module + CLI skeleton committed; `CGO_ENABLED=0 go build ./...` exits 0.
- [ ] Markdown engine + vault abstraction passes `go test ./internal/vault/... ./internal/markdown/...` against Python prototype reference outputs on the fixture vault.
- [ ] Golden fixture vault committed at pinned SHA under `testdata/fixture-vault/` with committed expected-output fixtures.
- [ ] All 4 tracks reach completed status with their full acceptance_criteria met.
- [ ] CI grep gates green: no forbidden model literals, no Python symbols, no MCP imports in any Go source.

## HITL gates

- **P1-W1 gate:** Doc review of `ARCHITECTURE.md`. Confirms architecture decision matches RETRIEVAL-RECOMMENDATION.md. Human approval required before P1-W2 starts.
- **P1-W2 gate:** Build/test review. Confirms scaffold compiles and config plumbing resolves vault path correctly. Human approval required before P1-W3 starts.
- **P1-W3 gate:** Track-completion review (T03 + T09 closed in parallel). Confirms markdown engine parity against Python reference, fixture vault pinned, expected-output fixtures committed. Human approval required before Phase 1 closeout.
- **Phase 1 closeout:** Full retrospective. Update kickoff with final statuses, document any deferred items, archive closed wave kickoffs, rebaseline P2 lookahead into P2 kickoff, create P3 lookahead.

## Key unknowns

- **Git init policy:** Should the repo be initialized at root (`/Users/kd-m2air/Local/sea-noise`) or a sub-directory like `app/`? Affects worktree provisioning. Resolve before T01 commit.
- **Go module name:** TBD — likely `github.com/<user>/sea-noise` or `<user>/sea-noise`. Resolve at T02 kickoff.
- **CLI framework choice:** cobra vs stdlib flag. Defer to T02 scaffolding; non-blocking decision.
- **Note-type coverage in fixture vault:** The skill description names 7 case types; verify against actual Python prototype usage during T09 design. May expand.
- **modernc.org/sqlite FTS5 verification:** Real-corpus check happens in Phase 4. If it fails, fallback to `ncruces/go-sqlite3` per RETRIEVAL-RECOMMENDATION.md — note in Phase 4 lookahead.

## Carry-forward items

_None — project start._

## Skill attachments

- `git-commit-workflow` (required for each wave gate and closeout commit)
- `pulse-save` (eject checkpoint if context-window pressure surfaces before a formal gate)

## Product impact briefing (initial)

- **Active context:** Plan: parity-first Go port. Phase 1 / Wave 1 / Track T01.
- **Benefits unlocked at phase exit:**
  - A canonical, committed architecture statement other agents can route through without reinterpreting scope.
  - A working Go scaffold that proves the hard binary constraints (CGO_ENABLED=0, no Python, no MCP) are buildable from day one.
  - A pinned golden fixture vault decoupled from the live iCloud Obsidian vault, so all later parity verification has a stable substrate that can be CI'd.
- **Tradeoffs accepted:**
  - Doc-first cost upfront (T01 produces no executable artifact). Justified because every later phase has architectural questions that benefit from a single canonical answer.
  - Parallel T03/T09 in worktrees adds orchestration cost vs sequential. Justified because the two tracks have no cross-dependency and Phase 1 is the foundation gate for everything.
  - Hand-authored fixture vault is upfront work that won't auto-update. Justified because live-vault extraction would couple parity verification to evolving content.
- **Linkage:**
  - Strategic plan: `.tempor/plans/sea-noise/go-port/plan_gjt5io5gp71d/versions/v8/plan.json`
  - Decomposition: `.tempor/plans/sea-noise/go-port/plan_gjt5io5gp71d/versions/v8/decompositions/decomp_v1/plan.json`
  - Architectural authority: `tempor/RETRIEVAL-RECOMMENDATION.md`
  - This plan index: `tempor/plans/plan_gjt5io5gp71d/PLAN_INDEX_V8.md`

## Drift watch

At each wave gate evaluate:
1. **Scope drift:** Are tracks reaching beyond their stated scope (e.g., T01 starting to do retrieval implementation, T03 implementing FTS)?
2. **Decision drift:** Are foundational decisions (driver choice, CLI framework, module structure) being revisited mid-phase?
3. **Context pressure:** Are unknowns from the kickoff resurfacing without resolution?

If sustained drift across the phase: trigger Tempor re-council via fresh `iterate_plan_local` cycle, rebaseline Phase 1 kickoff from output.

## Rebaseline rationale

_N/A — first kickoff, no prior lookahead to rebaseline._
