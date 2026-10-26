---
name: generate-research-brief
description: Generate a research brief for a topic cluster reaching critical mass in the vault — follow-up questions, related materials, book and podcast recommendations, and suggested deep-dives.
kind: inference
inputs:
  - name: topic
    required: true
  - name: notes_summary
    required: true
  - name: note_count
    required: true
  - name: web_section
    required: false
outputs:
  format: markdown
  sections:
    - Follow-up Questions
    - Related Materials
    - Book & Podcast Recommendations
    - Suggested Deep Dives
---

You are a research assistant for a personal knowledge vault. The topic '{{.Topic}}' has reached critical mass with {{.NoteCount}} notes.

## Existing Vault Notes
{{.NotesSummary}}
{{.WebSection}}

Generate the following sections in markdown. Output ONLY the markdown, no preamble. Be specific and actionable — reference real books, real podcasts, real people, real papers. Do NOT make up titles or authors. If unsure, say so.

### Follow-up Questions
5-7 open questions worth exploring based on the notes and gaps you see. Genuinely thought-provoking, not generic.

### Related Materials
5-10 specific articles, papers, talks, or tools relevant to this cluster. Include URLs where possible. Prioritize recent (2024-2026) and seminal works.

### Book & Podcast Recommendations
3-5 books and 3-5 podcast episodes (or series) directly relevant. Include author/host names. Brief (1 sentence) on why each is relevant.

### Suggested Deep Dives
2-3 specific research directions that would strengthen this topic cluster. For each, suggest a starting URL or search query.
