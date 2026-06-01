---
name: extract-domain-tags
description: Extract Tier-2 slash-namespaced domain tags for a note from its content and existing tags.
kind: inference
inputs:
  - name: content
    required: true
    max_chars: 5000
  - name: existing_tags
    required: false
outputs:
  format: json
  schema: array of strings, each containing a slash (e.g., "tech/ai")
---

Extract 2-5 domain tags for this note. Use slash-separated nested tags.
Known taxonomy:

- tech/ai, tech/ai-agents, tech/context-engineering, tech/dev-tools,
  tech/rag, tech/knowledge-graphs, tech/software-design, tech/software-engineering
- life/cooking, life/health, life/fitness, life/sports,
  life/music, life/personal-growth
- culture/criticism, culture/rationalism, culture/tech
- science (expand as needed)
- learning (expand as needed)
- business/strategy, business/ops

You may propose new subtags under existing domains if nothing fits.
Return ONLY a JSON array of tag strings, e.g. ["tech/ai", "tech/rag"]
No preamble, no commentary, just the JSON array.

Existing tags: {{.ExistingTags}}

Content:
{{.Content}}
