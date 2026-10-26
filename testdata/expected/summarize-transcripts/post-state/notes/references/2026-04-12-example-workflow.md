---
content-type: listen-watch
source: https://www.youtube.com/watch?v=synthetic-example
tags:
- reference
- tech/go-workflows
title: Example Workflow
---

# Example Workflow

## Research Context

### Key Takeaways
- **Keep the vault as the source of truth.** The Markdown vault stays an ordinary directory. Search indexes, temporary views and summaries are built beside it and never replace it.
- **Everything derived should be rebuildable.** If you can delete an index or summary and regenerate it from the source, a failed run costs almost nothing.
- **Separate configuration from execution.** Keep configuration near the command that uses it, but apart from the logic that runs it.
- **Commands that change files should list what they write.** Stating which files a command touches makes the first run easy to follow and a failed run easy to recover.
- **Offer dry runs for destructive work.** Let users preview changes before anything is overwritten or deleted.
- **Prefer boring, standard formats.** Plain Markdown and common config formats outlast any one tool and stay readable without it.
- **Record enough context to explain a result later.** Logging inputs and decisions turns "why did this happen?" into a quick lookup.
- **Ask where ownership should live.** For any piece of data, decide up front whether it is source material or derived from it.

### Follow-up Questions
- When a derived artifact, such as an AI-generated summary, gets edited by hand, does it become source material? How should a system handle that change?
- How do you know when a derived index has drifted from the vault? Should rebuilds run on every change, on a schedule, or only on demand?
- Is a dry run still trustworthy when the command depends on outside state, like network calls or LLM output that differs between runs?
- How much context is "enough" to explain a result later before logging becomes a maintenance burden of its own?
- Should derived files live inside the vault, next to it, or somewhere else entirely, given sync, backups and editor plugins?

### Related Materials
The transcript does not name any specific sources. These are relevant to its themes:
- **"Local-first software: You own your data, in spite of the cloud"** by Ink & Switch (Kleppmann, Wiggins, van Hardenberg, McGranaghan): an essay on keeping user-owned data as the source of truth. https://www.inkandswitch.com/local-first/
- **"Turning the database inside out"** by Martin Kleppmann (Strange Loop 2014 talk): treats indexes and caches as derived views of an immutable source log. https://martin.kleppmann.com/2015/03/04/turning-the-database-inside-out.html
- **The Twelve-Factor App, "III. Config"**: the classic guidance on keeping configuration separate from code. https://12factor.net/config
- **CommonMark Spec**: the standard Markdown definition, which fits the "boring standard formats" advice. https://commonmark.org
- **Obsidian**: a widely used editor built on a plain folder of Markdown files, the vault model the transcript describes. https://obsidian.md

### Book & Podcast Recommendations
**Books**
- **_Designing Data-Intensive Applications_** by Martin Kleppmann: its chapters on derived data, the difference between a source of truth and derived data, and rebuildable indexes match the transcript's main point.
- **_The Pragmatic Programmer_** by Andrew Hunt and David Thomas: covers the "power of plain text," automation and defensive habits that line up with the operational advice here.
- **_Building a Second Brain_** by Tiago Forte: gives the personal-knowledge-management reasons for keeping a vault useful over the long term.

**Podcasts**
- **Software Engineering Radio** (IEEE Software): regular in-depth episodes on configuration management, reliability and system design trade-offs.
- **The Changelog** (Adam Stacoviak and Jerod Santo): often features developers of tools and CLIs who discuss local-first and plain-text workflows.
- **CoRecursive** (Adam Gordon Bell): long-form stories about how engineers made design decisions, including when they failed and how they recovered.

## Transcript

This synthetic transcript explains how a small command can keep a Markdown vault useful. It separates configuration from execution and keeps the external vault as the source of truth. A simple workflow favors clear boundaries, repeatable checks, and explicit recovery when a command fails. The example is intentionally fictional and exists only to exercise transcript enrichment.

The speaker compares a source document with derived indexes and asks where ownership should live. A source vault can remain an ordinary directory while a runtime builds temporary views, search indexes, and summaries beside it. Each derived artifact should be rebuildable, and each mutating command should say which files it writes. This makes a first run understandable and makes a failed run recoverable.

The transcript also describes small operational habits: keep configuration near the command, prefer boring standard formats, make dry runs available for destructive work, and record enough context to explain a result later. These are general engineering examples rather than claims about a real product or person. The test fixture keeps the language generic so its output can be published safely.