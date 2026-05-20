#!/usr/bin/env bash
# Run the Go test suite.
set -euo pipefail
cd "$(dirname "$0")/.."
go test ./...
