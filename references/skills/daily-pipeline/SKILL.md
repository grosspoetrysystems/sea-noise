# Daily Pipeline

## Purpose
Once-per-day synthesis. Reviews fleeting notes, adds wikilinks, promotes to final directories, updates MOCs, tracks assertions, generates signal queue, syncs Todoist completions.

## Chain
Each step runs independently via `run_step()` — failures are logged but don't block others.
```
fetch-stub-content.py --retry    → Retry failed crawls from manifest
summarize-transcripts.py         → Enrich transcripts + web clips (Claude)
_rag/indexer.py                  → Rebuild vector index
claude -p "daily synthesis"      → Wikilinks, promotion, MOCs, assertions
generate-signal.py               → Regenerate read queue dashboard
sync-todoist.py                  → Push new todos (non-fatal)
sync-todoist-completions.py      → Pull completed todos (non-fatal)
```

## Resilience
- No `set -e` — each step runs independently
- Failures logged to `_meta/pipeline-health.md` (visible in Obsidian via `ALERT.md`)
- Marker written even on partial failure (prevents retry storms)

## Marker Gate
- **File:** `_meta/.daily-last-run` (contains YYYY-MM-DD)
- **Logic:** Skip if file contains today's date
- **Orchestrator:** `_scripts/run-daily-catchup.sh`
- **Callers:** inbox plist (every 30 min catch-up) + daily plist (6 AM preferred)

## Claude Synthesis Logic
Defined in `.claude/CLAUDE.md` under "Command: daily synthesis":
1. Catch-up check: scan for missing daily notes, process gaps
2. Read each fleeting note, identify themes
3. Scan vault for related notes, add [[wikilinks]]
4. Log rejected connections to `_meta/rejected-connections.md`
5. Update `_meta/topic-frequencies.md`
6. Find/create atlas/ MOCs, add wikilinks
7. Re-enrich modified notes before promotion
8. Promote by Tier 1 tag to final directory
9. Append summaries to `daily/YYYY-MM-DD.md`
10. Update `bloom/assertion-tracker.md`

## Input
- `notes/fleeting/*.md` — from last 24h (or longer if catching up)

## Output
- `notes/{ideas,references,todos,entities,assertions}/*.md` — promoted
- `daily/YYYY-MM-DD.md` — daily note with summaries
- `atlas/*.md` — updated MOCs
- `bloom/assertion-tracker.md` — updated
- `signal/queue.md` — regenerated
- `_meta/topic-frequencies.md` — updated

## Go Rewrite Target
`mind-palace run daily` — Phase 6 in go-rewrite-plan.md
