# v8 — P3-W1 Wave Kickoff — Inference router seam

**Plan / Phase / Wave:** `plan_gjt5io5gp71d` / `phase3_router_seam` / `phase3_wave1`
**Wave name:** P3-W1 — Inference router seam (interface + stub)
**Status:** Active.
**Reference commit:** `3fa6047` (Phase 3 promoted active).

## Track

| Track ID | Title | Worktree | Branch |
|----------|-------|----------|--------|
| T06 | Inference eval router seam (interface + stub) | `.worktrees/t-t06-inference-router-seam` | `swarm/t-t06-inference-router-seam` |

## Decisions (pinned for this wave)

- **Vocabulary** (Sea Noise's own contract, not borrowed from any external product):
  - `ClassificationTier`: `Unspecified`, `Simple`, `Medium`, `Complex`, `Reasoning`
  - `RoutingProfile`: `Unspecified`, `Auto`, `Eco`, `Premium`, `Free`
  - `LocalTarget`: `Unspecified`, `Fast`, `Smart`, `Heavy`
  - Zero-value is `Unspecified` so callers can leave fields unset and the router applies its defaults.
- **`InferenceRequest` fields**: minimal — `Tier`, `Profile`, `InputSize int`, `Purpose string` (freeform telemetry tag). Grow when concrete callers in T07 surface needs.
- **`InferenceDecision` fields**: `Tier`, `Profile`, `Target`. No model/provider strings — transport layer resolves those from config keyed on Target.
- **LocalRouter behavior**: return sensible defaults (`Medium / Auto / Smart`) regardless of input. Real heuristics are post-parity work; the stub's job is to make AI command wrappers compile and run.
- **Enum String / Parse**: implemented so config files can use lowercase names (`profile = "premium"`).

## Acceptance criteria

- [ ] `internal/ai/inferencerouter/` exports `Router` interface, `InferenceRequest`, `InferenceDecision`, three enums.
- [ ] `LocalRouter` satisfies `Router`. Compile-time assertion via `var _ Router = (*LocalRouter)(nil)`.
- [ ] `LocalRouter.Route` returns the sensible-defaults decision; verified by table-driven test.
- [ ] Each enum has `String()` (canonical lowercase) and `Parse*` (from string, with `ErrInvalid*` sentinel for unknown values).
- [ ] CGO_ENABLED=0 build passes; `scripts/grep-gates.sh` green.
- [ ] Package contains zero `exec.Command`, zero `net/http`, zero provider SDK imports, zero model/provider string literals.

## HITL gate

- Build / test / grep all exit 0.
- Code review of the interface shape — confirm fields and enums match the kickoff's pinned decisions.
- No drift into transport / Claude-CLI territory (that's T07).

## Next wave

P3-W2 (T07 — Claude integration + AI command wrappers). Wave kickoff just-in-time when work starts.
