#!/usr/bin/env python3
"""
Poll Todoist for completed tasks labelled 'mind-palace' and mark
them as done in vault frontmatter (notes/todos/).

Uses Todoist API v1: GET /api/v1/tasks/completed/by_completion_date
(replaces deprecated Sync API v9 completed/get_all).
"""
import os
import requests
import frontmatter
from datetime import datetime, timedelta, timezone
from pathlib import Path

API_TOKEN = os.environ.get("TODOIST_API_TOKEN")
if not API_TOKEN:
    raise EnvironmentError("TODOIST_API_TOKEN not set")

VAULT = Path(__file__).parent.parent
TODOS_DIR = VAULT / "notes/todos"
HEADERS = {"Authorization": f"Bearer {API_TOKEN}"}

completed = 0
skipped = 0


def get_completed_tasks(days_back: int = 14) -> list[dict]:
    """Fetch recently completed tasks from Todoist API v1.
    Uses cursor-based pagination to get all results."""
    since = (datetime.now(timezone.utc) - timedelta(days=days_back)).strftime("%Y-%m-%dT00:00:00Z")
    until = datetime.now(timezone.utc).strftime("%Y-%m-%dT23:59:59Z")

    all_items = []
    cursor = None

    while True:
        params = {"since": since, "until": until, "limit": 200}
        if cursor:
            params["cursor"] = cursor

        resp = requests.get(
            "https://api.todoist.com/api/v1/tasks/completed/by_completion_date",
            headers=HEADERS,
            params=params,
        )
        resp.raise_for_status()
        data = resp.json()
        items = data.get("items", [])
        all_items.extend(items)

        cursor = data.get("next_cursor")
        if not cursor or not items:
            break

    return all_items


def find_note_by_task_id(task_id: str) -> Path | None:
    """Find the vault note that has synced-todoist matching this task ID."""
    for note_path in TODOS_DIR.glob("*.md"):
        post = frontmatter.load(note_path)
        if str(post.get("synced-todoist", "")) == str(task_id):
            return note_path
    return None


def main():
    global completed, skipped

    done_tasks = get_completed_tasks()
    # Filter to mind-palace labelled tasks only
    mp_tasks = [t for t in done_tasks if "mind-palace" in t.get("labels", [])]
    print(f"Found {len(mp_tasks)} completed mind-palace tasks (of {len(done_tasks)} total)")

    for task in mp_tasks:
        task_id = str(task.get("id", ""))
        if not task_id:
            continue

        note_path = find_note_by_task_id(task_id)
        if not note_path:
            skipped += 1
            continue

        post = frontmatter.load(note_path)
        if post.get("done"):
            skipped += 1
            continue

        post["done"] = True
        completed_at = task.get("completed_at", "")
        post["completed-date"] = completed_at[:10] if completed_at else ""
        note_path.write_text(frontmatter.dumps(post))
        completed += 1
        print(f"  ✓ {note_path.stem}")

    print(f"\nDone: {completed} marked complete, {skipped} skipped")


if __name__ == "__main__":
    main()
