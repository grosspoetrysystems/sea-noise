# Enrich Topics

## Purpose
Generate research briefs for topics that reach critical mass (5+ notes by default). Optionally crawls the web for external context via Cloudflare.

## Script
`_scripts/enrich-topics.py` (428 lines)

## Modes
- **Auto:** `enrich-topics.py` — scan `_meta/topic-frequencies.md` for topics with count ≥5
- **Manual topic:** `enrich-topics.py "ai-agents"` — enrich specific topic
- **Single note:** `enrich-topics.py --note path/to/note.md` — enrich one note

## Process
1. Identify topic (auto: threshold scan, or manual)
2. Collect all vault notes with topic in tags or filename
3. Optionally crawl web via Cloudflare /crawl for external context
4. Call Claude to generate research brief from vault + web context
5. Write output

## Output
- **Topic mode:** `notes/references/briefs/YYYY-MM-DD-<topic>-research-brief.md`
  - Tags: #reference, #reference/brief, relevant domain tags
- **Single note mode:** Appends sections directly to note:
  - ## Follow-up Questions
  - ## Related Materials
  - ## Book & Podcast Recommendations
- **Log:** `_meta/enrichment-log.md`

## Skips
- Notes already enriched by summarize-transcripts (has ## Research Context)

## Environment
- `CF_ACCOUNT_ID`, `CF_API_TOKEN` — optional, falls back to vault-only context + Claude

## Go Rewrite Target
`mind-palace enrich [topic]` / `mind-palace enrich --note <path>` — Phase 4 in go-rewrite-plan.md.
Maps to `internal/enrich/`.
