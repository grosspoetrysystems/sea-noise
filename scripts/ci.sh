#!/usr/bin/env bash
# Single CI gate. Runs everything the workflow at .github/workflows/ci.yml
# runs, so contributors can reproduce CI locally with one command.
#
#   build  → CGO-disabled go build (scripts/build.sh)
#   test   → full go test ./... including parity tests (scripts/test.sh)
#   gates  → architectural invariants (scripts/grep-gates.sh)
#   hooks  → node --test for the rule-injector hook (.agents/shared/hooks/
#            and .claude/shared/hooks/)
#
# Per .tempor/exec/P1-W2-kickoff.md T-CI scope: no matrix, no caching
# tuning, no flakiness retry. Minimize first; layer up after proof.
set -euo pipefail
cd "$(dirname "$0")/.."

./scripts/build.sh
./scripts/test.sh
./scripts/grep-gates.sh
node --test .agents/shared/hooks/*.test.mjs .claude/shared/hooks/*.test.mjs
