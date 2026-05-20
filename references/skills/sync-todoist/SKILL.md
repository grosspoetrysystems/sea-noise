# Sync Todoist

## Purpose
Bidirectional sync between vault todo notes and Todoist. Push creates tasks. Pull marks completed tasks as done in vault.

## Scripts
- `_scripts/sync-todoist.py` (77 lines) — push
- `_scripts/sync-todoist-completions.py` (75 lines) — pull

## Push Logic
1. Scan `notes/todos/` for notes without `synced-todoist` in frontmatter
2. Route by tag:
   - `#work` → `TODOIST_WORK_ID` project
   - `#personal` → `TODOIST_PERSONAL_ID` project
   - Neither → `TODOIST_INBOX_ID` project
3. Create task via Todoist REST v2 API (POST /tasks):
   - Content: title from frontmatter
   - Description: Obsidian deep link (`obsidian://open?vault=mind-palace&file=...`)
   - Label: `mind-palace`
   - Due: `due_string` from frontmatter + `due_lang: en` for NLP parsing
4. Due string normalization (regex safety net):
   - "5p" → "5pm", "5a" → "5am"
   - Strip "before"/"by" qualifiers
5. Update frontmatter: `synced-todoist: <task_id>`, `synced-todoist-url: <url>`

## Pull Logic
1. Fetch completed tasks from Todoist Sync v9 API
2. For each completed task, search `notes/todos/` for matching `synced-todoist` value
3. Update frontmatter: `done: true`, `completed-date: YYYY-MM-DD`

## Environment
- `TODOIST_API_TOKEN` — API token (required)
- `TODOIST_WORK_ID` — Work project ID
- `TODOIST_PERSONAL_ID` — Personal project ID
- `TODOIST_INBOX_ID` — Default inbox project ID

## Go Rewrite Target
`mind-palace sync todoist` / `mind-palace sync todoist --completions` — Phase 3 in go-rewrite-plan.md.
Maps to `internal/todoist/` (client + completions).
