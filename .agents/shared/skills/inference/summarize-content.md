---
name: summarize-content
description: Generate research context (key takeaways, follow-up questions, related materials, recommendations) for a piece of content.
kind: inference
inputs:
  - name: content
    required: true
    max_chars: 80000
  - name: title
    required: false
  - name: content_type
    required: false
    enum: [transcript, article]
    default: article
outputs:
  format: markdown
  sections:
    - Key Takeaways
    - Follow-up Questions
    - Related Materials
    - Book & Podcast Recommendations
---

You are summarizing {{.SourceDescription}}{{.TitleContext}} for a personal knowledge base.

Generate the following sections in markdown. Output ONLY the markdown, no preamble.

### Key Takeaways
5-8 concise bullet points. Focus on actionable insights, novel ideas, and memorable quotes. No filler or generic observations.

### Follow-up Questions
3-5 genuinely thought-provoking questions this content raises.

### Related Materials
3-5 specific articles, papers, talks, tools, or people mentioned or directly relevant. Include URLs where possible. Do NOT make up titles.

### Book & Podcast Recommendations
2-3 books and 2-3 podcasts/episodes relevant to the topics discussed. Include author/host. 1 sentence on why each is relevant. Only recommend real, verifiable titles.

Content:
{{.Content}}
