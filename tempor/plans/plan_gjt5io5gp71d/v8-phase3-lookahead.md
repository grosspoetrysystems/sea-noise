# v8 — Phase 3 Lookahead — Router seam + LocalRouter

**Plan / Phase:** `plan_gjt5io5gp71d` / `phase3_router_seam`
**Phase name:** Phase 3 — Router seam + LocalRouter
**Status:** Lookahead (rebaselined into Phase 3 kickoff at Phase 2 closeout).

## Objective

Define the inference eval router seam and wire AI command wrappers through it. The seam is harness-agnostic (per `ARCHITECTURE.md` §6) — clean internal interface, no anticipated external swap, no Claude-CLI-specific coupling in the routing decision.

## Scope (proposed — 2 tracks across 2 waves)

| Wave | Track | Title |
|------|-------|-------|
| P3-W1 | T06 | Inference eval router seam (interface + stub) |
| P3-W2 | T07 | Claude integration + AI command wrappers |

## Dependency order

```
(Phase 2 complete: deterministic clients, Embedder interface, pipeline commands)
            │
            ▼
          T06 ──→ T07
```

T06 defines `internal/ai/inferencerouter.Router` interface + `LocalRouter` stub (deterministic Go decision logic; **not** a subprocess to `claude -p`). T07 implements the AI command wrappers that obtain an `InferenceDecision` from the router before invoking the LLM transport (currently `claude -p`, but the transport is downstream of routing).

## Proposed waves

### P3-W1 — Inference router seam (interface + stub)

Per `ARCHITECTURE.md` §6:

- `internal/ai/inferencerouter/` package: `Router` interface, `InferenceRequest` / `InferenceDecision` typed structs. Specific enums (tier/profile/target) decided here as Sea Noise's own contract — present as part of the seam's design, not as "borrowed" vocabulary.
- `LocalRouter`: deterministic Go decision logic. Zero `exec.Command`, zero outbound `net/http`, zero provider SDK imports, zero model/provider string literals.
- Stub returns sensible defaults so AI command wrappers compile and run.
- CI grep gate confirms zero provider SDK imports under `internal/ai/inferencerouter/`.

### P3-W2 — Claude integration + AI command wrappers

- Port the AI-using Python scripts (`enrich-topics.py`, `profile-entities.py`, `summarize-transcripts.py`) to Go CLI subcommands.
- Each AI command wrapper obtains an `InferenceDecision` from the inference router BEFORE invoking any LLM transport.
- LLM transport for parity: `claude -p` subprocess (matching the Python prototype's transport choice). Mock test asserts the contract "router decision precedes transport call."
- Sentinel errors for transport failures (timeout, non-zero exit, no Claude available).

## Entry criteria

- [ ] Phase 2 closeout passed.
- [ ] `internal/ai/embedder/` (Embedder interface) exists from P2-W1.
- [ ] All Phase 2 deterministic surfaces stable (so AI wrappers can layer on top without churn).

## Exit criteria

- [ ] `internal/ai/inferencerouter/` interface + stub implemented.
- [ ] CI grep gate green: zero provider SDK imports, zero string literals for model/provider IDs anywhere outside `internal/config/`.
- [ ] AI command wrappers obtain InferenceDecision before invoking Claude transport (verified by mock test).
- [ ] All Python AI-using scripts have Go equivalents.
- [ ] CGO_ENABLED=0 still passing; grep gates still green.
- [ ] No retrieval engine work yet (drift check — T10 lives in Phase 4).

## Proposed HITL gates

- **P3-W1 gate:** Code review of router interface + stub; CI grep gate confirms architectural invariants.
- **P3-W2 gate:** Parity test review; AI commands run end-to-end against the fixture vault (or a mock transport for hermetic tests).
- **Phase 3 closeout:** Archive. Rebaseline P4 lookahead into P4 kickoff. Create P5 lookahead.

## Key unknowns

- **InferenceDecision shape.** What fields does the decision need? Tier (SIMPLE/MEDIUM/COMPLEX/REASONING)? Profile? Model hint? Decide at P3-W1 kickoff, document in `internal/ai/inferencerouter/doc.go`.
- **Routing heuristics for LocalRouter.** Pure default behavior, or simple input-based classification? Default (return MEDIUM/auto/smart for everything) is enough for parity; smarter routing is future work.
- **Claude transport invocation pattern.** The prototype uses `claude -p` via `subprocess`. Go equivalent uses `os/exec`. Subprocess timeout, stdin/stdout handling, error parsing — match the prototype.
- **Mock transport for tests.** Likely an interface `internal/ai/transport.Transport` that the real `claude -p` impl and a `mock.Transport` both satisfy. Pin at P3-W2 kickoff.

## Drift signals

- T06 making routing decisions by calling an LLM (defeats the purpose of having a router).
- T07 invoking Claude before obtaining InferenceDecision from the router.
- Provider/model strings appearing outside `internal/config/`.
- Phase 3 starting retrieval implementation (drift into Phase 4).

## Skill attachments (planned)

- `git-commit-workflow` at each gate.
- `pulse-save` if porting AI scripts hits context limits.

## Rebaseline notes (at promotion to kickoff)

Promote into `v8-phase3-kickoff.md` once Phase 2 closes. Rebaseline against:
- Final URL client + HTTP fetcher API from T04.
- Embedder interface signature as finalized in T04.
- Pipeline command patterns from T05 (the AI wrappers should follow the same CLI subcommand structure).
- Updated unknowns from Phase 2 outcomes.
- Reference commit at Phase 2 closeout.
