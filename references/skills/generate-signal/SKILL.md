# Generate Signal

## Purpose
Build `signal/queue.md` — a curated read queue dashboard scoring unconsumed notes by relevance. Pure Python, no Claude calls. Fast.

## Script
`_scripts/generate-signal.py` (266 lines)

## Scoring Algorithm
For each note with `consumed: false`:
- **Assertion tag overlap:** +3 per matching domain tag with active assertions
- **Rising topic:** +2 per match from `_meta/topic-frequencies.md` (trend = rising)
- **Recency:** +1 if note is ≤7 days old

## Read Time Estimation
- Articles: 250 words per minute
- Transcripts: 150 words per minute

## Content Type Lanes (replaces time-based buckets)
- **Priority** — score ≥3, assertion-aligned (across all types)
- **Deep Reads** — essays, articles, threads with a thesis
- **Listen / Watch** — podcasts, videos (detected by source URL)
- **Tools & Reference** — manuals, docs, case studies
- **By Domain** — domain tag counts
- **Stale Queue** — >14 days unconsumed

Content type set via `content-type` frontmatter field. Auto-inferred
from source URL (youtube/spotify → listen-watch) and content heuristics
(code examples/docs → tool-reference) when not explicitly set.

## Filtering
Excluded from the queue entirely:
- `consumed: true` (already read/watched)
- `low-signal: true` (stubs, profile pages, landing pages)
- `mood: true` (anchor items — clipped for reference, not to consume)
- `content-type: anchor` (same as mood, agent-inferred)

## Consumed Sync
`_scripts/sync-consumed.py` reads checked boxes (`- [x]`) in queue.md
and propagates `consumed: true` back to the source note's frontmatter.
Runs in the daily chain before signal regeneration. This enables
checking items off directly in Obsidian.

## Input
- Notes with `consumed: false` from `notes/fleeting/` + `notes/references/`
- `notes/assertions/` — active assertions (for tag overlap scoring)
- `_meta/topic-frequencies.md` — rising topics

## Output
- `signal/queue.md` — formatted dashboard with content-type lanes

## Go command
`sea-noise generate-signal` writes the signal queue.
