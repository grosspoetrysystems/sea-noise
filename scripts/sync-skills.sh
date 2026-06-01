#!/usr/bin/env bash
# Mirror inference skill files from the canonical source location
# (.agents/shared/skills/inference/) into the go:embed target
# location (internal/ai/skill/embed/inference/). The canonical
# location is authoritative; the embed target is a tracked mirror
# so that `go build ./cmd/sea-noise/` produces a self-contained
# binary without requiring a build-time sync step.
#
# Run this after editing any skill file in
# .agents/shared/skills/inference/, before committing.
set -euo pipefail
cd "$(dirname "$0")/.."

SRC=".agents/shared/skills/inference"
DST="internal/ai/skill/embed/inference"

if [[ ! -d "$SRC" ]]; then
  echo "sync-skills: source dir $SRC missing" >&2
  exit 1
fi
mkdir -p "$DST"

# Wipe stale files in the mirror so removals propagate.
find "$DST" -maxdepth 1 -name '*.md' -delete

# Copy every *.md from source.
for f in "$SRC"/*.md; do
  cp "$f" "$DST/"
done

count=$(find "$DST" -maxdepth 1 -name '*.md' | wc -l | tr -d ' ')
echo "sync-skills: mirrored $count skill file(s) from $SRC to $DST"
