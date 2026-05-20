# v8 — Phase 2 Lookahead — Deterministic surface parity port

**Plan / Phase:** `plan_gjt5io5gp71d` / `phase2_deterministic_surface`
**Phase name:** Phase 2 — Deterministic surface parity port
**Status:** Lookahead (groomed at project start; rebaselined into Phase 2 kickoff at Phase 1 closeout).

## Objective

Port every non-AI Python surface (file walkers, frontmatter parsers, tag/link/backlink extractors, signal generators, `fetch-stub-content`, `generate-signal`, related deterministic scripts) into Go and prove byte-for-byte parity against the Python prototype on the golden fixture vault from T09.

## Scope (proposed — 2 tracks across 2 waves)

| Wave | Track | Title |
|------|-------|-------|
| P2-W1 | T04 | Deterministic clients + Embedder interface |
| P2-W2 | T05 | Deterministic pipeline commands |

Note: this phase has **no AI calls.** AI integration is Phase 3 (router seam) and Phase 4 (retrieval).

## Dependency order

```
(Phase 1: T03 markdown engine, T09 fixture vault) ──→ T04 ──→ T05
```

Both T04 and T05 are sequential within the phase. T04 must land before T05 because T05's pipeline commands consume the deterministic clients.

## Proposed waves

### P2-W1 — Clients + Embedder iface (single track)

T04 implements:
- Pure-Go deterministic clients for any external services the Python prototype hits without AI (URL fetch with normalization stripping tracking params/fragments/trailing slashes and extracting YouTube video IDs; HTTP wrappers; etc.).
- The Embedder interface (concrete impl deferred to Phase 4; this is interface-only here to unblock Phase 3 wrappers).
- Table-driven tests for URL normalization, frontmatter parsing edge cases, and other deterministic logic surfaces.

### P2-W2 — Pipeline commands (single track)

T05 implements the deterministic pipeline commands:
- Go equivalents of `fetch-stub-content`, `generate-signal`, `audit-links`, `enrich-topics` (etc. — exact set determined by Python script inventory).
- Each command run on the golden fixture vault produces output that diffs zero against the captured Python reference output.
- CLI subcommands wired into the T02 skeleton.

## Entry criteria

- [ ] Phase 1 closeout passed.
- [ ] Markdown engine + vault abstraction (T03) committed and tested.
- [ ] Golden fixture vault (T09) committed at pinned SHA with expected outputs captured.
- [ ] Python prototype reference outputs captured for the in-scope deterministic scripts (recipe: run each Python script against the fixture vault, capture stdout/stderr/written files, commit as `testdata/expected-deterministic/`).

## Exit criteria

- [ ] All in-scope Python deterministic scripts have Go equivalents.
- [ ] `go test ./...` includes parity tests that diff Go output against captured Python references; zero diffs.
- [ ] No AI calls in any Phase 2 code path (CI grep gate confirms — no `claude`, `claude -p`, no provider SDK imports in any new code).
- [ ] CLI subcommands for each ported pipeline are usable from `cmd/` entry point.
- [ ] CGO_ENABLED=0 still passing.

## Key unknowns

- **Exact inventory of deterministic Python scripts to port.** Pre-Phase-2 grooming should produce a manifest mapping each Python file in `references/scripts/` to "port / drop / defer". The plan's T02 manifest is a starting point but Phase 2 kickoff should refine it.
- **URL normalization rule set.** The Python prototype's behavior is the canonical spec, but any rules that surface from real-corpus behavior in T03 may extend it.
- **Test fixture format for parity diffs.** Likely golden files in `testdata/expected-deterministic/`, but the exact organization (per-script subdirs, per-command-invocation subdirs, etc.) is a Phase 2 kickoff decision.
- **Embedder interface shape.** Deferred from T01 → T04. Interface signature decided here, concrete impl is Phase 4.

## Proposed HITL gates

- **P2-W1 gate:** Code review of clients + Embedder interface; table-driven test results; CI grep gate confirming no AI calls.
- **P2-W2 gate:** Parity test review; diff vs Python prototype outputs is empty for every in-scope script.
- **Phase 2 closeout:** Retrospective. Archive P2 wave docs. Rebaseline P3 lookahead into P3 kickoff. Create P4 lookahead.

## Carry-forward considerations from Phase 1

- ARCHITECTURE.md's Embedder interface section informs T04's interface decision.
- The Markdown engine from T03 is the substrate T04/T05 consume — must be stable before P2 starts.
- Whatever package layout T02 establishes is honored here.

## Drift signals (watch list)

- Phase 2 starting to implement retrieval (drift into Phase 4) — strictly deterministic surfaces only.
- Phase 2 introducing AI calls (drift into Phase 3) — should not happen.
- Hidden dependencies on Python sub-scripts (calling Python from Go) — explicitly forbidden by the "no Python in shipped output" constraint.

## Skill attachments (planned)

- `git-commit-workflow` at each wave gate and phase closeout.
- `pulse-save` if context-window pressure surfaces between gates.

## Rebaseline notes (at promotion to kickoff)

Promote into `v8-phase2-kickoff.md` once Phase 1 closes. Rebaseline against:

- Python script inventory verified against actual `references/scripts/` content.
- Embedder interface signature as defined in ARCHITECTURE.md.
- Any deferred T03/T09 items that need to be picked up.
- Updated dependency analysis: what actually shipped in Phase 1 vs what was assumed.
- The repo's commit ref at Phase 1 closeout.
