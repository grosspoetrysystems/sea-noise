# v8 — P2-W1 Wave Kickoff — Clients + Embedder interface

**Plan / Phase / Wave:** `plan_gjt5io5gp71d` / `phase2_deterministic_surface` / `phase2_wave1`
**Wave name:** P2-W1 — Deterministic clients + Embedder interface
**Status:** Active.
**Reference commit:** `b5009ff` (Phase 1 final commit; updated at Phase 1 closeout merge).

## Track

| Track ID | Title | Owner | Execution mode | Workspace |
|----------|-------|-------|----------------|-----------|
| T04 | Deterministic clients + Embedder interface | unassigned | background-parallel | `.worktrees/t-t04-deterministic-clients-embedder-iface` (branch `swarm/t-t04-deterministic-clients-embedder-iface`) |

Single track.

## Scope

### URL normalization

Mirror the Python prototype's URL handling per `fetch-stub-content.py` and any other URL-touching script. At minimum:

- Strip tracking parameters (full set from the prototype — verify at start of wave).
- Strip URL fragments.
- Strip trailing slashes.
- Extract YouTube video IDs into a canonical form (`youtu.be/X`, `youtube.com/watch?v=X`, `youtube.com/embed/X`, etc. → just `X`).
- Table-driven tests against representative inputs, including malformed URLs (parity behavior: pass through, don't crash).

Package: `internal/url/` (decision pin: `url` over `clients` to keep the package narrow and aligned with the stdlib `net/url` shape). Final package decision goes at wave kickoff if a strong reason to deviate surfaces.

### HTTP fetcher

A small wrapper around `net/http`:
- `Fetch(ctx, url) (body []byte, contentType string, status int, err error)`
- No retries at this layer; T05's pipeline commands choose retry behavior.
- Honors `context.Context` for cancellation.
- Sentinel errors for: timeout, non-2xx response, network failure.

Package: `internal/url/` or `internal/http/`. Decide at wave kickoff.

### Embedder interface

Per `ARCHITECTURE.md` §5. In `internal/ai/embedder/`:

```go
type Embedder interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    Dimension() int
    ProviderID() string
}
```

Plus a stub implementation `StubEmbedder` that returns zero-vectors of the configured dimension. No concrete real implementation in this wave — Phase 4 (T10) decides whether to ship a semantic layer.

## Acceptance criteria

- [ ] `internal/url/` (or chosen package) implements URL normalization with table-driven tests.
- [ ] HTTP fetcher implemented with sentinel errors (`errors.Is`-checkable).
- [ ] `internal/ai/embedder/` exports `Embedder` interface, `Dimension()`, `ProviderID()`, plus `StubEmbedder` returning deterministic zero-vectors.
- [ ] CGO_ENABLED=0 build passes.
- [ ] Grep gates green: no model literals, no Python invocations in Go, no MCP imports.
- [ ] No outbound HTTP from within `internal/ai/embedder/` (the Embedder interface doesn't make calls itself; concrete impls would).
- [ ] Tests honor the test-discipline rule (hermetic via `httptest.Server` for HTTP fetcher, table-driven for URL normalization).

## HITL gate

- Code review focused on package boundaries (`internal/url/`, `internal/ai/embedder/`).
- Confirm URL normalization matches the prototype on representative inputs (run the prototype against a few URLs, diff outputs).
- Confirm Embedder interface shape matches `ARCHITECTURE.md` §5.
- CI grep gates green.
- Quality gates: `scripts/build.sh && scripts/test.sh && scripts/grep-gates.sh` all exit 0.

## Skill attachments

- `git-commit-workflow` at the gate.

## Drift signals

- HTTP fetcher growing retry/backoff logic (push that to T05).
- URL normalization growing into a full URL parser (use `net/url` for parsing; normalization is the only added behavior).
- Embedder interface getting a concrete real implementation (deferred to Phase 4).
- Any AI provider SDK appearing in `internal/ai/embedder/` or its callers (Phase 3+ concern).

## Next-wave lookahead

See [v8-phase2-wave2-lookahead.md](v8-phase2-wave2-lookahead.md) — promote to wave kickoff at the P2-W1 gate.
