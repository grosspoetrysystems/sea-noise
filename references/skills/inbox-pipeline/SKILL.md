# Inbox Pipeline

## Purpose
Full inbox processing chain. Runs every 30 minutes via LaunchAgent. Captures arrive from iOS Web Clipper (stubs), voice dictation, or typed notes. Pipeline hydrates, triages, transcribes, indexes, and syncs — then attempts daily/weekly catchup.

## Chain
```
fetch-stub-content.py          → Hydrate stubs via Cloudflare, dedup         (&&)
claude -p "process my inbox"   → Triage: detect source, clean, tag, rename   (&&)
fetch-youtube-transcript.py    → Append transcripts for YouTube clips         (&&)
_rag/indexer.py                → Rebuild vector index                         (;)
sync-todoist.py                → Push new #todo notes to Todoist              (|| true, non-fatal)
run-daily-catchup.sh           → Daily synthesis if not yet today             (;, always attempt)
run-weekly-catchup.sh          → Weekly synthesis if Sun/Mon + not this week  (;, always attempt)
```

**Resilience:** Core steps (fetch→claude→youtube→index) use `&&`. Todoist is
non-fatal (`|| true`). Daily/weekly catchup always attempt (`;`) regardless
of prior failures. This prevents a single broken step from cascading.

## Scheduler
- **Plist:** `~/Library/LaunchAgents/com.mindpalace.inbox.plist`
- **Schedule:** StartCalendarInterval — Minute: 0 and Minute: 30
- **Log:** `/tmp/mindpalace-inbox.log`

## Claude Triage Logic
Defined in `.claude/CLAUDE.md` under "Command: process my inbox". Detects three source types:

1. **Web Clipper** — has `source: "https://..."` or `tags: clippings`. Preserve content, extract title, write summary, tag as #reference, keep `consumed: false`.
2. **Voice Capture** — raw dictation with filler words. Clean transcript, preserve raw under ## Raw. Detect todos ("add a todo", "todo:", "task:").
3. **Ad Hoc Note** — clean prose. Do NOT rewrite. Only add missing frontmatter.

All sources: assign Tier 1 + Tier 2 tags, rename to `YYYY-MM-DD-slug.md`, move to `notes/fleeting/` (or `notes/todos/` for tasks).

## Input
- `inbox/*.md` — raw captures

## Output
- `notes/fleeting/*.md` — triaged notes
- `notes/todos/*.md` — extracted tasks
- `_meta/processing-logs/YYYY-MM-DD.md` — triage log

## Go Rewrite Target
`mind-palace run inbox` — Phase 6 in go-rewrite-plan.md
