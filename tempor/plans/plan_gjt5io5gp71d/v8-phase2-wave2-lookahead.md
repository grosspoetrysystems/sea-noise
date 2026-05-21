# v8 — P2-W2 Wave Lookahead — Pipeline commands

**Plan / Phase / Wave:** `plan_gjt5io5gp71d` / `phase2_deterministic_surface` / `phase2_wave2`
**Wave name:** P2-W2 — Deterministic pipeline commands
**Status:** Lookahead (rebaselined into wave kickoff at the P2-W1 gate).

## Track (proposed)

| Track ID | Title | Worktree branch |
|----------|-------|-----------------|
| T05 | Deterministic pipeline commands | `swarm/t-t05-deterministic-pipeline-commands` |

Single track.

## Scope (proposed)

Port AI-free Python scripts to Go CLI subcommands wired into `cmd/sea-noise`. Candidates from the prototype (verify AI/no-AI status at wave kickoff by reading each script):

- `audit-links.py` → `sea-noise audit-links` (vault walker + wikilink extraction; we already have both)
- `fetch-stub-content.py` → `sea-noise fetch-stub-content` (uses T04 URL client + HTTP fetcher)
- `fetch-youtube-transcript.py` → `sea-noise fetch-youtube-transcript` (uses T04 YouTube ID extractor)
- `generate-signal.py` → `sea-noise generate-signal` (deterministic frontmatter aggregation)
- `sync-todoist*.py` → optional; depends on whether Todoist surface involves AI

Excluded from this wave (Phase 3+):
- `enrich-topics.py` (calls `claude` CLI)
- `profile-entities.py` (calls `claude` CLI)
- `summarize-transcripts.py` (calls `claude` CLI)

## Entry criteria

- [ ] P2-W1 gate passed (URL client + HTTP fetcher + Embedder interface merged).
- [ ] Each candidate script confirmed AI-free by direct source inspection.
- [ ] `requirements.txt` updated with the Python deps the captured-parity needs (likely `python-frontmatter`, `requests`, `youtube-transcript-api`).
- [ ] Capture script in `testdata/scripts/` extended to cover each ported surface, OR per-surface capture scripts added.

## Exit criteria

- [ ] All in-scope pipeline commands implemented and wired into `cmd/sea-noise`.
- [ ] Parity tests: `go test ./...` includes diffs against captured Python outputs on the fixture vault. Zero diffs.
- [ ] CLI subcommands usable end-to-end against the fixture vault.
- [ ] No AI calls in any new code path. Grep gates confirm.
- [ ] CGO_ENABLED=0 still passing.

## Proposed gate

- Parity diff review: each ported script's output diffs against the Python prototype's output on the fixture vault. Zero diffs.
- Manual CLI smoke test: run each subcommand against the fixture vault, verify expected outputs.
- Quality gates: build/test/grep all exit 0.

## Key unknowns (to resolve at kickoff)

- **Script-by-script AI status.** Some scripts may make AI calls in code paths I haven't read closely. Confirm at kickoff by `grep -E 'subprocess|claude' references/scripts/*.py`.
- **Todoist API surface.** Whether the Todoist scripts have AI calls or just API calls. Read carefully.
- **Output format for `audit-links`** — the prototype writes to `_meta/broken-links.md`. Should the Go port do the same? Probably yes for parity.
- **Test capture strategy per command.** Extend `testdata/scripts/capture-parity.py`, or one script per surface? Decide at kickoff based on which is more maintainable.

## Drift signals

- Trying to port AI scripts in this wave (e.g., enrich-topics) — defer to Phase 3.
- "Improving" the prototype's output format. Parity-first; deltas need explicit `KNOWN-DELTA.md` entry in Phase 5.
- New deps creeping in beyond what the prototype already uses.

## Skill attachments (planned)

- `git-commit-workflow` at the gate.
- `pulse-save` if context-window pressure surfaces (likely candidate: porting 4–5 scripts is a lot of code in one wave).

## Rebaseline notes (at promotion to kickoff)

Promote into `v8-phase2-wave2-kickoff.md` once P2-W1 gate passes. Rebaseline against:

- Final package layout from T04 (was it `internal/url/` or `internal/clients/`? does Embedder live where I expected?).
- Updated `requirements.txt` and capture extensions ready.
- Reference commit at P2-W1 gate.
- Confirmed AI-free script inventory.
