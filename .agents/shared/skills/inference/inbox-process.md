---
name: inbox-process
description: Process every file currently in the vault's inbox/ — triage, tag, enrich frontmatter, write topic-frequencies + rejected-connections + atlas updates, then promote to its final directory.
kind: inference
inputs:
  - name: vault_root
    required: true
outputs:
  format: report
---

You are processing the inbox/ folder of a personal knowledge vault rooted at:

{{.VaultRoot}}

Read the vault's CLAUDE.md (or AGENTS.md) for the full tag taxonomy, content-type rules, and atlas/ conventions. This skill is the operational invocation of the "process my inbox" command described there. Follow the conventions verbatim.

For each file in {{.VaultRoot}}/inbox/:

1. **Triage by source type.** Web Clipper, Voice Capture, Ad Hoc Note, or Voice Todo — detect from frontmatter, source URL, and body shape. Apply the per-type rules from CLAUDE.md.

2. **Title + tags + content-type.** Add a descriptive title if missing; assign exactly one Tier 1 tag; add 1–3 Tier 2 domain tags; set content-type (deep-read, tool-reference, listen-watch, anchor, stub). Respect any `mood: true` and `tags: [assertion]` the user pre-set.

3. **Link.** Scan the vault for related notes and add `[[wikilinks]]` where genuinely relevant. When a link is considered and rejected, append the (link, reason) pair to `{{.VaultRoot}}/_meta/rejected-connections.md`.

4. **Update topic-frequencies.** For each tag and topic seen, update `{{.VaultRoot}}/_meta/topic-frequencies.md`. The file is a single markdown table with these columns and header row (do not add or remove columns; do not change column order):

   ```
   | tag | count | last_seen | trend |
   |-----|-------|-----------|-------|
   ```

   - `tag` — the tag literal (e.g. `tech/ai`, no leading `#`)
   - `count` — total mentions across the vault, integer
   - `last_seen` — `YYYY-MM-DD` of the most recent note carrying the tag
   - `trend` — `rising` when count increased by more than 2 in the last 7 days, otherwise `stable` or `falling`

   This format is the contract `internal/signal/` parses. Do not change it.

5. **Atlas MOC updates.** For each note's primary topic cluster, find or create the relevant `{{.VaultRoot}}/atlas/<topic>.md` Map of Content and add a `[[wikilink]]` to the note under the appropriate section. Create a new atlas/ MOC when a tag cluster has 5+ notes without one. Split any atlas/ MOC section with 15+ links into a sub-MOC.

6. **Promote.** Move every note with a clear Tier 1 tag out of `inbox/` to its final directory (`notes/ideas/`, `notes/references/`, `notes/entities/`, `notes/assertions/`, `notes/todos/`, etc.). `#question` and `#fleeting` stay in `notes/fleeting/`. Low-signal captures move to `_meta/discarded/`.

Report when finished: how many notes processed, how many promoted by tier, how many discarded, and the new tags introduced into topic-frequencies. Do not write a separate daily/YYYY-MM-DD.md — that's a different command.
