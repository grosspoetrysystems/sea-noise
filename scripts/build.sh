#!/usr/bin/env bash
# Build the sea-noise binary with CGO disabled. Failure here means a
# transitive dependency dragged in CGO — investigate and remove.
set -euo pipefail
cd "$(dirname "$0")/.."
CGO_ENABLED=0 go build ./...
