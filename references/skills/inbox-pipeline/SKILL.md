# Inbox Pipeline

## Purpose

Process Markdown notes from `inbox/` into the configured vault. The pipeline hydrates source stubs, triages notes, enriches transcripts, and records the result. Run it explicitly with the Go CLI or through `sea-noise serve`.

## Chain

```text
fetch → process-inbox → youtube-transcript
```

The daemon schedules the chain. `sea-noise run inbox` executes one pass for inspection. A failed step stops the current pass and returns an error; retryable fetch failures are recorded in `_meta/crawl-failures.json`.

## Triage logic

1. **Web clip** — a note with an `https://` source. Preserve source content, add context, and keep it unconsumed until downstream processing succeeds.
2. **Voice capture** — raw dictation. Preserve the raw transcript and extract explicit tasks into the configured todo path.
3. **Ad hoc note** — clean Markdown. Preserve the body and add only missing metadata.

All sources receive normalized frontmatter and are moved out of `inbox/` into the configured note path. Processing logs are written under `_meta/processing-logs/`.

## Input

- `inbox/*.md` — raw captures or source stubs.

## Output

- configured note and todo directories;
- `_meta/processing-logs/YYYY-MM-DD.md`; and
- `_meta/crawl-failures.json` when a source fetch needs retry.

## Go command

```sh
sea-noise process-inbox
```

Use `sea-noise run inbox --force` for a one-shot diagnostic pass that ignores the period marker. The vault path comes from `sea-noise.toml` or the documented compatibility environment variable.
