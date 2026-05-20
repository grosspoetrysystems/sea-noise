# v8 — P1-W1 Wave Kickoff — Architecture Alignment

**Plan / Phase / Wave:** `plan_gjt5io5gp71d` / `phase1_foundation` / `phase1_wave1`
**Wave name:** P1-W1 — Architecture alignment
**Status:** Active (waiting on entry criteria — git repo init).

## Track

| Track ID | Title | Owner | Execution mode | Workspace |
|----------|-------|-------|----------------|-----------|
| T01 | Retrieval-first architecture alignment | unassigned | background-parallel | `.worktrees/t-t01-retrieval-first-architecture` (branch `swarm/t-t01-retrieval-first-architecture`) |

Single track — no intra-wave parallelism.

## Scope

T01 produces `ARCHITECTURE.md` at the repo root. The doc must include the following sections (mapped from T01 acceptance criteria in v8.1 decomp):

1. **Retrieval substrate decision**: SQLite FTS5/BM25 as primary, with structural signals enumerated explicitly: tags, links/backlinks, entities, assertions, recency, note types.
2. **Vault-derived SQL schema**: Tables and their purpose — notes, chunks, tags, entities, links, assertions, timestamps. May link to a separate design doc if it gets long.
3. **Retrieval router interface**: All five modes named — direct lookup, FTS search, graph/link expansion, deterministic context builders, optional semantic expansion.
4. **Hard binary constraints statement**: explicit declaration that the shipped binary contains no Python runtime, no MCP requirement, CGO_ENABLED=0, optional semantic layer is interface-bound (Embedder) and not required for initial ship.
5. **Fallback chain**: chromem/exact-cosine (semantic, optional) → FTS5/BM25 (keyword, primary) → disabled.
6. **Inference router seam** (cross-reference to Phase 3 / T06): note that inference routing is a separate concern from retrieval routing, with a separate interface (`internal/ai/inferencerouter`); ROUTER-SEAM.md will be the Phase 5 doc that captures both seams.

## Acceptance criteria (from v8.1 decomp)

- [ ] ARCHITECTURE.md committed naming SQLite FTS5/BM25 as primary retrieval with structural signals enumerated.
- [ ] Schema for vault-derived SQL tables documented in ARCHITECTURE.md or linked design doc.
- [ ] Retrieval router interface defined with all five modes.
- [ ] ARCHITECTURE.md explicitly states binary constraints (no Python in shipped binary, CGO_ENABLED=0, no MCP, optional semantic layer is interface-bound).
- [ ] Fallback chain documented: chromem/exact-cosine (optional) → FTS5/BM25 (primary) → disabled.

## Evaluation criteria

- **Doc review** of ARCHITECTURE.md against `tempor/RETRIEVAL-RECOMMENDATION.md` and the project-memory constraint list. Pull-request review (once repo is git-initialized) confirms all required sections present.

## Dependencies satisfied

- [x] No upstream tracks; T01 has zero dependencies in the decomposition.
- [ ] Entry-criteria blockers from Phase 1 kickoff (git init).

## Execution discipline

- **Docs-first.** This wave IS a docs deliverable. No implementation code lands in this wave.
- **Tests-first (N/A).** Doc-only wave — no test suites to write.
- **Worktree provisioning.** Once git is initialized, provision worktree `.worktrees/t-t01-retrieval-first-architecture` on branch `swarm/t-t01-retrieval-first-architecture`.

## Gate checklist (HITL)

- [ ] ARCHITECTURE.md exists at repo root.
- [ ] All five required sections present (substrate, schema, router interface, constraints, fallback).
- [ ] No retrieval implementation code introduced (drift check — T01 is doc-only).
- [ ] No metric thresholds appear (Recall@K, MRR, latency) in any acceptance language anywhere in the doc.
- [ ] No language referencing a retrieval evaluation harness.
- [ ] Cross-references to RETRIEVAL-RECOMMENDATION.md are consistent.
- [ ] Inference router seam mentioned as separate from retrieval router.
- [ ] Quality gates: `CGO_ENABLED=0 go build ./...` not yet expected (no Go code yet — defer to P1-W2 gate).
- [ ] Commit history clean and follows `git-commit-workflow`.
- [ ] Product-impact briefing for the gate (below) reviewed.
- [ ] `.tempor/PULSE_CHECK.md` refreshed via `pulse-save` after gate evidence.
- [ ] Human approves before P1-W2 starts.

## Skill attachments

- `git-commit-workflow` — required for the commit landing ARCHITECTURE.md and for the gate-evidence commit.

## Product impact briefing (gate template)

To be filled in at the gate:

- **Active context:** Plan: parity-first Go port. Phase 1 / Wave 1 / Track T01.
- **Benefits unlocked by delivered scope:** _(at gate)_
- **Tradeoffs accepted:** _(at gate)_
- **Linkage:** strategic plan `versions/v8/plan.json`; decomposition `versions/v8/decompositions/decomp_v1/plan.json`; this wave kickoff; ARCHITECTURE.md commit ref.

## Drift signals (watch list)

- T01 producing schema specifics that lock retrieval implementation (drift into Phase 4 scope) — keep at interface/contract level.
- ARCHITECTURE.md acquiring metric thresholds or harness language — reject at review per Correction 2 of v8.
- Conflict between RETRIEVAL-RECOMMENDATION.md and the new doc — RETRIEVAL-RECOMMENDATION.md wins as the canonical source for retrieval architecture decisions.

## Next-wave lookahead

See [v8-phase1-wave2-lookahead.md](v8-phase1-wave2-lookahead.md) — promote to wave kickoff once this gate passes.
