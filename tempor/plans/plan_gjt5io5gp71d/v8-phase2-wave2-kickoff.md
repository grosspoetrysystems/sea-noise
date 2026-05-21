# v8 — P2-W2 Wave Kickoff — Pipeline commands

**Plan / Phase / Wave:** `plan_gjt5io5gp71d` / `phase2_deterministic_surface` / `phase2_wave2`
**Wave name:** P2-W2 — Deterministic pipeline commands
**Status:** Active.
**Reference commit:** `6818ba2` (T04 merged: `internal/url/` for normalization + HTTP fetcher, `internal/ai/embedder/` for Embedder interface + Stub, all with table-driven and sentinel-error tests).

## Rebaseline notes

Carry-forward from P2-W1:

- `internal/url/` exports `Normalize`, `ExtractYouTubeID`, `Fetcher` with sentinel errors (`ErrNon2xx`, `ErrTimeout`, `ErrNetwork`). Mirrors `references/scripts/fetch-stub-content.py:normalize_url` and `fetch-youtube-transcript.py:YT_PATTERN` byte-for-byte, including the prototype's first-param tracking-strip quirk (documented in test cases).
- `internal/ai/embedder/` exports `Embedder` interface (`Embed`, `Dimension`, `ProviderID`) plus `Stub` with deterministic zero-vectors. T05 commands that need an embedder use the Stub for now.
- The Fetcher is constructed with `NewFetcher()`; tests inject `httptest.Server` clients via `WithClient`. T05's pipeline commands obtain a Fetcher via dependency injection at the command boundary (no global state).

## Narrowed scope decision

The lookahead listed five candidate scripts. After surveying their sizes and external dependencies:

| Script | Lines | External deps | T05 scope? |
|--------|-------|---------------|----------|
| `audit-links.py` | 69 | none (pure stdlib) | ✅ in scope |
| `generate-signal.py` | 283 | `frontmatter` | ✅ in scope |
| `fetch-stub-content.py` | 440 | `platform_apis` module (not in references/), external scrapers, HTTP | ⛔ deferred |
| `fetch-youtube-transcript.py` | 75 | `youtube_transcript_api`, external API | ⛔ deferred |
| `sync-todoist.py`, `sync-todoist-completions.py` | 178 | Todoist API, credentials | ⛔ deferred |

**Deferral rationale:** the three deferred scripts depend on **external API surfaces** (YouTube transcripts, Todoist, generic web scrapers with platform-specific quirks). Each needs:
- Its own credentials / rate-limit / auth design pass
- A Go-native equivalent for libraries the prototype gets for free (e.g., `youtube_transcript_api`)
- A live-network capture procedure for parity (since outputs depend on external state)

That work belongs in a dedicated wave with its own design constraints, not bundled into "deterministic surface parity port." Document the deferral in `KNOWN-DELTA.md` at Phase 5; revisit the deferred scripts in a follow-up wave once the external-API design pattern is established.

In-scope for THIS wave:
- **`audit-links`**: vault walk + wikilink extraction (already implemented in T03) + report writer. Output written to `_meta/broken-links.md` per prototype.
- **`generate-signal`**: frontmatter aggregation for the signal queue. Reads vault, groups by criteria, writes report.

## Tracks

| Track ID | Title | Owner | Execution mode | Workspace |
|----------|-------|-------|----------------|-----------|
| T05 | Deterministic pipeline commands (narrowed: audit-links + generate-signal) | unassigned | foreground-sequential | `.worktrees/t-t05-deterministic-pipeline-commands` (branch `swarm/t-t05-deterministic-pipeline-commands`) |

Single track. Two CLI subcommands ported within it.

## Scope

### `sea-noise audit-links`

Mirrors `references/scripts/audit-links.py`:
- Walk vault with `LinksExcludeDirs` exclude set.
- Extract all wikilinks per note via `markdown.ExtractWikilinks`.
- For each link target, check if its lowercased stem appears in the vault's stem set.
- Emit a report at `_meta/broken-links.md` listing broken links grouped by source note.
- Report header includes the date (deterministic — accepts an injected clock for hermetic tests).

CLI:
```
sea-noise audit-links             # uses vault from config, writes _meta/broken-links.md
sea-noise audit-links --dry-run   # prints report to stdout, doesn't write
```

### `sea-noise generate-signal`

Mirrors `references/scripts/generate-signal.py`. Surface to nail at wave-execution time by reading the prototype source carefully (283 lines — needs a focused pass). Likely involves:
- Read notes via vault walker
- Parse frontmatter for signal criteria
- Aggregate/group notes
- Emit signal queue file (probably `signal/queue.md`)
- Possibly use clock injection for date handling (parity-honest)

Verify exact surface at start of work. The Python prototype is canonical.

## Acceptance criteria

- [ ] `cmd/sea-noise` wires `audit-links` and `generate-signal` subcommands.
- [ ] `sea-noise audit-links` against the T09 fixture vault produces output matching the Python prototype's output on the same fixture (parity test diffs against a new golden file in `testdata/expected/audit-links/`).
- [ ] `sea-noise generate-signal` against the T09 fixture vault produces output matching the Python prototype's output (parity test, golden file at `testdata/expected/generate-signal/`).
- [ ] `testdata/scripts/capture-parity.py` extended to capture both surfaces from the prototype, OR per-surface capture scripts added with `capture-audit-links.py` / `capture-generate-signal.py` parallel pattern.
- [ ] CGO_ENABLED=0 still passing.
- [ ] No AI calls in any new code (grep gates green).
- [ ] No new external Go deps (the deferred scripts would have needed them; in-scope scripts are pure stdlib + existing internal packages).

## HITL gate

- Parity diff review: each subcommand produces zero-diff output vs Python on the fixture vault.
- CLI smoke test: run each subcommand against the fixture vault, verify expected outputs match what's on disk after the prototype run.
- Build / test / grep gates all exit 0.

## Skill attachments

- `git-commit-workflow` at the gate.
- `pulse-save` if context gets heavy (porting 283-line Python script is a lot of context).

## Drift signals

- Attempting to port the deferred scripts in this wave (defeats the deferral decision; reschedule into a dedicated wave).
- "Improving" the prototype's report format or signal grouping logic. Parity-first; deltas need `KNOWN-DELTA.md` entries in Phase 5.
- Adding external Go deps (`golang.org/x/text/encoding`, third-party YAML parsers, etc.) without a documented reason — the parity surface should be stdlib unless a clear gap surfaces.
- Generate-signal acquiring AI-call code paths (drift into Phase 3).

## Next-wave lookahead

This is the **final wave of Phase 2**. No next-wave lookahead within phase. Phase-level next-horizon is [v8-phase3-lookahead.md](v8-phase3-lookahead.md) (Router seam + Claude integration), rebaselined into Phase 3 kickoff at Phase 2 closeout.

The deferred scripts (`fetch-stub-content`, `fetch-youtube-transcript`, `sync-todoist*`) need a home. Likely:
- Phase 3 if framing as "AI/external integration" (they're external API integration, even if not LLM)
- Or a Phase 5 "external surfaces" sub-wave

That allocation decision happens at Phase 2 closeout.
