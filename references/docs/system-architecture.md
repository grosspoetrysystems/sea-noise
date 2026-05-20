# Mind-Palace System Architecture

> Complete reference for the mind-palace knowledge management system.
> This document captures everything needed to reconstruct the system
> from scratch — scripts, schedules, data flows, env vars, and quirks.
>
> **Related docs:**
> - `AGENTS.md` — project overview, pipeline diagrams, script table
> - `.agents/skills/SKILL_MANIFEST.json` — skill registry with deps and env vars
> - `.agents/skills/<name>/SKILL.md` — detailed spec per skill
> - `.agents/go-rewrite-plan.md` — Go rewrite plan (active)
> - `.claude/CLAUDE.md` — Claude system instructions (triage, synthesis, taxonomy)
> - Suite context: `~/Local/chops/.agents/SUITE_VISION.md` — how mind-palace deploys under chops (systemd scheduling, chopsd governance, email ingestion layer). Mind-palace is standalone; the suite doc captures the composed vision.

## Overview

Mind-palace is a personal knowledge management system built on:
- **Obsidian** — front-end (reading, writing, linking notes)
- **Claude Code CLI** (`claude -p`) — AI backbone (inbox triage, synthesis, enrichment)
- **macOS LaunchAgents** — scheduling (30-min inbox, daily, weekly cadences)
- **Python scripts** — glue (web crawling, Todoist sync, RAG indexing, NER) — being rewritten to Go
- **LanceDB + sentence-transformers** — semantic search (RAG) — being replaced with SQLite + API embeddings
- **Cloudflare /crawl** — optional web content fetching
- **Todoist API** — task management sync

The vault itself lives in iCloud at:
```
~/Library/Mobile Documents/iCloud~md~obsidian/Documents/mind-palace/
```

---

## Directory Structure

```
mind-palace/
├── inbox/                          — Raw captures landing zone
├── notes/
│   ├── fleeting/                   — Stage 1 processed, awaiting daily review
│   ├── ideas/                      — Promoted original thinking (#idea)
│   ├── todos/                      — Extracted action items (#todo)
│   ├── references/                 — Web clips, articles (#reference)
│   │   └── briefs/                 — Auto-generated research briefs
│   ├── entities/                   — Auto-generated entity profiles (#entity)
│   └── assertions/                 — Active intentions/goals (#assertion)
├── atlas/                          — Maps of Content (one per domain)
│   └── index.md                    — Master vault orientation
├── bloom/                          — Elaboration tier
│   ├── YYYY-MM-DD-weekly.md        — Weekly synthesis + provocations
│   ├── assertion-tracker.md        — Rolling assertion dashboard
│   └── YYYY-MM-DD-reflect.md       — /reflect session companion pieces
├── signal/
│   └── queue.md                    — Generated read queue dashboard
├── daily/
│   └── YYYY-MM-DD.md              — Daily notes with note summaries
├── _meta/
│   ├── .daily-last-run             — Date marker (YYYY-MM-DD)
│   ├── .weekly-last-run            — Date marker (YYYY-MM-DD)
│   ├── crawl-failures.json         — Retry manifest with backoff
│   ├── processing-logs/            — Inbox triage logs per day
│   ├── topic-frequencies.md        — Tag mention tracking + trends
│   ├── enrichment-log.md           — Research brief generation log
│   ├── entity-profiles-log.md      — Entity profiling log
│   ├── broken-links.md             — Audit report from audit-links.py
│   ├── rejected-connections.md     — Link rejection log
│   ├── taxonomy-proposals.md       — Proposed new tags (user review)
│   ├── claude-md-changelog.md      — Self-improvement proposals
│   ├── pipeline-health.md          — Auto-updated failure log (visible in Obsidian)
│   └── discarded/                  — Removed/duplicate content with reasons
├── _scripts/                       — All automation scripts (see below)
├── _rag/
│   ├── vault.db/                   — LanceDB vector database
│   ├── indexer.py                  — Embedding + indexing script
│   └── mcp_server.py              — MCP server for semantic search
├── AGENTS.md                       — Project overview + pipeline diagrams
├── ALERT.md                        — Pinnable status page (embeds pipeline-health)
├── .agents/                        — Agent/skill definitions + planning
│   ├── system-architecture.md      — This file
│   ├── go-rewrite-plan.md          — Go rewrite plan
│   ├── product-vision.md           — Three-layer product strategy
│   └── skills/
│       ├── SKILL_MANIFEST.json     — Skill registry
│       ├── inbox-pipeline/SKILL.md
│       ├── daily-pipeline/SKILL.md
│       ├── weekly-pipeline/SKILL.md
│       ├── fetch-stubs/SKILL.md
│       ├── sync-todoist/SKILL.md
│       ├── generate-signal/SKILL.md
│       ├── enrich-topics/SKILL.md
│       ├── profile-entities/SKILL.md
│       ├── audit-links/SKILL.md
│       ├── rag-index/SKILL.md
│       └── summarize-transcripts/SKILL.md
├── .claude/
│   ├── CLAUDE.md                   — Claude system instructions (points to .agents/)
│   └── commands/
│       ├── reflect.md              — /reflect slash command
│       ├── signal.md               — /signal slash command
│       └── research.md             — /research slash command
├── AGENTS.md                       — Project overview + pipeline diagrams
└── .obsidian/                      — Obsidian app config
```

---

## Content Pipeline

### Stage 1: Capture → Inbox

Sources:
- **iOS Web Clipper** — Obsidian Share Sheet shortcut saves stub .md to inbox/
  - Frontmatter: title, source URL, tags: [clippings], consumed: false
  - Body is minimal (stub) — hydrated later by fetch-stub-content.py
- **Voice Capture** — Raw dictation dumped to inbox/ (filler words, no frontmatter)
- **Ad Hoc Notes** — Typed directly into inbox/

### Stage 2: Inbox Processing (every 30 min)

Chain executed by `com.mindpalace.inbox.plist`:
1. `fetch-stub-content.py` — Hydrate stubs via Cloudflare, deduplicate
2. `claude -p "process my inbox"` — Triage: detect source type, clean, tag, rename, move to fleeting/
3. `fetch-youtube-transcript.py` — Append transcripts for YouTube clips
4. `indexer.py` — Rebuild vector index
5. `sync-todoist.py` — Push new #todo notes to Todoist
6. `run-daily-catchup.sh` — Marker-gated daily synthesis (if not yet today)
7. `run-weekly-catchup.sh` — Marker-gated weekly synthesis (Sun/Mon only)

### Stage 3: Daily Synthesis (once per day)

Chain executed by `run-daily-catchup.sh` (marker: `_meta/.daily-last-run`):
1. `fetch-stub-content.py --retry` — Retry failed crawls
2. `summarize-transcripts.py` — Enrich with ## Research Context via Claude
3. `indexer.py` — Rebuild vector index
4. `claude -p "daily synthesis"` — Review fleeting/, add wikilinks, promote to final dirs, update MOCs
5. `generate-signal.py` — Regenerate read queue
6. `sync-todoist.py` — Push new todos
7. `sync-todoist-completions.py` — Pull completed todos from Todoist

### Stage 4: Weekly Synthesis (Sunday/Monday)

Chain executed by `run-weekly-catchup.sh` (marker: `_meta/.weekly-last-run`):
1. `fetch-stub-content.py --retry` — Retry failed crawls
2. `claude -p "weekly synthesis"` — Patterns, provocations, MOC hygiene, assertion tracker
3. `indexer.py` — Rebuild vector index
4. `generate-signal.py` — Regenerate read queue
5. `audit-links.py` — Scan for broken wikilinks
6. `enrich-topics.py` — Generate research briefs for topics ≥5 notes
7. `profile-entities.py` — Generate entity profiles for entities ≥5 mentions

---

## Scripts Reference

All scripts live in `$MIND_PALACE/_scripts/`.

### fetch-stub-content.py (~371 lines)
**Purpose:** Hydrate mobile web clipper stubs with full content; deduplicate.
- Default mode: process inbox stubs
- `--retry` mode: retry from `_meta/crawl-failures.json`
- Dedup: normalizes URLs (strips tracking params, fragments, trailing slashes), scans fleeting/references/ideas/ for existing sources
- Duplicates → `_meta/discarded/` with reason
- Backoff: 6h → 24h → 72h → 168h, abandon after 4 failures
- **Env:** `CF_ACCOUNT_ID`, `CF_API_TOKEN` (optional — skips hydration if unset)

### fetch-youtube-transcript.py (~75 lines)
**Purpose:** Fetch transcripts for YouTube video clips.
- Scans fleeting/ + references/ + inbox/ for youtube.com source URLs
- Appends `## Transcript` section to note
- **Deps:** `youtube_transcript_api`

### summarize-transcripts.py (~231 lines)
**Purpose:** Enrich transcripts and web clips with Claude-generated Research Context.
- Transcripts: must have `## Transcript` section
- Web clips: must have source URL or clippings tag, ≥500 chars body
- Adds sections: Key Takeaways, Follow-up Questions, Related Materials, Book & Podcast Recs
- Auto-extracts Tier 2 domain tags via Claude, merges into frontmatter
- Min content: 500 chars (skips memes/short content)
- Max content: 80,000 chars (truncates)
- **Runs in daily chain only** (not inbox) to save API calls
- **Deps:** `claude -p` (subprocess)

### sync-todoist.py (~78 lines)
**Purpose:** Push new todo notes to Todoist.
- Routing: #work → TODOIST_WORK_ID, #personal → TODOIST_PERSONAL_ID, else → TODOIST_INBOX_ID
- Passes `due_string` + `due_lang: en` for NLP date parsing
- Regex safety net: "5p"→"5pm", strips "before"/"by"
- Adds Obsidian deep link in task description
- Updates frontmatter: `synced-todoist: <task_id>`, `synced-todoist-url: <url>`
- **Env:** `TODOIST_API_TOKEN`, `TODOIST_WORK_ID`, `TODOIST_PERSONAL_ID`, `TODOIST_INBOX_ID`

### sync-todoist-completions.py (~76 lines)
**Purpose:** Pull completed tasks from Todoist, mark done in vault.
- Fetches from Todoist Sync API v9
- Searches `notes/todos/` for matching `synced-todoist` frontmatter
- Sets `done: true`, `completed-date: YYYY-MM-DD`
- **Env:** `TODOIST_API_TOKEN`

### generate-signal.py (~266 lines)
**Purpose:** Build `signal/queue.md` — curated read queue dashboard.
- Scoring: assertion tag overlap (+3), rising topic (+2), recency ≤7d (+1)
- Buckets by read time: Quick Hits (<10m), Medium (10-30m), Deep Dives (30m+)
- Word count estimation: 250 WPM articles, 150 WPM transcripts
- Input: notes with `consumed: false` from fleeting/ + references/
- Output: `signal/queue.md`
- Pure Python, no Claude calls — fast

### enrich-topics.py (~350 lines)
**Purpose:** Generate research briefs for topics reaching critical mass.
- Auto mode: scans `_meta/topic-frequencies.md` for topics with count ≥5
- Manual: `enrich-topics.py "ai-agents"` or `--note path/to/note.md`
- Uses Cloudflare /crawl for web research (falls back to vault-only + Claude)
- Output: `notes/references/briefs/YYYY-MM-DD-<topic>-research-brief.md`
- Single note mode: appends enrichment sections directly to note
- Log: `_meta/enrichment-log.md`
- **Env:** `CF_ACCOUNT_ID`, `CF_API_TOKEN` (optional)

### profile-entities.py (~300 lines)
**Purpose:** Auto-generate profiles for people/companies/concepts with 5+ mentions.
- NER: regex extraction of capitalized phrases, structural text stripped
- Claude pre-filter: single call to classify real entities vs false positives
- Max 5 profiles per run
- Output: `notes/entities/YYYY-MM-DD-profile-<slug>.md`
- Tags: #entity, #entity/person|company|concept, #entity-profile
- Log: `_meta/entity-profiles-log.md`

### audit-links.py (~70 lines)
**Purpose:** Scan vault for broken [[wikilinks]].
- Collects all note stems as valid targets
- Regex scans all .md files for [[link]] patterns
- Output: `_meta/broken-links.md`

### run-daily-catchup.sh
**Purpose:** Marker-gated daily synthesis orchestration.
- Marker: `_meta/.daily-last-run` (contains YYYY-MM-DD)
- Skips if already ran today
- Full chain: retry crawls → summarize → index → daily synthesis → signal → todoist push+pull
- Called by both inbox plist (every 30 min) and daily plist (6 AM)

### run-weekly-catchup.sh
**Purpose:** Marker-gated weekly synthesis orchestration.
- Marker: `_meta/.weekly-last-run` (contains YYYY-MM-DD)
- Day check: only fires on Sunday (7) or Monday (1) as grace day
- Week tracking: ISO week comparison
- Full chain: retry crawls → weekly synthesis → index → signal → audit → enrich → profile
- Called by both inbox plist (every 30 min) and weekly plist (Sun 9 AM)

---

## RAG System

Located in `$MIND_PALACE/_rag/`.

### indexer.py (~150 lines)
- **Model:** `BAAI/bge-base-en-v1.5` (768-dim, cosine similarity)
- **DB:** LanceDB at `_rag/vault.db`, table "notes"
- **Strategy:** Drop + recreate (not incremental)
- **Excludes:** _rag, _meta, templates, .git, .obsidian, signal, bloom
- **Fields:** id, path, title, tags, content (first 4000 chars), vector
- **Index:** PQ index created if ≥256 rows
- **Deps:** `sentence_transformers`, `lancedb`, `pyarrow`, `frontmatter`

### mcp_server.py (~116 lines)
- **Tool:** `search_vault(query: str, top_k: int = 5)`
- Embeds query with same model, cosine search, returns top_k results
- Registered as project-scoped MCP server (vault-rag) in Claude CLI
- **Deps:** `mcp`, `sentence_transformers`, `lancedb`

---

## LaunchAgents

All plist files in `~/Library/LaunchAgents/`.

### com.mindpalace.inbox.plist
- **Schedule:** StartCalendarInterval — Minute: 0 and Minute: 30 (every 30 min)
- **Program:** `/bin/sh -c` with full pipeline chain
- **Env:** All API keys + PATH hardcoded in plist EnvironmentVariables dict
- **Chain resilience:** Core steps use `&&`. Todoist uses `|| true` (non-fatal).
  Daily/weekly catchup use `;` (always attempt regardless of prior failures).
- **Log:** `/tmp/mindpalace-inbox.log`

### com.mindpalace.daily.plist
- **Schedule:** StartCalendarInterval — Hour: 6, Minute: 0
- **Program:** `run-daily-catchup.sh`
- **Env:** All API keys + PATH hardcoded in plist EnvironmentVariables dict
- **Log:** `/tmp/mindpalace-daily.log`

### com.mindpalace.weekly.plist
- **Schedule:** StartCalendarInterval — Weekday: 0 (Sunday), Hour: 9, Minute: 0
- **Program:** `run-weekly-catchup.sh`
- **Env:** All API keys + PATH hardcoded in plist EnvironmentVariables dict
- **Log:** `/tmp/mindpalace-weekly.log`

### com.mindpalace.env.plist
- **RunAtLoad:** true
- **Injects via launchctl setenv:** all API keys (hardcoded values)
- **Note:** Backup mechanism. Primary injection is via each plist's EnvironmentVariables.
  `launchctl setenv` unreliable on Sequoia.

### Pipeline Resilience (fixed 2026-03-15)
- Each step in catchup scripts runs independently via `run_step()` wrapper
- Failures are logged to `_meta/pipeline-health.md` (visible in Obsidian via `ALERT.md`)
- Marker is written even on partial failure to prevent retry storms
- No `set -e` — one broken step doesn't kill the entire pipeline
- Todoist sync is non-fatal in inbox chain (`|| true`)

---

## Environment Variables

| Variable | Source | Used By |
|----------|--------|---------|
| `TODOIST_API_TOKEN` | plist EnvironmentVariables + .zshrc | sync-todoist.py, sync-todoist-completions.py |
| `TODOIST_WORK_ID` | plist EnvironmentVariables + .zshrc | sync-todoist.py |
| `TODOIST_PERSONAL_ID` | plist EnvironmentVariables + .zshrc | sync-todoist.py |
| `CF_ACCOUNT_ID` | plist EnvironmentVariables + .zshrc | fetch-stub-content.py, enrich-topics.py |
| `CF_API_TOKEN` | plist EnvironmentVariables + .zshrc | fetch-stub-content.py, enrich-topics.py |
| `HF_TOKEN` | plist EnvironmentVariables | sentence-transformers model download |
| `TRANSFORMERS_VERBOSITY` | plist EnvironmentVariables | suppress HuggingFace warnings |
| `CLAUDECODE` | set to "" in scripts | allow nested claude -p invocation |

---

## Claude Integration

### System Instructions (.claude/CLAUDE.md)
354 lines defining:
- Vault structure and directory mapping
- Tag taxonomy (4 tiers: type, domain, people, context)
- `process my inbox` command — triage logic for voice/web/ad-hoc captures
- `daily synthesis` command — wikilinks, promotion, MOC updates, assertion tracking
- `weekly synthesis` command — patterns, provocations, hygiene, taxonomy evolution
- Assertion lifecycle (active → evidence tracking → stale detection)
- Signal queue rules (consumed field, scoring, stale items)
- Bloom tier (weekly synthesis, assertion tracker, reflect companions)
- Crawl retry and dedup rules
- Vault hygiene imperatives

### Slash Commands (.claude/commands/)
- `/reflect [topic]` — Interactive vault exploration with semantic search
- `/research [topic|--note path]` — Trigger research enrichment pipeline
- `/signal [time]` — Curated reading recommendations by time budget

### MCP Servers
- **vault-rag** — Project-scoped, search_vault tool for semantic search
- **context7** — Global, library documentation lookup

---

## Tag Taxonomy

### Tier 1 — Type (mutually exclusive, determines directory)
| Tag | Directory |
|-----|-----------|
| #idea | notes/ideas/ |
| #todo | notes/todos/ |
| #reference | notes/references/ |
| #entity | notes/entities/ |
| #assertion | notes/assertions/ |
| #fleeting | notes/fleeting/ |
| #question | notes/fleeting/ (stays until answered) |

### Tier 2 — Domain (nested, slash-separated, multiple OK)
tech/ai, tech/ai-agents, tech/context-engineering, tech/dev-tools, tech/rag,
tech/knowledge-graphs, tech/software-design, tech/software-engineering,
life/cooking, life/health, life/fitness, life/sports, life/music,
life/personal-growth, culture/criticism, culture/rationalism, culture/tech,
science/*, learning/*, business/strategy, business/ops

### Tier 3 — People
#people/<name-slug> (only when person is central to the note)

### Tier 4 — Context/Source
#work, #personal, #entity-profile, #reference/brief, #web-clipper,
#voice-capture, #voice-todo, #clippings

---

## Python Dependencies

```
frontmatter              — YAML frontmatter parsing
requests                 — HTTP (Cloudflare, Todoist APIs)
youtube_transcript_api   — YouTube transcript fetching
sentence_transformers    — Embeddings (BAAI/bge-base-en-v1.5)
lancedb                  — Vector database
pyarrow                  — LanceDB dependency
mcp                      — Model Context Protocol server framework
```

---

## Known Quirks

### macOS LaunchAgent (Sequoia / Darwin 24.6)
- **StartInterval is BROKEN** — never fires ("pended nondemand spawn = speculative")
- **StartCalendarInterval WORKS** — used exclusively for all scheduling
- Must set PATH in EnvironmentVariables (/opt/homebrew/bin not in launchd default)
- Background Items toggles in System Settings must be enabled
- **com.apple.provenance BLOCKS .sh scripts in iCloud Drive** — Sequoia adds
  provenance xattr to files created by non-trusted processes (Claude Code, iCloud
  sync). Launchd refuses to execute them: "Operation not permitted". `xattr -cr`
  does NOT work — iCloud re-adds the attribute. **Fix:** use Python wrapper scripts
  (`run_daily.py`, `run_weekly.py`) that call `/bin/bash` on the .sh files via
  subprocess. Python scripts aren't blocked because `python3` is the executable.
- **launchctl setenv unreliable** — env vars must be hardcoded in each plist's
  EnvironmentVariables dict, not injected via a separate env plist

### LanceDB (Python 3.14, lancedb 0.29)
- `db.list_tables()` returns `ListTablesResponse` — must use `.tables` attribute
- ANN index (`create_index`) requires ≥256 rows; skip for small vaults
- `table.to_pandas()` requires pandas installed separately

### Claude -p Headless Mode
- Runs on Max plan (no API key needed)
- Set `CLAUDECODE=""` to allow nested invocation from scripts
- Timeout: 300-600 seconds depending on script complexity

### iCloud Path
- Full path contains spaces: `~/Library/Mobile Documents/iCloud~md~obsidian/Documents/mind-palace/`
- All scripts must quote paths
- Go rewrite separates binary from vault — binary points to vault via `mind-palace.toml` config

---

## iOS Capture Flow

1. User shares URL via iOS Share Sheet
2. "CLIPPER" Shortcut creates stub .md in inbox/ via iCloud sync
3. Stub has frontmatter (title, source, tags: clippings, consumed: false) but no body
4. Next 30-min cycle: fetch-stub-content.py hydrates via Cloudflare /crawl
5. Dedup check runs before hydration (normalized URL comparison)
6. `claude -p "process my inbox"` triages the hydrated note
7. Note lands in notes/fleeting/ (or notes/todos/ if task detected)

---

## Marker-Gated Execution

Both daily and weekly chains use date-based marker files to ensure
they run at most once per period, even though they're triggered
every 30 minutes by the inbox chain.

**Daily:** `_meta/.daily-last-run` contains YYYY-MM-DD of last run.
Skip if file contains today's date.

**Weekly:** `_meta/.weekly-last-run` contains YYYY-MM-DD of last run.
Skip if same ISO week. Only attempt on Sunday (preferred) or Monday (grace).

This design handles macOS sleep/wake gracefully — if the machine is
asleep at 6 AM, the next inbox fire at :00 or :30 catches up.

---

## Key File Paths (Absolute)

```
Vault root:       ~/Library/Mobile Documents/iCloud~md~obsidian/Documents/mind-palace/
Scripts:          <vault>/_scripts/                    (current — moves into Go binary)
RAG:              <vault>/_rag/                        (current — replaced by Go binary)
Agent docs:       <vault>/.agents/                     (system architecture, skills, plans)
Skill manifest:   <vault>/.agents/skills/SKILL_MANIFEST.json
Claude config:    <vault>/.claude/CLAUDE.md
Slash commands:   <vault>/.claude/commands/
Project overview: <vault>/AGENTS.md
LaunchAgents:     ~/Library/LaunchAgents/com.mindpalace.*.plist
Claude global:    ~/.claude/settings.json              (MCP servers, hooks)
```
