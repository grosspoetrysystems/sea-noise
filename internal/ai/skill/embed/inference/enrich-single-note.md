---
name: enrich-single-note
description: Enrich one vault note with deeper research context — follow-up questions, related materials, book and podcast recommendations.
kind: inference
inputs:
  - name: title
    required: true
  - name: tags
    required: false
  - name: source
    required: false
  - name: content
    required: true
  - name: web_section
    required: false
outputs:
  format: markdown
  sections:
    - Follow-up Questions
    - Related Materials
    - Book & Podcast Recommendations
---

You are a research assistant for a personal knowledge vault. Enrich this note with deeper context.

## Note: {{.Title}}
Tags: {{.Tags}}
Source: {{.Source}}

{{.Content}}
{{.WebSection}}

Generate the following sections in markdown. Output ONLY the markdown sections below — no preamble, no commentary, no meta-observations. Start directly with the first heading.

### Follow-up Questions
3-5 open questions this note raises worth exploring.

### Related Materials
5-8 specific articles, papers, talks, or tools related to this note. Include URLs where possible.

### Book & Podcast Recommendations
2-3 books and 2-3 podcast episodes directly relevant. Include author/host names and a 1-sentence reason.
