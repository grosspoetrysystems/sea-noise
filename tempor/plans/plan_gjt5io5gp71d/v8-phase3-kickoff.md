# v8 — Phase 3 Kickoff — Router seam + LocalRouter

**Plan / Phase:** `plan_gjt5io5gp71d` / `phase3_router_seam`
**Phase name:** Phase 3 — Router seam + LocalRouter
**Status:** Active.
**Reference commit:** `48c9d1c` (Phase 2 final commit: yaml.v3 dep, ParseFrontmatterYAML, python-frontmatter requirement, audit-links subcommand).

## Rebaseline notes

Promoted from `v8-phase3-lookahead.md` at Phase 2 closeout. Carry-forward from Phase 2:

- **`internal/url/`** ships `Normalize`, `ExtractYouTubeID`, and a `Fetcher` with sentinel errors (`ErrNon2xx`, `ErrTimeout`, `ErrNetwork`). T07's AI command wrappers will use the Fetcher for any HTTP transport they need.
- **`internal/ai/embedder/`** ships the `Embedder` interface + `Stub` returning zero-vectors. AI command wrappers that need an embedder use the Stub for now.
- **`internal/markdown/`** ships both `ParseFrontmatter` (indexer-parity) and `ParseFrontmatterYAML` (full YAML, using `gopkg.in/yaml.v3`). AI command wrappers that need to read frontmatter from notes use `ParseFrontmatterYAML`.
- **`cmd/sea-noise`** has a working subcommand dispatcher. T07 wires AI subcommands into the same pattern (`sea-noise enrich-topics`, `sea-noise profile-entities`, etc.).
- **CI gates** stay enforced: `CGO_ENABLED=0` build, `go test ./...`, grep gates for model literals / Python subprocesses / MCP imports.
- **Test discipline** rule fires automatically via PreToolUse hook on every `*_test.go` edit. Sentinel errors + `errors.Is`, table-driven, hermetic.

## Deferred from Phase 2 — generate-signal

`generate-signal.py` was deferred mid-execution due to scope. Its eventual home isn't decided yet — options:
- A Phase 3 sub-wave alongside the AI wrappers, since it's adjacent to "what does the agent prioritize reading next" surfaces.
- A Phase 5 "miscellaneous AI-free surfaces" sub-wave.
- A standalone follow-up wave after Phase 5.

Decide at the gate where it best fits. Infrastructure for it (yaml.v3, python-frontmatter, ParseFrontmatterYAML) is already in place.

## Objective

Define the inference eval router seam and wire AI command wrappers through it. The seam is harness-agnostic (per `ARCHITECTURE.md` §6): clean internal interface, no anticipated external swap, no Claude-CLI-specific coupling in the routing decision.

## Scope (2 tracks across 2 waves)

| Wave | Track | Title |
|------|-------|-------|
| P3-W1 | T06 | Inference eval router seam (interface + stub) |
| P3-W2 | T07 | Claude integration + AI command wrappers |

## Dependency order

```
(Phase 2 complete: deterministic clients, Embedder iface, audit-links, yaml infra)
            │
            ▼
          T06 ──→ T07
```

T06 defines `internal/ai/inferencerouter.Router` interface + `LocalRouter` stub (deterministic Go decision logic; **not** a subprocess to `claude -p`). T07 implements the AI command wrappers that obtain an `InferenceDecision` from the router before invoking the LLM transport.

## Wave-level scope summaries

### P3-W1 — Inference router seam (T06)

Per `ARCHITECTURE.md` §6:
- `internal/ai/inferencerouter/` exports `Router` interface, `InferenceRequest` / `InferenceDecision` typed structs, classification enums.
- `LocalRouter` is deterministic Go decision logic. Zero `exec.Command`, zero outbound `net/http`, zero provider SDK imports, zero model/provider string literals.
- Stub returns sensible defaults so AI command wrappers compile and run.
- CI grep gate confirms architectural invariants.

Full P3-W1 wave kickoff doc is written when T06 starts (just-in-time, per the deterministic-ceremonies discipline).

### P3-W2 — Claude integration + AI command wrappers (T07)

- Port the AI-using Python scripts (`enrich-topics.py`, `profile-entities.py`, `summarize-transcripts.py`) to Go CLI subcommands.
- Each AI command wrapper obtains an `InferenceDecision` from the inference router BEFORE invoking any LLM transport.
- LLM transport for parity: `claude -p` subprocess (matching the Python prototype's transport choice).
- Sentinel errors for transport failures (timeout, non-zero exit, no Claude available).
- Mock transport for hermetic tests.

Full P3-W2 wave kickoff doc is written when T07 starts.

## Entry criteria

- [x] Phase 2 closeout passed.
- [x] `internal/ai/embedder/` (Embedder interface) exists.
- [x] All Phase 2 deterministic surfaces stable.
- [ ] `claude` CLI available on the execution host (T07 only; T06 doesn't need it).

## Exit criteria (Phase 3)

- [ ] `internal/ai/inferencerouter/` interface + stub implemented and tested.
- [ ] CI grep gate green: zero provider SDK imports under the inference router, zero string literals for model/provider IDs anywhere outside `internal/config/`.
- [ ] All AI-using Python scripts have Go subcommand equivalents.
- [ ] AI command wrappers obtain InferenceDecision before invoking Claude transport (verified by mock test).
- [ ] CGO_ENABLED=0 still passing; grep gates still green.

## Key unknowns (resolve at P3-W1 kickoff)

- **InferenceDecision shape** — what fields the decision carries (tier, profile, model hint, max-tokens hint, etc.).
- **Routing heuristics for LocalRouter** — pure default behavior, or simple input-based classification.
- **Claude transport invocation pattern** — subprocess timeout, stdin/stdout handling, error parsing.
- **Mock transport interface** for hermetic tests.

## Drift signals

- T06 making routing decisions by calling an LLM (defeats the seam's purpose).
- T07 invoking Claude before obtaining InferenceDecision from the router.
- Provider/model strings appearing outside `internal/config/`.
- Phase 3 starting retrieval implementation (drift into Phase 4 / T10).

## Next-phase lookahead

See [v8-phase4-lookahead.md](v8-phase4-lookahead.md) — created at Phase 2 closeout. Rebaselined into Phase 4 kickoff at Phase 3 closeout.
