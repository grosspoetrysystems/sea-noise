# Weekly Pipeline

## Purpose
Weekly meta-synthesis. Reads compressed vault via MOCs, identifies patterns and gaps, writes provocations, runs vault hygiene, generates research briefs, profiles entities.

## Chain
```
fetch-stub-content.py --retry   → Retry failed crawls
claude -p "weekly synthesis"    → Patterns, provocations, MOC hygiene
_rag/indexer.py                 → Rebuild vector index
generate-signal.py              → Regenerate read queue
audit-links.py                  → Broken [[wikilink]] scan
enrich-topics.py                → Research briefs for topics ≥5 notes
profile-entities.py             → Entity profiles for mentions ≥5
```

## Marker Gate
- **File:** `_meta/.weekly-last-run` (contains YYYY-MM-DD)
- **Logic:** Skip if same ISO week. Only fires Sunday (preferred) or Monday (grace).
- **Orchestrator:** `_scripts/run-weekly-catchup.sh`
- **Callers:** inbox plist (every 30 min catch-up) + weekly plist (Sun 9 AM)

## Claude Synthesis Logic
Defined in `.claude/CLAUDE.md` under "Command: weekly synthesis":
1. Read all atlas/ MOCs as compressed vault index
2. Identify 3-5 disconnected clusters sharing implicit themes
3. Scan topic-frequencies for topics >5 in last 30 days
4. Read this week's bloom/*-reflect.md companion pieces
5. Write `bloom/YYYY-MM-DD-weekly.md` with:
   - Emerging patterns, surprise connections, latent priorities
   - ## Provocations — evocative writing prompts rooted in vault tensions
6. Vault hygiene: orphans, stale fleeting, near-duplicates
7. Catch-up audit for missing daily notes
8. Discarded cleanup
9. Self-improvement proposals → `_meta/claude-md-changelog.md`
10. Taxonomy evolution → `_meta/taxonomy-proposals.md`

## Output
- `bloom/YYYY-MM-DD-weekly.md` — weekly insight + provocations
- `bloom/assertion-tracker.md` — fully regenerated
- `_meta/broken-links.md` — audit report
- `notes/references/briefs/*.md` — new research briefs
- `notes/entities/*.md` — new entity profiles
- `signal/queue.md` — regenerated

## Go Rewrite Target
`mind-palace run weekly` — Phase 6 in go-rewrite-plan.md
