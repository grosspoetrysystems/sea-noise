# v8 — P1-W2 Wave Lookahead — Project Scaffold

**Plan / Phase / Wave:** `plan_gjt5io5gp71d` / `phase1_foundation` / `phase1_wave2`
**Wave name:** P1-W2 — Project scaffold + CLI skeleton
**Status:** Lookahead (groomed at project start; rebaselined into wave kickoff at P1-W1 gate).

## Track (proposed)

| Track ID | Title | Owner | Execution mode | Workspace |
|----------|-------|-------|----------------|-----------|
| T02 | Project scaffold + CLI skeleton | unassigned | background-parallel | `.worktrees/t-t02-project-scaffold-cli` (branch `swarm/t-t02-project-scaffold-cli-skeleton`) |

Single track.

## Scope (proposed)

T02 stands up the Go module, CLI skeleton, package layout, base config plumbing, and CI gate stubs. Specifically:

1. `go mod init` with the chosen module name (resolve at kickoff).
2. Package layout: at minimum `cmd/`, `internal/`, plus a clear convention for `internal/vault`, `internal/markdown`, `internal/ai`, `internal/retrieval` (created as empty packages or with placeholder files).
3. CLI entry point that compiles and runs `--help`. Framework choice (cobra vs stdlib flag) resolved at kickoff.
4. Config loader for `mind-palace.toml` with `MIND_PALACE_VAULT` env override. Returns a resolved vault path; errors clearly if neither is set.
5. CI gate stubs:
   - `CGO_ENABLED=0 go build ./...` script.
   - `go test ./...` script.
   - `grep` gate scripts for forbidden imports (`exec.Command` in retrieval/router packages once they exist, model literals outside config).
6. README updates that explain the build/test/grep gates.

## Entry criteria

- [ ] P1-W1 gate passed (ARCHITECTURE.md committed).
- [ ] Go toolchain available on the execution host.
- [ ] Module name and CLI framework decision made at this wave's kickoff.

## Exit criteria

- [ ] `CGO_ENABLED=0 go build ./...` exits 0 on the scaffold.
- [ ] `go test ./...` exits 0 (no tests yet, or smoke test on config loader).
- [ ] CLI `--help` runs.
- [ ] Config loader resolves vault path from `mind-palace.toml` and `MIND_PALACE_VAULT` env (priority order documented).
- [ ] CI grep gates exist as scripts; pass on the scaffold (no violations yet because there's no code).
- [ ] No Python invocations in the scaffold; no MCP imports.
- [ ] No model name literals outside an explicit `config/` package.

## Proposed gate (HITL)

- Build/test review on the scaffold.
- Smoke test the CLI manually with a fake vault path.
- Confirm worktree primitives work cleanly (provision a test worktree, verify isolation).
- Promote P1-W3 lookahead to wave kickoff for T03 || T09 parallel tracks.

## Key unknowns (to resolve at kickoff)

- Module name (e.g., `github.com/<user>/sea-noise` vs `<user>/sea-noise` — affects import paths in every subsequent file).
- CLI framework choice. Lean: stdlib `flag` for now to avoid the dependency footprint cobra introduces; cobra can be added later if subcommand sprawl warrants. **Recommend deferring cobra until subcommand needs are concrete.**
- Whether `mind-palace.toml` lives at repo root or under a config subdirectory. Lean: repo root for discoverability, since users edit it.
- Whether tests need a real fixture vault yet (likely no — defer real fixtures to T09 / P1-W3).

## Carry-forward considerations

- Any decisions captured in ARCHITECTURE.md (P1-W1 output) about package boundaries should be honored by the scaffold layout.
- The Embedder interface defined in ARCHITECTURE.md should have a corresponding empty `internal/ai/embedder` package stub.

## Drift signals (watch list)

- Scaffolding starts implementing retrieval logic (drift into Phase 4) — keep scaffold to package layout + CLI entry + config + CI gates.
- Premature dependency additions (cobra, viper, SQLite driver) — defer to the wave that actually needs them.
- CGO_ENABLED slipping (somehow ending up =1 via transitive deps) — CI gate must fail-fast on this.

## Skill attachments (planned)

- `git-commit-workflow` (required at the wave gate).

## Rebaseline notes (at promotion to kickoff)

Promote this lookahead into `v8-phase1-wave2-kickoff.md` once P1-W1 gate passes. Rebaseline against:

- ARCHITECTURE.md's actual contents (package boundaries, Embedder interface specifics).
- Any unknowns surfaced during T01 that affect the scaffold (e.g., specific schema tables that demand particular package names).
- The repo's current commit ref at P1-W1 gate.
- Any deferred items from P1-W1 (none expected, but capture if surfaced).
