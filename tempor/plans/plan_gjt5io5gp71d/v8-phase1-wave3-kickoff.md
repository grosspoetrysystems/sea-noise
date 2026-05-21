# v8 — P1-W3 Wave Kickoff — Vault + Fixture (parallel)

**Plan / Phase / Wave:** `plan_gjt5io5gp71d` / `phase1_foundation` / `phase1_wave3`
**Wave name:** P1-W3 — Vault + fixture (parallel)
**Status:** Active.
**Reference commit:** `9766ffa` (T02 merged: go module, CLI scaffold, config loader, CI gates, brittleness fix, rule-injection hook, AGENTS.md/CLAUDE.md entry points)

## Rebaseline notes

Promoted from the wave 3 lookahead at the P1-W2 gate. Carry-forward:

- Go module is named `sea-noise`. Package layout at `internal/vault/`, `internal/markdown/`, `internal/retrieval/`, `internal/ai/embedder/`, `internal/ai/inferencerouter/`, `internal/config/`. All `doc.go`-only stubs except `internal/config/` which has the real loader.
- `internal/config.LoadVaultPath()` resolves `MIND_PALACE_VAULT` env (priority) or `vault.path` in `mind-palace.toml` — and returns sentinel errors (`ErrNotConfigured`, `ErrPathMissing`, `ErrPathNotDir`) wrapped via `%w`. Both T03 and T09 use it where they need vault path resolution.
- Sentinel-error + `errors.Is` pattern is locked in (test-discipline rule 5). Any new test for T03 should follow the same shape.
- The PreToolUse hook injects [`test-discipline`](../../../.agents/shared/skills/rules/test-discipline.md) when editing any `**/*_test.go` file. Honor those eight rules when authoring tests in this wave.
- AGENTS.md is the agent-neutral source of truth at repo root; CLAUDE.md is just `@AGENTS.md`. Do not author Claude-specific content elsewhere.

## Tracks (parallel)

| Track ID | Title | Worktree | Branch |
|----------|-------|----------|--------|
| T03 | Markdown engine + vault abstraction | `.worktrees/t-t03-markdown-engine-vault-abstraction` | `swarm/t-t03-markdown-engine-vault-abstraction` |
| T09 | Golden fixture vault | `.worktrees/t-t09-golden-fixture-vault` | `swarm/t-t09-golden-fixture-vault` |

T03 and T09 have no cross-dependency and run in parallel. **First wave with intra-wave parallelism in this plan.**

## Scope

### T03 — Markdown engine + vault abstraction

Implement under `internal/markdown/` and `internal/vault/` (the `doc.go` stubs from T02 are replaced with real code):

- **Frontmatter parser** (YAML metadata at top of each note). Honor the Python prototype's exact parsing behavior: which YAML library/quirks it uses, what fields it extracts, how it handles missing/malformed frontmatter.
- **Wikilink extractor** — `[[Note]]`, `[[Note|alias]]`, `[[Note#section]]`. Match the prototype's exact regex/parsing rules.
- **Tag extractor** — frontmatter `tags:` list AND inline `#tag` matches in body text. Honor the prototype's stop-character set.
- **Backlink computation** — derived from forward links; same shape as the prototype.
- **Vault walker** — uses `internal/config.LoadVaultPath()` to resolve the vault, walks markdown files, returns note records.
- **Note ID strategy** — honor the prototype's exact scheme. Likely path-derived but verify against `references/scripts/` behavior.
- **Tests** — table-driven for each extractor + parity tests that diff Go output against captured Python prototype output on the T09 fixture vault.

### T09 — Golden fixture vault

Author under `testdata/fixture-vault/` and `testdata/expected-*/`:

- **Sourced from the live iCloud vault** (revised from the lookahead's "hand-authored" stance). 20–30 representative notes copied verbatim — higher parity fidelity than reimagined content, and faster than reconstructing realistic note shapes from scratch. Pinned at copy time; live vault edits do not propagate. Selection criteria: covers the 7 case types observed in the prototype (stub clip, transcript note, assertion, fleeting note, weekly bloom, topic frequencies, signal queue) plus structural variation (FTS-exercising body text, tags both frontmatter and inline, wikilinks plain/aliased/section, entities, assertions, recency variation).
- **Expected output fixtures** under `testdata/expected-*/` (per-script subdirectory):
  - For each parity-critical surface (frontmatter parsing, link extraction, tag extraction, signal generation, etc.), capture what the Python prototype emits on the fixture vault, and commit it as a golden file.
  - Capture method documented in a `testdata/CAPTURING-FIXTURES.md` so the procedure is reproducible.
- **Python capture environment**: use whatever Python is locally available at the maintainer's machine for the initial capture (single maintainer, one machine, this is acceptable). A pinned venv with `requirements.txt` is a deliberate future change — easy to add when (a) a second maintainer joins, (b) CI runs golden captures, or (c) the prototype's Python deps drift. Capture procedure should record the Python version and key package versions for forensic reproducibility even without a pinned venv.

T09 produces no Go code beyond optionally a small `testdata/capture.sh` helper. The deliverables are committed markdown notes and committed golden output files.

## Coordination between T03 and T09

- T03 cannot validate its parity tests without T09's golden outputs. T09's capture procedure depends on knowing what Go output shape T03 produces, which depends on the prototype's behavior. Resolution: T09 captures the **Python prototype's** output as-is; T03's parity tests then diff Go output against those committed Python outputs.
- T09 lands first if there's any sequencing — its fixture vault is the substrate T03's tests run against. If T03 hits questions about prototype behavior, run the prototype against T09's fixture vault to find the answer.

## Entry criteria

- [x] P1-W2 gate passed (T02 merged at `9766ffa`).
- [x] Go toolchain available (`go version` reports >= 1.22).
- [x] `internal/config.LoadVaultPath()` works end-to-end (verified by T02 tests).
- [ ] Python prototype runs against an arbitrary directory (verify before fixture capture).
- [ ] A scratch vault path is set aside for fixture authoring (NOT the live iCloud vault).

## Acceptance criteria (combined)

**T03:**
- [ ] `internal/markdown/` and `internal/vault/` packages implement frontmatter parsing, wikilink/tag extraction, vault walking with note records.
- [ ] `go test ./internal/markdown/... ./internal/vault/...` passes.
- [ ] Parity tests diff Go output against captured Python prototype output on the T09 fixture vault. Zero diffs.
- [ ] Backlink computation is correct (derived from forward links).
- [ ] CGO_ENABLED=0 still passes.
- [ ] Grep gates still green (no model literals, no Python invocations in Go, no MCP).
- [ ] Tests honor the test-discipline rule: `t.TempDir()`, `t.Setenv()`, table-driven for transforms, `errors.Is` for sentinel errors, no `time.Sleep`, no premature mocking.

**T09:**
- [ ] `testdata/fixture-vault/` committed with 20–30 notes copied from the live iCloud vault, covering all 7 case types.
- [ ] Structural variation present: FTS-exercising text, tags (frontmatter + inline), wikilinks (plain + aliased + section), entities, assertions, recency variation.
- [ ] `testdata/expected-*/` committed with golden outputs for representative surfaces (at minimum: frontmatter parsing, link extraction, tag extraction). Captured deterministically from the Python prototype against the fixture vault.
- [ ] `testdata/CAPTURING-FIXTURES.md` documents the capture procedure including Python version + key package versions for forensic reproducibility (pinned venv deferred per "single maintainer, single machine" stance).
- [ ] Fixture notes are pinned (committed once; live vault edits do not propagate). Subsequent live-vault deltas are out of scope unless explicitly chosen.

## HITL gates

- **T03 gate:** Code review of markdown engine + vault abstraction; parity test results (zero diff vs Python prototype on fixture vault); table-driven tests follow test-discipline rules.
- **T09 gate:** Fixture review (does the vault cover all 7 case types and the structural variation needed?); expected-output review; capture procedure reviewed.
- **Wave gate (combined):** Both tracks closed; ready for Phase 1 closeout.

## Resolved decisions (from T02 outputs)

- **Module name**: `sea-noise`.
- **Package paths**: `internal/markdown` (parsing), `internal/vault` (walker, note records).
- **Config plumbing**: `internal/config.LoadVaultPath()` is the only place that resolves vault path. T03's walker calls it, never resolves env or files itself.
- **Test discipline**: rule 5 applies — sentinel errors wrapped with `%w`, tests use `errors.Is`. Define `internal/markdown.ErrXYZ` and `internal/vault.ErrXYZ` as needed.

## Key unknowns (resolve as discovered)

- **Exact frontmatter quirks** in the Python prototype (which YAML library, handling of missing/malformed frontmatter). Read the Python source under `references/scripts/`.
- **Inline tag delimiter rules** — what characters end an inline `#tag`? The prototype's behavior is canonical; document the rule in a code comment.
- **Wikilink edge cases** — `[[]]` (empty)? `[[A|]]` (alias with empty)? Capture the prototype's behavior, encode as table-driven test cases.
- **Note ID derivation** — strict path-from-vault-root? Filename without extension? Frontmatter `id` override if present? Match the prototype.
- **Backlink ordering** — sorted by path? By first-link-seen? Deterministic order matters for golden file stability.
- **7 case-type definitions** — these come from the prototype. Read its scripts to confirm what "stub clip", "transcript note", "weekly bloom", etc., shapes look like in practice.

## Drift signals (watch list)

- T03 starting to implement FTS, SQLite, or any retrieval logic (drift into Phase 4 / T10). Strictly markdown parsing + vault walking only.
- T03 making "improvements" to the Python prototype's extraction (drift away from parity-first). When in doubt, mirror the prototype's behavior even if it's not the ideal design.
- T09 fixture vault auto-syncing with the live iCloud vault (drift away from pinned-and-deterministic). Notes are copied once and committed; never wired to re-sync from the live vault.
- Fixture vault scope creeping toward "exhaustive coverage." Keep it representative (20–30 notes), not exhaustive.
- Tests using `time.Sleep`, `time.Now()`, `strings.Contains(err.Error(), ...)`, or any of the brittleness sources from the test-discipline rule.

## Skill attachments

- `git-commit-workflow` — required at each gate.
- `pulse-save` — eject if context-window pressure surfaces during parallel track work. Likely candidate here.

## Worktree orchestration

This is the first wave with intra-wave parallelism. Both tracks run as background-parallel under `git_worktree_per_track`:

```sh
git worktree add .worktrees/t-t03-markdown-engine-vault-abstraction -b swarm/t-t03-markdown-engine-vault-abstraction
git worktree add .worktrees/t-t09-golden-fixture-vault -b swarm/t-t09-golden-fixture-vault
```

Both branches are merged separately to main at their respective track gates. Wave closeout happens when both tracks are merged.

## Next-wave lookahead

This is the **final wave of Phase 1**. There is no next-wave lookahead within this phase. The phase-level next-horizon document is [v8-phase2-lookahead.md](v8-phase2-lookahead.md), which gets rebaselined into the Phase 2 kickoff at Phase 1 closeout.

At Phase 1 closeout, additionally create:
- `v8-phase2-kickoff.md` (rebaseline of `v8-phase2-lookahead.md` against P1 outcomes)
- `v8-phase2-wave1-kickoff.md` (T04 — Deterministic clients + Embedder interface)
- `v8-phase2-wave2-lookahead.md` (T05 — Deterministic pipeline commands)
- `v8-phase3-lookahead.md` (next-phase horizon)
