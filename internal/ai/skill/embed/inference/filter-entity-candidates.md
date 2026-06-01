---
name: filter-entity-candidates
description: Classify a list of regex-extracted entity candidates as keep/drop and assign a type, so vault-wide profile-entities can prune false positives before generating full profiles.
kind: inference
inputs:
  - name: candidates_text
    required: true
outputs:
  format: json
---

You are filtering entity candidates extracted via regex NER from a knowledge vault.
For each name below, respond with ONLY a JSON array of objects: [{"name": "...", "keep": true/false, "type": "person|company|project|concept|other"}]

Keep = true ONLY for: real people, real companies/orgs, notable projects/products, or specific frameworks/methodologies worth a dedicated profile.
Keep = false for: places, generic concepts, markdown headings, section titles, generic tech terms, or anything too vague for a useful standalone profile.

Candidates:
{{.CandidatesText}}
