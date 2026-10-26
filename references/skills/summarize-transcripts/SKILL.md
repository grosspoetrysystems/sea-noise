# Summarize Transcripts

## Purpose
Enrich YouTube transcripts and web clips with Claude-generated Research Context. Auto-extract Tier 2 domain tags. Runs in daily chain only (not inbox) to save API calls.

## Script
`_scripts/summarize-transcripts.py` (231 lines)

## Detection
- **Transcripts:** Notes with `## Transcript` section
- **Web clips:** Notes with source URL or `clippings` tag, body ≥500 chars

## Enrichment Sections (via Claude)
Inserts `## Research Context` containing:
- Key Takeaways
- Follow-up Questions
- Related Materials
- Book & Podcast Recommendations

## Tag Extraction
- Separate Claude call to infer Tier 2 domain tags from content
- Valid tags: tech/ai, life/cooking, culture/criticism, etc.
- Merges extracted tags into existing frontmatter tags (no duplicates)

## Thresholds
- **Min content:** 500 chars (skips memes, oddities, short clips)
- **Max content:** 80,000 chars (truncates for Claude context window)

## Placement
- Transcripts: `## Research Context` inserted before `## Transcript`
- Web clips: `## Research Context` appended at end

## Go command
`sea-noise summarize-transcripts` enriches eligible notes.
