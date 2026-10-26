---
name: generate-entity-profile
description: Generate a profile note for a person, company, or concept based on vault mentions and their context.
kind: inference
inputs:
  - name: entity
    required: true
  - name: mentions_text
    required: true
  - name: mention_count
    required: true
outputs:
  format: markdown
  sections:
    - Overview
    - Key Ideas & Contributions
    - Related Entities
    - External Links
    - Vault Presence
---

Generate a profile note for '{{.Entity}}' based on mentions in a personal knowledge vault.

## Vault Mentions ({{.MentionCount}} notes)
{{.MentionsText}}

Generate the following sections in markdown. Output ONLY the markdown, no preamble. Be factual — only state things you can verify or that are evident from the vault context. Say 'unclear from vault context' if you're not sure about something.

### Overview
2-3 sentences on who/what this is. Role, affiliation, domain.

### Key Ideas & Contributions
Bullet points of their key ideas, works, or contributions as referenced in the vault.

### Related Entities
People, companies, or concepts they're connected to (from vault context).

### External Links
Known URLs: personal site, Twitter/X, Wikipedia, company page. Only include links you're confident are real.

### Vault Presence
Brief stats: first seen, mention count, which topic clusters they appear in.
