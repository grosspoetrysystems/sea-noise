#!/usr/bin/env bash
# Reproduce the required CI checks locally.
#
#   build  → CGO-disabled Go build
#   test   → full Go test suite
#   gates  → architectural invariants and embedded-skill sync
#   hooks  → rule-injector tests
set -euo pipefail
cd "$(dirname "$0")/.."

./scripts/build.sh
./scripts/test.sh
./scripts/grep-gates.sh
node --test .agents/shared/hooks/*.test.mjs .claude/shared/hooks/*.test.mjs
