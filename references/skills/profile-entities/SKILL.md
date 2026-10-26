# Profile Entities

## Purpose
Auto-generate profiles for people, companies, and concepts appearing 5+ times across the vault. Conservative: Claude pre-filters false positives, max 5 profiles per run.

## Script
`_scripts/profile-entities.py` (325 lines)

## Modes
- **Auto:** `profile-entities.py` — threshold scan (default 5 mentions)
- **Manual:** `profile-entities.py "Sam Kriss"` — profile specific entity
- **Custom threshold:** `profile-entities.py --threshold 10`

## Process
1. Extract entity candidates: regex NER (capitalized multi-word phrases)
2. Strip structural text (headings, frontmatter, URLs) from extraction
3. Claude pre-filter: single call to classify real entities vs false positives
4. For each valid entity (max 5 per run):
   - Classify type: person / company / concept
   - Collect all mentioning notes
   - Generate profile via Claude
5. Write profile note

## Output Structure
```
notes/entities/YYYY-MM-DD-profile-<slug>.md
---
title: "Entity Name"
tags: [entity, entity/person, entity-profile]
date: YYYY-MM-DD
---
## Overview
## Key Ideas & Contributions
## Related Entities
## External Links
## Vault Presence (backlinks)
```

## Log
`_meta/entity-profiles-log.md`

## Go command
`sea-noise profile-entities` generates profiles for qualifying entities.
