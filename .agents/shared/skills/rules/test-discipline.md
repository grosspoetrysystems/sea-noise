---
name: test-discipline
description: Sea Noise testing discipline — eight rules for writing extensible, composable, non-brittle Go tests.
globs: **/*_test.go
kind: rule
---

# test-discipline

Rules for writing and editing tests in this repo. The hook fires this rule whenever you Write or Edit a `*_test.go` file. Read before touching one.

## Canon

- `internal/config/config_test.go` — table-driven helper test (`TestSplitKeyValue`), `t.TempDir()` / `t.Setenv()` for hermetic state, `errors.Is` for sentinel-error assertions, locally-scoped `chdir` helper.
- `internal/config/config.go` — sentinel errors (`ErrNotConfigured`, `ErrPathMissing`, `ErrPathNotDir`) wrapped with `%w` so the message wording can evolve without breaking tests.
- `scripts/grep-gates.sh` — architectural invariants enforced as part of the test signal: model-name leakage, Python subprocess invocations, MCP imports.

## The eight rules

### 1. Tests land with implementation, not after

No implementation merges to `main` without tests for every meaningful behavior it introduces. Doc-only changes do not require tests. A "meaningful behavior" is any code path a caller can rely on or a regression could break — not every unexported helper, but every exported function and every observable side effect.

### 2. Table-driven for transforms

Parsers, normalizers, extractors, splitters, and any function shaped `input → output` get a single table-driven test. New cases are a struct entry, not a new test function. Use `t.Run(name, ...)` to surface individual case failures.

### 3. Hermetic via `t.TempDir()` and `t.Setenv()`

Tests do not touch the real filesystem outside their `t.TempDir()`. Tests do not read or write env vars without `t.Setenv()`. Tests can run in any order, in parallel, on any machine, with no shared state. A test that requires a specific CWD, env var, or pre-existing file is broken — fix it.

### 4. Golden files for parity work

Parity tests diff Go output against committed Python prototype reference output under `testdata/expected-*/`. Zero-diff or test fails. The fixture vault is the substrate; expected outputs are captured deterministically and committed. Updating a golden file is a deliberate, reviewable change — not a casual `-update` flag flip.

### 5. Sentinel errors over message strings

Use `errors.Is(err, ErrXYZ)` to lock in failure modes. Never use `strings.Contains(err.Error(), "...")` — that couples tests to error wording, which should be free to evolve.

```go
// Bad — brittle on wording:
if !strings.Contains(err.Error(), "not a directory") { ... }

// Good — locks the contract:
if !errors.Is(err, config.ErrPathNotDir) { ... }
```

If a failure mode is worth asserting in tests, it is worth defining as a sentinel error in the producing package.

### 6. No premature mocking

Use real types, real disk via `t.TempDir()`, real config files, real HTTP servers via `httptest`. Mocks come only when there is a costly external dependency — e.g., an LLM transport, where we mock the transport interface rather than spending tokens per test run. Mocking internal package boundaries is a smell; if the boundary needs a mock, the boundary is wrong.

### 7. Helpers extracted progressively

Test helpers (`chdir`, fixture loaders, builders) live in the same file as their consumer until a second consumer needs them. At two consumers, lift to a `testhelper` package or a `_test.go` file shared across the package. Do not build `testhelper` infrastructure before two real consumers exist.

### 8. CI gates are tests too

`scripts/grep-gates.sh` enforces architectural invariants and runs in CI alongside `go test`. A grep-gate failure is a test failure. Do not silence a gate; fix the violation or narrow the gate with a comment explaining why.

## Non-obvious

- **`t.Setenv("VAR", "")` is not the same as unsetting.** It sets the env var to the empty string. Read `os.Getenv` and treat empty string as "not configured" if you need that distinction.
- **`t.Cleanup` runs in LIFO order.** If a test sets up multiple `Cleanup`s, the last registered runs first.
- **`t.TempDir()` is per-test, not per-package.** Each `t` (sub-test or top-level) gets its own. Don't try to share a tempdir across test functions — make a helper that returns one.
- **`t.Parallel()` interacts with `t.Setenv`.** A parallel test that calls `t.Setenv` is fine; Go arranges process-level env safety. But if you've factored env mutation into a helper, the helper must take `*testing.T` and call `t.Setenv` so the cleanup is correctly scoped.
- **`errors.Is` traverses `%w`-wrapped chains.** When wrapping a sentinel with `fmt.Errorf("...: %w", ErrXYZ)`, `errors.Is(err, ErrXYZ)` returns true. Multiple wraps are fine. Use `errors.As` for typed error values that carry data.
- **Time and randomness are not hermetic by default.** A test that uses `time.Now()` or `rand.Int()` directly will be flaky or non-deterministic. Inject a clock/randomness source, or pin via `time.Date(...)` constants in test setup.

## Brittleness sources to watch for

- Error message wording → use sentinel errors (rule 5)
- Map iteration order → sort or compare as sets
- Floating-point exact equality → use tolerance, or fixed decimal types
- Wall-clock time → inject a clock, or use `time.Date` constants
- File ordering on disk → sort filesystem walk results before comparing
- Goroutine scheduling → use channels and `<-done`, never `time.Sleep`
- Random ports → use `httptest.Server` which picks a free port and gives you its URL

If you find yourself adding `time.Sleep` to "fix a flaky test," stop. The test is wrong, not slow.

## Composability cues

A test setup that's ten lines of boilerplate wants a helper. A helper that takes `*testing.T` and returns a struct of pre-wired collaborators is even better — composition becomes one call:

```go
func newConfigFixture(t *testing.T) *configFixture { ... }

func TestSomething(t *testing.T) {
    f := newConfigFixture(t)
    // ... use f.Vault, f.Loader, etc.
}
```

If the same fixture is used by three or more test files in the same package, extract it to its own `*_test.go` (Go's test files share package scope within a directory). If it's used across packages, lift to `testhelper` — but only at that threshold, not earlier.
