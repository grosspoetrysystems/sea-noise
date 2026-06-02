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

7. **Write your report.** Append a per-note classification report to `{{.VaultRoot}}/_meta/inbox-reports/YYYY-MM-DD.md` (today's date), creating the file + directory if missing. Use one bullet per note in the shape:

   ```
   - `<filename>` — tags: <comma-list> — content-type: <type> — <one-line summary> — promoted to <final-path>
   ```

   Do **NOT** write any report to `_meta/processing-logs/` — that file belongs exclusively to the daemon's step-completion log. Mixing report content with daemon log entries pollutes the format the `internal/signal/` and audit tooling parses.

Then return a brief summary string as your response: "Processed N notes; promoted X to references/, Y to ideas/, Z to discarded/; W new tags introduced." The daemon captures this string for its own step-completion log line. Do not include the per-note detail in your response — that's already in the report file.
