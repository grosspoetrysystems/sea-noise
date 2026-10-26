# Audit Links

## Purpose
Scan vault for broken [[wikilinks]] — links that point to notes that don't exist.

## Script
`_scripts/audit-links.py` (69 lines)

## Process
1. Collect all note stems (lowercased filenames without extension) as valid targets
2. Regex scan all .md files for `[[link]]` patterns
3. Compare each link target against valid stems (case-insensitive)
4. Report missing targets grouped by source file

## Output
`_meta/broken-links.md`

## Go command
`sea-noise audit-links` writes the broken-link report.
