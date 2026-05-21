# v8 — Phase 2 Kickoff — Deterministic surface parity port

**Plan / Phase:** `plan_gjt5io5gp71d` / `phase2_deterministic_surface`
**Phase name:** Phase 2 — Deterministic surface parity port
**Status:** Active.
**Reference commit:** `b5009ff` (T03 — markdown parser and vault walker with parity tests). Plus Phase 1 closeout commit to follow this kickoff.

## Rebaseline notes

Promoted from `v8-phase2-lookahead.md` at Phase 1 closeout. Updated with Phase 1 outcomes:

- **`internal/markdown`** ships `ParseFrontmatter`, `ExtractWikilinks`, `TitleFromFilenameStem`. Behavior mirrors the Python prototype byte-for-byte (verified by parity tests against the T09 fixture vault).
- **`internal/vault`** ships `Walk(root, excludeDirs) ([]Note, error)` plus `IndexerExcludeDirs` and `LinksExcludeDirs` matching `_rag/indexer.py` and `audit-links.py` respectively. Sentinel error `ErrVaultNotFound`. Note record shape: `{ID, Path, Title, Tags, Body}`.
- **`testdata/fixture-vault/`** has 34 hand-selected notes from the live iCloud vault, pinned. **`testdata/expected/frontmatter/index.json`** and **`testdata/expected/links/per-note.json`** are the golden files. The `testdata/scripts/capture-parity.py` script regenerates them and is the documented recapture procedure.
- **Capture environment** uses a gitignored `.venv/` and a sparse `requirements.txt`. Phase 2 adds Python deps to `requirements.txt` as the heavier prototype scripts get ported (anticipated: `python-frontmatter`, `requests`, `youtube-transcript-api`).
- **`internal/config.LoadVaultPath()`** resolves vault path; sentinel errors and `errors.Is` pattern in place. T05's pipeline commands consume this.
- **Test discipline rule** is enforced via PreToolUse hook injection on any `*_test.go` edit. The eight rules are now visible to every agent that touches a test file.

## Objective

Port every non-AI Python surface (file walkers consumed by external scripts, frontmatter parsers, link/tag extractors are done; **now**: URL normalization, HTTP fetchers, deterministic pipeline commands, generate-signal-style processing) into Go and prove byte-for-byte parity against the Python prototype on the fixture vault.

## Scope (2 tracks across 2 waves)

| Wave | Track | Title |
|------|-------|-------|
| P2-W1 | T04 | Deterministic clients + Embedder interface |
| P2-W2 | T05 | Deterministic pipeline commands |

**No AI calls in this phase.** AI integration is Phase 3 (T06 router seam + T07 Claude wrappers).

## Dependency order

```
(Phase 1 complete: markdown engine, vault walker, fixture vault, golden files)
            │
            ▼
          T04 ──→ T05
```

T04 produces the client primitives + Embedder interface contract. T05 consumes them when porting the pipeline commands (e.g., a stub-fetcher uses T04's URL client; an entity profiler stub uses the Embedder interface even though the concrete impl is Phase 4).

## Proposed waves

### P2-W1 — Clients + Embedder iface (single track)

T04 implements under `internal/ai/embedder/` and a new `internal/clients/` (or similar — pin at wave kickoff):

- **URL normalization**: strip tracking params (`utm_*`, `ref`, `fbclid`, etc. — full set per the Python prototype), strip fragments, strip trailing slashes, extract YouTube video IDs into a canonical form. Table-driven tests against the prototype's behavior.
- **HTTP fetcher** (deterministic-ish): a small wrapper around `net/http` for fetching arbitrary URLs, returning body + content-type + status. No retries baked in at this layer — T05's pipeline commands choose retry behavior.
- **Embedder interface** in `internal/ai/embedder/`: per ARCHITECTURE.md §5. `Embed(ctx, []string) ([][]float32, error)`, `Dimension() int`, `ProviderID() string`. No concrete implementation in this wave — Phase 4 (T10) decides whether to ship a semantic layer.
- **Stub Embedder for tests**: a deterministic fake (returns zero vectors of the configured dimension) so downstream consumers in T05+ can wire it without a real provider.

### P2-W2 — Pipeline commands (single track)

T05 ports the AI-free Python scripts to Go CLI subcommands wired into `cmd/sea-noise`:

Candidates from `references/scripts/` (AI-free per surveys in P1 — verify at wave kickoff):
- `audit-links.py` → `sea-noise audit-links` (uses `internal/vault.Walk` + `internal/markdown.ExtractWikilinks`)
- `fetch-stub-content.py` → `sea-noise fetch-stub-content` (uses T04 URL client + HTTP fetcher)
- `fetch-youtube-transcript.py` → `sea-noise fetch-youtube-transcript` (uses T04 YouTube ID extractor)
- `generate-signal.py` → `sea-noise generate-signal` (deterministic frontmatter aggregation)
- `sync-todoist*.py` → defer or include depending on Todoist API parity (verify if AI calls are involved)

Each command:
- Reads from the configured vault via `internal/config.LoadVaultPath()`.
- Writes outputs to the vault path or stdout per the prototype's behavior.
- Has parity tests using the golden fixture vault. Capture procedure (`testdata/scripts/capture-parity.py`) extends to cover these surfaces.

## Entry criteria

- [x] Phase 1 closeout passed: markdown engine + vault walker + fixture vault + golden files all committed and tested.
- [ ] Python prototype scripts' dependency footprint verified — add to `requirements.txt` what T05's parity captures need.
- [ ] CGO_ENABLED=0 still passing on main.

## Exit criteria

- [ ] All in-scope Python deterministic scripts have Go equivalents wired into `cmd/sea-noise`.
- [ ] `go test ./...` includes parity tests that diff Go output against captured Python references; zero diffs.
- [ ] No AI calls in any Phase 2 code path (CI grep gate confirms — no `claude`, `claude -p`, no provider SDK imports in any new code).
- [ ] CLI subcommands for each ported pipeline are usable end-to-end against the fixture vault.
- [ ] CGO_ENABLED=0 still passing; grep gates still green.

## Proposed HITL gates

- **P2-W1 gate:** Code review of clients + Embedder interface; table-driven test results; CI grep gate confirming no AI calls.
- **P2-W2 gate:** Parity test review; diff vs Python prototype outputs is empty for every in-scope script.
- **Phase 2 closeout:** Retrospective. Archive P2 wave docs. Rebaseline P3 lookahead into P3 kickoff. Create P4 lookahead.

## Carry-forward from Phase 1

- ARCHITECTURE.md's Embedder interface section informs T04's interface decision.
- The markdown engine + vault walker from Phase 1 are stable — T05's pipeline commands consume them.
- The capture procedure under `testdata/scripts/` is the template T05 extends to capture new pipeline surfaces.
- Sentinel-error + `errors.Is` discipline (test-discipline rule 5) applies to any new failure modes T04/T05 introduce.
- Worktree-first orchestration discipline: each track gets its own worktree at `.worktrees/<track-id>/` on branch `swarm/<track-id>`.

## Key unknowns

- **Exact AI-script inventory for T05.** Some scripts shell out to `claude` CLI (enrich-topics, profile-entities, summarize-transcripts); those are NOT Phase 2. Verify the full classification at P2-W2 kickoff.
- **URL normalization rule set.** The Python prototype's exact tracking-param list and edge cases. Capture from the prototype source at T04 kickoff.
- **`internal/clients/` vs `internal/url/` vs `internal/http/`.** Package boundary decision deferred to T04 kickoff.
- **Embedder interface dimension parameter.** Is dimension fixed at construction time or per-call? Defer to T04 kickoff after reading the prototype's usage.
- **Todoist surface scope.** Whether sync-todoist scripts are in P2 or punted. Verify at P2-W2 kickoff.

## Drift signals

- Phase 2 starting to implement retrieval (drift into Phase 4 / T10) — strictly deterministic clients + pipeline commands only.
- Phase 2 introducing AI calls (drift into Phase 3) — should not happen.
- Hidden dependencies on Python sub-scripts (calling Python from Go) — explicitly forbidden by the "no Python in shipped output" constraint.
- T05 acquiring "improvements" to the prototype's behavior. Parity-first, even where the prototype is awkward.

## Skill attachments

- `git-commit-workflow` at each wave gate and phase closeout.
- `pulse-save` if context-window pressure surfaces between gates.

## Next-phase lookahead

See [v8-phase3-lookahead.md](v8-phase3-lookahead.md) — created at Phase 1 closeout. Rebaselined into Phase 3 kickoff at Phase 2 closeout.
