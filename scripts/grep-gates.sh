#!/usr/bin/env bash
# Enforce the architectural dependency and embedded-skill invariants.
#
# Each gate looks for forbidden patterns in Go source. Any non-zero count
# fails the gate. The script exits non-zero if any gate fails.
#
# The gates are intentionally conservative — false positives (e.g., a
# legitimate use of "python" in a comment) should be addressed by
# refactoring or by narrowing the gate, not by silencing it.
set -eu
cd "$(dirname "$0")/.."

fail=0

# count_hits runs a grep pipeline that may legitimately produce zero
# matches (grep exit 1) and prints the line count to stdout. We avoid
# pipefail so the "no match" case is a clean 0.
count_hits() {
  pattern="$1"
  exclude="$2"
  grep -rE "${pattern}" --include='*.go' . 2>/dev/null \
    | grep -v -E "${exclude}" \
    | wc -l \
    | tr -d ' '
}

# Gate 1: no provider/model name string literals outside config/ and tests.
count=$(count_hits 'claude-3|gpt-4|gemini|text-embedding' '(config|/tests?/|_test\.go)' || true)
if [ "${count}" -ne 0 ]; then
  echo "GATE FAIL: model/provider literals found outside config/ (${count} hits):"
  grep -rE 'claude-3|gpt-4|gemini|text-embedding' --include='*.go' . \
    | grep -v -E '(config|/tests?/|_test\.go)' || true
  fail=1
fi

# Gate 2: no Python subprocess invocations in Go source.
count=$(count_hits '"(python|python3)"' '(_test\.go|/tests?/)' || true)
if [ "${count}" -ne 0 ]; then
  echo "GATE FAIL: Python subprocess invocations in Go source (${count} hits):"
  grep -rE '"(python|python3)"' --include='*.go' . \
    | grep -v -E '(_test\.go|/tests?/)' || true
  fail=1
fi

# Gate 3: no MCP server / transport imports.
# Look for import paths that contain "mcp" in their last segment.
count=$(count_hits '"[^"]*/mcp(/|")' '(_test\.go|/tests?/)' || true)
if [ "${count}" -ne 0 ]; then
  echo "GATE FAIL: MCP imports in Go source (${count} hits):"
  grep -rE '"[^"]*/mcp(/|")' --include='*.go' . \
    | grep -v -E '(_test\.go|/tests?/)' || true
  fail=1
fi

# Gate 5: keep the CLI and config loader on the standard-library path.
# Checks both Go source and go.mod.
count=$(count_hits '"[^"]*/(kong|koanf)(/|")' '(_test\.go|/tests?/)' || true)
if [ "${count}" -ne 0 ]; then
  echo "GATE FAIL: Kong or koanf imports in Go source (${count} hits):"
  grep -rE '"[^"]*/(kong|koanf)(/|")' --include='*.go' . \
    | grep -v -E '(_test\.go|/tests?/)' || true
  fail=1
fi
if grep -qE '(kong|koanf)' go.mod 2>/dev/null; then
  echo "GATE FAIL: Kong or koanf entry found in go.mod:"
  grep -nE '(kong|koanf)' go.mod
  fail=1
fi

# Gate 6: embedded skill mirror is in sync with the canonical source.
# .agents/shared/skills/inference/ is the source of truth; the
# go:embed target at internal/ai/skill/embed/inference/ must match
# byte-for-byte. Drift means `scripts/sync-skills.sh` wasn't run
# after a skill edit, and the shipped binary would carry stale
# embedded content.
src_dir=".agents/shared/skills/inference"
mirror_dir="internal/ai/skill/embed/inference"
if [ -d "${src_dir}" ] && [ -d "${mirror_dir}" ]; then
  if ! diff -r "${src_dir}" "${mirror_dir}" > /dev/null 2>&1; then
    echo "GATE FAIL: embedded skill mirror out of sync with ${src_dir}."
    echo "Run scripts/sync-skills.sh to refresh ${mirror_dir} before committing."
    diff -r "${src_dir}" "${mirror_dir}" || true
    fail=1
  fi
fi

if [ "${fail}" -ne 0 ]; then
  echo "" >&2
  echo "One or more grep gates failed. See ARCHITECTURE.md §6 and §7 and .tempor/KNOWN-DELTA.md." >&2
  exit 1
fi

echo "All grep gates passed."
