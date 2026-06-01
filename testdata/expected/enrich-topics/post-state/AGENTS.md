# AGENTS.md

> **Architecture reference:** [.agents/system-architecture.md](.agents/system-architecture.md)
> **Skill manifest:** [.agents/skills/SKILL_MANIFEST.json](.agents/skills/SKILL_MANIFEST.json)
> **Go rewrite plan:** [.agents/go-rewrite-plan.md](.agents/go-rewrite-plan.md)

## Project

Mind-palace is a personal knowledge management system built on Obsidian with automated capture, triage, synthesis, and enrichment. Claude Code CLI (`claude -p`) handles AI reasoning. Python scripts handle deterministic orchestration. macOS LaunchAgents run the schedules. The vault lives in iCloud; the automation is being rewritten into a single Go binary.

## Architecture

```
iOS Clipper ──→ inbox/ ──→ [fetch-stubs → claude triage → youtube → index → todoist]
                                              ↓
                              notes/fleeting/ (daily workbench)
                                              ↓
                     [summarize → claude daily synthesis → signal → todoist sync]
                                              ↓
                     notes/{ideas,references,todos,entities,assertions}/
                                              ↓
                     [claude weekly synthesis → audit → enrich → profile]
                                              ↓
                              bloom/ (provocations, assertion tracker)
```

- **Claude Code CLI** — AI backbone. Inbox triage, daily/weekly synthesis, enrichment prompts. Invoked headless via `claude -p`.
- **Python scripts** (`_scripts/`) — Deterministic glue. HTTP clients (Cloudflare, Todoist, YouTube), signal scoring, link auditing, NER extraction.
- **RAG** (`_rag/`) — Semantic search. LanceDB + sentence-transformers. MCP server exposes `search_vault` tool.
- **LaunchAgents** (`~/Library/LaunchAgents/com.mindpalace.*`) — macOS scheduling. 30-min inbox cycle, daily 6AM, weekly Sunday 9AM.
- **Obsidian** — User-facing front-end. Reading, writing, linking notes.

## Pipelines

Three marker-gated pipelines run on cadence:

### Inbox (every 30 min)
```
fetch-stubs → claude "process my inbox" → youtube-transcripts → rag-index → sync-todoist → daily-catchup → weekly-catchup
```

### Daily (once per day, marker: _meta/.daily-last-run)
```
fetch-stubs --retry → summarize-transcripts → rag-index → claude "daily synthesis" → generate-signal → sync-todoist → sync-todoist-completions
```

### Weekly (Sunday/Monday, marker: _meta/.weekly-last-run)
```
fetch-stubs --retry → claude "weekly synthesis" → rag-index → generate-signal → audit-links → enrich-topics → profile-entities
```

## Scripts

All in `_scripts/`. ~2,261 lines total.

| Script | Lines | Purpose | Claude? |
|--------|-------|---------|---------|
| `fetch-stub-content.py` | 370 | Hydrate web clip stubs via Cloudflare, dedup | No |
| `fetch-youtube-transcript.py` | 75 | Fetch YouTube transcripts | No |
| `summarize-transcripts.py` | 231 | Enrich transcripts + web clips with Research Context | Yes |
| `sync-todoist.py` | 77 | Push new todos to Todoist | No |
| `sync-todoist-completions.py` | 75 | Pull completed todos from Todoist | No |
| `generate-signal.py` | 266 | Build signal/queue.md read queue dashboard (by content-type lanes) | No |
| `sync-consumed.py` | 50 | Sync checked items in queue.md back to note frontmatter | No |
| `enrich-topics.py` | 428 | Research briefs for topics ≥5 notes | Yes |
| `profile-entities.py` | 325 | Entity profiles for people/companies ≥5 mentions | Yes |
| `audit-links.py` | 69 | Broken [[wikilink]] scanner | No |
| `run-daily-catchup.sh` | 33 | Marker-gated daily chain orchestration | — |
| `run-weekly-catchup.sh` | 48 | Marker-gated weekly chain orchestration | — |

## RAG

| File | Lines | Purpose |
|------|-------|---------|
| `_rag/indexer.py` | 149 | Embed vault notes → LanceDB (BAAI/bge-base-en-v1.5) |
| `_rag/mcp_server.py` | 115 | MCP server exposing `search_vault` tool |

## Slash Commands

User-invocable via `.claude/commands/`:

| Command | Purpose |
|---------|---------|
| `/reflect [topic]` | Interactive vault exploration + original thinking |
| `/research [topic\|--note path]` | Trigger research enrichment pipeline |
| `/signal [time]` | Curated reading recommendations by time budget |

## External Dependencies

| System | Location | Purpose |
|--------|----------|---------|
| LaunchAgents | `~/Library/LaunchAgents/com.mindpalace.*.plist` | macOS scheduling |
| Env plist | `~/Library/LaunchAgents/com.mindpalace.env.plist` | API key injection at boot |
| Claude CLI | System PATH | AI reasoning (`claude -p`) |
| Todoist API | Remote | Task sync (v2 REST + v9 Sync) |
| Cloudflare /crawl | Remote | Web content fetching (optional) |

## Environment Variables

| Variable | Required | Used By |
|----------|----------|---------|
| `TODOIST_API_TOKEN` | For todo sync | sync-todoist, sync-todoist-completions |
| `TODOIST_WORK_ID` | For todo routing | sync-todoist |
| `TODOIST_PERSONAL_ID` | For todo routing | sync-todoist |
| `TODOIST_INBOX_ID` | For todo routing | sync-todoist |
| `CF_ACCOUNT_ID` | For web crawling | fetch-stub-content, enrich-topics |
| `CF_API_TOKEN` | For web crawling | fetch-stub-content, enrich-topics |
| `HF_TOKEN` | For embeddings | sentence-transformers model download |

## Active Plans

- **Product vision:** `.agents/product-vision.md` — three-layer product strategy (CLI → plugin → interactive app)
- **Go rewrite:** `.agents/go-rewrite-plan.md` — Layer 1 implementation plan
- **System architecture:** `.agents/system-architecture.md` — full reference doc

## Pipeline Resilience

Each pipeline step runs independently. Failures are logged, not fatal.

- **Inbox chain:** Core steps use `&&`. Todoist sync is non-fatal (`|| true`).
  Daily/weekly catchup always attempt (`;`) regardless of prior failures.
- **Catchup scripts:** Each step wrapped in `run_step()`. Failures logged to
  `_meta/pipeline-health.md`. Marker written even on partial failure to prevent
  retry storms (48 attempts/day).
- **Health surface:** `ALERT.md` (pin in Obsidian) embeds `_meta/pipeline-health.md`.
  Check it for at-a-glance pipeline status.
- **Env vars:** Hardcoded in each plist's EnvironmentVariables dict. No dependency
  on `launchctl setenv` (unreliable on Sequoia).

## What not to do

- Don't delete user content — move to `_meta/discarded/` with reason
- Don't set `consumed: true` — only the user marks content as read
- Don't auto-add tags to taxonomy — proposals go to `_meta/taxonomy-proposals.md`
- Don't create notes in deprecated `notes/recipes/`
- Don't use `StartInterval` in LaunchAgents — it's broken on Sequoia
- Don't use `set -e` in pipeline scripts — one broken step shouldn't kill everything
- Don't use `&&` for non-critical pipeline steps — use `|| true` or `;`
