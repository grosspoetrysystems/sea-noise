#!/usr/bin/env python3
"""
Hydrate stub web clips (from iOS Share Sheet) with full page content.

Detects inbox files that have a source URL and clippings tag but minimal
body content, then uses Cloudflare /crawl to fetch the page and append
the markdown under ## Raw.

Modes:
  (default)   Process inbox stubs — hydrate and dedupe
  --retry     Retry failed crawls from _meta/crawl-failures.json

Runs before inbox processing so Claude has full content to work with.
"""
import json
import os
import sys
import time
import re
from datetime import datetime, timedelta
from pathlib import Path

import frontmatter
import requests

VAULT = Path(__file__).parent.parent
INBOX = VAULT / "inbox"
CRAWL_MANIFEST = VAULT / "_meta" / "crawl-failures.json"
DISCARD_DIR = VAULT / "_meta" / "discarded"
MIN_BODY_CHARS = 200  # below this = stub

CF_ACCOUNT_ID = os.environ.get("CF_ACCOUNT_ID", "")
CF_API_TOKEN = os.environ.get("CF_API_TOKEN", "")
CF_CRAWL_URL = f"https://api.cloudflare.com/client/v4/accounts/{CF_ACCOUNT_ID}/browser-rendering/crawl"

CRAWL_POLL_INTERVAL = 5
CRAWL_POLL_MAX = 30

# Backoff schedule in hours: 6h → 24h → 72h → 168h
BACKOFF_HOURS = [6, 24, 72, 168]
MAX_RETRIES = len(BACKOFF_HOURS)


# --- Crawl Failure Manifest ---

def load_manifest() -> list[dict]:
    if CRAWL_MANIFEST.exists():
        try:
            return json.loads(CRAWL_MANIFEST.read_text(encoding="utf-8"))
        except (json.JSONDecodeError, Exception):
            return []
    return []


def save_manifest(entries: list[dict]):
    CRAWL_MANIFEST.parent.mkdir(parents=True, exist_ok=True)
    CRAWL_MANIFEST.write_text(
        json.dumps(entries, indent=2, default=str), encoding="utf-8"
    )


def log_failure(url: str, error_type: str, note_path: str):
    """Log a crawl failure to the manifest with backoff scheduling."""
    entries = load_manifest()

    # Check if URL already tracked
    for entry in entries:
        if entry["url"] == url:
            entry["retry_count"] = entry.get("retry_count", 0) + 1
            entry["last_error"] = error_type
            entry["last_attempt"] = datetime.now().isoformat()
            if entry["retry_count"] < MAX_RETRIES:
                hours = BACKOFF_HOURS[entry["retry_count"]]
                entry["next_retry_after"] = (
                    datetime.now() + timedelta(hours=hours)
                ).isoformat()
            else:
                entry["abandoned"] = True
            save_manifest(entries)
            return

    # New failure
    entry = {
        "url": url,
        "note_path": note_path,
        "error_type": error_type,
        "retry_count": 0,
        "first_failure": datetime.now().isoformat(),
        "last_attempt": datetime.now().isoformat(),
        "next_retry_after": (
            datetime.now() + timedelta(hours=BACKOFF_HOURS[0])
        ).isoformat(),
        "abandoned": False,
    }
    entries.append(entry)
    save_manifest(entries)


def remove_from_manifest(url: str):
    """Remove a URL from the failure manifest (on success)."""
    entries = load_manifest()
    entries = [e for e in entries if e["url"] != url]
    save_manifest(entries)


# --- Deduplication ---

def collect_existing_urls() -> set[str]:
    """Scan fleeting/ and references/ for source URLs already in the vault."""
    urls = set()
    scan_dirs = [
        VAULT / "notes" / "fleeting",
        VAULT / "notes" / "references",
        VAULT / "notes" / "ideas",
    ]
    for d in scan_dirs:
        if not d.exists():
            continue
        for path in d.rglob("*.md"):
            try:
                post = frontmatter.load(path)
            except Exception:
                continue
            source = str(post.get("source", ""))
            if source.startswith("http"):
                urls.add(normalize_url(source))
    return urls


def normalize_url(url: str) -> str:
    """Normalize URL for dedup comparison (strip trailing slash, fragments, tracking params)."""
    url = url.strip()
    # Remove common tracking parameters
    url = re.sub(r'[?&](utm_\w+|ref|fbclid|gclid|mc_[a-z]+)=[^&]*', '', url)
    # Clean up leftover ? or &
    url = re.sub(r'\?&', '?', url)
    url = re.sub(r'\?$', '', url)
    # Remove fragment
    url = url.split('#')[0]
    # Strip trailing slash
    url = url.rstrip('/')
    return url


def discard_duplicate(note_path: Path, reason: str):
    """Move a duplicate note to _meta/discarded/."""
    DISCARD_DIR.mkdir(parents=True, exist_ok=True)
    dest = DISCARD_DIR / note_path.name
    # Avoid overwriting
    if dest.exists():
        stem = note_path.stem
        dest = DISCARD_DIR / f"{stem}-dup{note_path.suffix}"
    note_path.rename(dest)
    print(f"  ✗ Duplicate discarded: {note_path.name} → _meta/discarded/ ({reason})")


# --- Platform APIs (external module) ---
from platform_apis import fetch_via_platform_api, crawl_upgrade_queue


# --- Crawl (Cloudflare fallback) ---

def crawl_url(url: str) -> str | None:
    """Fetch a single page. Tries platform APIs first, then Cloudflare /crawl."""
    api_result = fetch_via_platform_api(url)
    if api_result:
        return api_result

    if not CF_ACCOUNT_ID or not CF_API_TOKEN:
        print("  ⚠ CF credentials not set, skipping", file=sys.stderr)
        return None

    headers = {
        "Authorization": f"Bearer {CF_API_TOKEN}",
        "Content-Type": "application/json",
    }
    payload = {
        "url": url,
        "limit": 1,
        "depth": 0,
        "formats": ["markdown"],
        "render": True,
    }

    try:
        resp = requests.post(CF_CRAWL_URL, headers=headers, json=payload)
    except requests.RequestException as e:
        print(f"  ✗ Crawl request failed: {e}", file=sys.stderr)
        return None

    if not resp.ok:
        print(f"  ✗ Crawl failed: {resp.status_code}", file=sys.stderr)
        return None

    result = resp.json().get("result", "")
    job_id = result.get("id") if isinstance(result, dict) else result
    if not job_id:
        print("  ✗ No job ID", file=sys.stderr)
        return None

    for i in range(CRAWL_POLL_MAX):
        time.sleep(CRAWL_POLL_INTERVAL)
        try:
            status_resp = requests.get(
                f"{CF_CRAWL_URL}/{job_id}",
                headers={"Authorization": f"Bearer {CF_API_TOKEN}"},
            )
        except requests.RequestException:
            continue
        if not status_resp.ok:
            continue
        data = status_resp.json().get("result", {})
        status = data.get("status", "")

        if status in ("completed", "cancelled_due_to_timeout", "cancelled_due_to_limits"):
            records = data.get("records", [])
            for r in records:
                if r.get("status") == "completed" and r.get("markdown"):
                    return r["markdown"]
            return None
        if status == "errored":
            print("  ✗ Crawl errored", file=sys.stderr)
            return None

    print("  ✗ Crawl timed out", file=sys.stderr)
    return None


def is_stub(post: frontmatter.Post) -> bool:
    """Check if a clip is a stub (URL + clippings tag, minimal body)."""
    source = str(post.get("source", ""))
    if not source.startswith("http"):
        return False

    tags = post.get("tags", [])
    if isinstance(tags, str):
        tags = [tags]
    if "clippings" not in tags:
        return False

    body = post.content.strip()
    return len(body) < MIN_BODY_CHARS


# --- Find note by source URL (for retry mode) ---

def find_note_by_source_url(url: str) -> Path | None:
    """Search the vault for a note with a matching source URL."""
    search_dirs = [
        INBOX,
        VAULT / "notes" / "fleeting",
        VAULT / "notes" / "references",
    ]
    normalized = normalize_url(url)
    for d in search_dirs:
        if not d.exists():
            continue
        for path in d.rglob("*.md"):
            try:
                post = frontmatter.load(path)
            except Exception:
                continue
            source = str(post.get("source", ""))
            if source.startswith("http") and normalize_url(source) == normalized:
                return path
    return None


# --- Main modes ---

def process_inbox():
    """Default mode: hydrate stubs in inbox/, dedup before processing."""
    if not INBOX.exists():
        return

    existing_urls = collect_existing_urls()
    hydrated = 0
    deduped = 0

    for note_path in sorted(INBOX.glob("*.md")):
        try:
            post = frontmatter.load(note_path)
        except Exception:
            continue

        # Dedup check: does this URL already exist in the vault?
        source = str(post.get("source", ""))
        if source.startswith("http"):
            normalized = normalize_url(source)
            if normalized in existing_urls:
                discard_duplicate(note_path, f"URL already in vault: {source}")
                deduped += 1
                continue
            # Track this URL to catch inbox-internal dupes too
            existing_urls.add(normalized)

        if not is_stub(post):
            continue

        title = post.get("title", note_path.stem)
        print(f"  Hydrating stub: {title}")
        print(f"  URL: {source}")

        content = crawl_url(source)
        if content:
            # Trim to reasonable size
            content = content[:80000]
            post.content = post.content.rstrip() + f"\n\n{content}\n"
            note_path.write_text(frontmatter.dumps(post))
            hydrated += 1
            print(f"  ✓ {note_path.name} ({len(content)} chars)")
        else:
            rel = str(note_path.relative_to(VAULT))
            log_failure(source, "crawl_failed", rel)
            print(f"  ✗ Could not fetch content for {note_path.name} (logged to manifest)")

    # Crawl upgrade pass: re-fetch URLs that got partial content via platform API
    # (e.g. Substack Notes where we got note text but not the linked article).
    # Uses Cloudflare /crawl (JS rendering) which may reveal the linked content.
    upgraded = 0
    if crawl_upgrade_queue and CF_ACCOUNT_ID and CF_API_TOKEN:
        print(f"\n  Crawl upgrade: {len(crawl_upgrade_queue)} URLs queued for JS rendering")
        for upgrade_url in crawl_upgrade_queue:
            note_path = find_note_by_source_url(upgrade_url)
            if not note_path or not note_path.exists():
                continue
            post = frontmatter.load(note_path)
            existing_len = len(post.content.strip())

            # Skip Cloudflare crawl if we already have substantial content
            if existing_len > 1000:
                continue

            # Direct Cloudflare crawl (bypass platform API)
            print(f"  ↻ Crawl upgrade: {upgrade_url}")
            headers = {
                "Authorization": f"Bearer {CF_API_TOKEN}",
                "Content-Type": "application/json",
            }
            payload = {
                "url": upgrade_url,
                "limit": 1,
                "depth": 0,
                "formats": ["markdown"],
                "render": True,
            }
            try:
                resp = requests.post(CF_CRAWL_URL, headers=headers, json=payload)
                if resp.ok:
                    result = resp.json().get("result", "")
                    job_id = result.get("id") if isinstance(result, dict) else result
                    if job_id:
                        for _ in range(CRAWL_POLL_MAX):
                            time.sleep(CRAWL_POLL_INTERVAL)
                            status_resp = requests.get(
                                f"{CF_CRAWL_URL}/{job_id}",
                                headers={"Authorization": f"Bearer {CF_API_TOKEN}"},
                            )
                            if not status_resp.ok:
                                continue
                            data = status_resp.json().get("result", {})
                            status = data.get("status", "")
                            if status in ("completed", "cancelled_due_to_timeout", "cancelled_due_to_limits"):
                                records = data.get("records", [])
                                for r in records:
                                    md = r.get("markdown", "")
                                    if r.get("status") == "completed" and md and len(md) > existing_len:
                                        post.content = post.content.rstrip() + f"\n\n## Crawl Content\n\n{md[:80000]}\n"
                                        note_path.write_text(frontmatter.dumps(post))
                                        upgraded += 1
                                        print(f"  ✓ Upgraded: {note_path.name} (+{len(md)} chars)")
                                break
                            if status == "errored":
                                break
            except Exception as e:
                print(f"  ⚠ Crawl upgrade failed for {upgrade_url}: {e}")
        crawl_upgrade_queue.clear()

    print(f"\nDone: {hydrated} stubs hydrated, {deduped} duplicates discarded, {upgraded} crawl-upgraded")


def retry_failures():
    """--retry mode: retry failed crawls from the manifest."""
    entries = load_manifest()
    if not entries:
        print("No crawl failures to retry.")
        return

    now = datetime.now()
    retried = 0
    succeeded = 0

    for entry in entries:
        if entry.get("abandoned"):
            continue

        next_retry = entry.get("next_retry_after", "")
        if next_retry:
            try:
                retry_time = datetime.fromisoformat(next_retry)
                if now < retry_time:
                    continue  # Not yet time
            except (ValueError, TypeError):
                pass

        url = entry["url"]
        print(f"  Retrying: {url} (attempt {entry.get('retry_count', 0) + 1})")
        retried += 1

        content = crawl_url(url)
        if content:
            content = content[:80000]
            note_path = find_note_by_source_url(url)
            if note_path and note_path.exists():
                try:
                    post = frontmatter.load(note_path)
                    post.content = post.content.rstrip() + f"\n\n{content}\n"
                    note_path.write_text(frontmatter.dumps(post))
                    print(f"  ✓ Hydrated in place: {note_path.name}")
                except Exception as e:
                    print(f"  ⚠ Content fetched but write failed: {e}", file=sys.stderr)
            else:
                print(f"  ⚠ Content fetched but note not found for {url}", file=sys.stderr)
            remove_from_manifest(url)
            succeeded += 1
        else:
            log_failure(url, "retry_failed", entry.get("note_path", ""))

    print(f"\nRetry done: {retried} attempted, {succeeded} succeeded")


def main():
    if "--retry" in sys.argv:
        retry_failures()
    else:
        process_inbox()


if __name__ == "__main__":
    main()
