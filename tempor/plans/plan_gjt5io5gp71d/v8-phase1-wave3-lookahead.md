# v8 — P1-W3 Wave Lookahead — Vault + Fixture (parallel)

**Plan / Phase / Wave:** `plan_gjt5io5gp71d` / `phase1_foundation` / `phase1_wave3`
**Wave name:** P1-W3 — Vault + fixture (parallel)
**Status:** Lookahead (rebaselined into wave kickoff at the P1-W2 gate).

## Tracks (proposed — 2 in parallel)

| Track ID | Title | Worktree branch |
|----------|-------|-----------------|
| T03 | Markdown engine + vault abstraction | `swarm/t-t03-markdown-engine-vault-abstraction` |
| T09 | Golden fixture vault | `swarm/t-t09-golden-fixture-vault` |

T03 and T09 are independent — no cross-dependency. Both depend on T02 (P1-W2) for the package layout and config loader. Both run in parallel via separate worktrees under `.worktrees/`.

## Scope (proposed)

### T03 — Markdown engine + vault abstraction

- Frontmatter parser (YAML metadata at the top of each note).
- Wikilink extractor (`[[Note]]`, `[[Note|alias]]`).
- Tag extractor (frontmatter `tags:` and inline `#tag`).
- Backlink computation (derived from forward links).
- Vault walker (resolve vault path via `internal/config`, walk markdown files, return note records).
- Note ID assignment (stable, derived from path).
- Table-driven tests against the Python prototype's behavior on edge cases (collected from `references/scripts/`).

### T09 — Golden fixture vault

- Hand-authored markdown notes representing the 7 case types observed in the Python prototype (stub clip, transcript note, assertion, fleeting note, weekly bloom, topic frequencies, signal queue).
- Structural variation covering FTS, tags, links, entities, assertions, recency.
- Committed under `testdata/fixture-vault/` at a pinned SHA (no live-vault extraction).
- A set of golden expected retrieval outputs (top-K result files, expected note IDs) for representative queries — captured deterministically from running the Python prototype against the fixture vault, then committed under `testdata/expected-retrieval/`.

## Entry criteria

- [ ] P1-W2 gate passed (scaffold merged, build/test/grep gates green, config loader smoke-tested).
- [ ] Python prototype scripts confirmed runnable against an arbitrary vault directory (otherwise capturing expected outputs for T09 isn't reliable).
- [ ] A scratch vault path is available for T09 fixture authoring (could be a `tempor`-staging vault, not the live iCloud one).

## Exit criteria

- [ ] **T03:** `go test ./internal/vault/... ./internal/markdown/...` passes against the T09 fixture vault.
- [ ] **T03:** Byte-for-byte parity with the Python prototype's frontmatter/wikilink/tag extraction on the fixture vault (diffs zero).
- [ ] **T09:** `testdata/fixture-vault/` committed with hand-authored notes covering the 7 case types plus structural variation.
- [ ] **T09:** `testdata/expected-retrieval/` committed with golden expected outputs for representative queries against the fixture.
- [ ] No retrieval engine work yet (drift check — T10 lives in Phase 4).
- [ ] CGO_ENABLED=0 still passing; grep gates still green.

## Proposed HITL gates

- **T03 gate:** Code review of markdown engine + vault abstraction; parity test results vs Python prototype output.
- **T09 gate:** Fixture review (does the vault cover the 7 case types and the structural variation needed for later phases?). Expected-output review.
- **Wave gate (combined):** Both tracks closed; ready for Phase 1 closeout.

## Key unknowns

- **Exact note shape per case type.** Hand-authoring requires modeling the prototype's real note conventions. Some of this is derivable from `references/scripts/` (which scripts read which fields), some requires running the prototype against a real-ish vault to capture conventions.
- **Note ID strategy.** Stable derivation from path is the natural choice, but the Python prototype's exact ID scheme should be honored if it differs (e.g., uses the filename minus extension, or a frontmatter `id` field if present).
- **Test fixture format for parity diffs.** Likely golden files under `testdata/expected-*` mirroring the directory structure of the corresponding Python script output. Pin at wave kickoff.

## Drift signals (watch list)

- T03 starting to implement FTS or SQLite logic (drift into Phase 4 / T10).
- T03 making "improvements" to the Python prototype's extraction (drift away from parity-first).
- T09 fixture vault using content from the user's actual live vault (drift away from "pinned, no live-vault extraction").
- Fixture vault scope creeping toward "exhaustive coverage" — keep it representative, not exhaustive.

## Skill attachments (planned)

- `git-commit-workflow` (required at the wave gate).
- `pulse-save` (eject checkpoint if context-window pressure surfaces during parallel track work — likely candidate since two tracks run simultaneously).

## Rebaseline notes (at promotion to kickoff)

Promote this lookahead into `v8-phase1-wave3-kickoff.md` once P1-W2 gate passes. Rebaseline against:

- The actual package layout and config loader API from T02.
- Python script inventory verified against `references/scripts/` (which scripts read frontmatter / tags / links — that's the surface T03 must reproduce).
- Any deferred T02 items.
- Updated unknowns: note ID strategy, case-type fixtures, test fixture format.
- Reference commit at P1-W2 gate.
