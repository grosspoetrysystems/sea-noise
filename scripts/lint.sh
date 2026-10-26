#!/usr/bin/env bash
# Run formatting and static analysis through golangci-lint v2.
set -euo pipefail
cd "$(dirname "$0")/.."

golangci-lint fmt --diff
golangci-lint run ./...
