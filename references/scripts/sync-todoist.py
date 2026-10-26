import os
import re
import requests
import frontmatter
from pathlib import Path

API_TOKEN = os.environ.get("TODOIST_API_TOKEN")
if not API_TOKEN:
    raise EnvironmentError("TODOIST_API_TOKEN not set")

TODOIST_INBOX_PROJECT_ID = os.environ.get("TODOIST_INBOX_ID", "")
TODOIST_WORK_PROJECT_ID = os.environ.get("TODOIST_WORK_ID", "")
TODOIST_PERSONAL_PROJECT_ID = os.environ.get("TODOIST_PERSONAL_ID", "")

VAULT = Path(__file__).parent.parent
TODOS_DIR = VAULT / "notes/todos"
VAULT_NAME = "mind-palace"

synced = 0
skipped = 0
failed = 0

for note_path in TODOS_DIR.glob("*.md"):
    post = frontmatter.load(note_path)

    if post.get("synced-todoist"):
        skipped += 1
        continue

    title = post.get("title")
    if not title:
        # Fallback: filename stem, strip YYYY-MM-DD- prefix
        title = re.sub(r"^\d{4}-\d{2}-\d{2}-?", "", note_path.stem).replace("-", " ").strip().title()
    obsidian_link = f"obsidian://open?vault={VAULT_NAME}&file=notes/todos/{note_path.stem}"
    content = f"{title} [→ vault]({obsidian_link})"

    raw_tags = post.get("tags", [])
    tags = raw_tags if isinstance(raw_tags, list) else [raw_tags]

    if "#work" in tags or "work" in tags:
        project_id = TODOIST_WORK_PROJECT_ID
    elif "#personal" in tags or "personal" in tags:
        project_id = TODOIST_PERSONAL_PROJECT_ID
    else:
        project_id = TODOIST_INBOX_PROJECT_ID

    payload = {"content": content, "labels": ["mind-palace"]}
    if project_id:
        payload["project_id"] = project_id
    due_string = str(post.get("due", "")).strip()
    if due_string:
        # Normalize bare "Xp" / "Xam" → "Xpm" / "Xam" so Todoist accepts it
        due_string = re.sub(r'(\d)(p)\b', r'\1pm', due_string, flags=re.IGNORECASE)
        due_string = re.sub(r'(\d)(a)\b', r'\1am', due_string, flags=re.IGNORECASE)
        # Drop vague "before"/"by" qualifiers Todoist can't parse
        due_string = re.sub(r'\b(before|by)\s+', '', due_string, flags=re.IGNORECASE).strip()
        payload["due_string"] = due_string
        payload["due_lang"] = "en"

    response = requests.post(
        "https://api.todoist.com/api/v1/tasks",
        headers={"Authorization": f"Bearer {API_TOKEN}"},
        json=payload,
    )

    if response.ok:
        task_id = response.json()["id"]
        post["synced-todoist"] = task_id
        post["synced-todoist-url"] = f"https://todoist.com/app/task/{task_id}"
        note_path.write_text(frontmatter.dumps(post))
        synced += 1
        print(f"  ✓ {note_path.stem}")
    else:
        failed += 1
        print(f"  ✗ {note_path.stem} — {response.status_code}: {response.text}")

print(f"\nDone: {synced} synced, {skipped} skipped, {failed} failed")
