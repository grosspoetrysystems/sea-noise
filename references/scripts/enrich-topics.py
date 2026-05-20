#!/usr/bin/env python3
"""
Research enrichment for vault topics that reach critical mass.

Reads topic-frequencies.md, identifies topics crossing the threshold,
optionally uses Cloudflare /crawl for web context, then uses Claude Code
(claude -p) to generate enrichment sections.

Uses claude -p (headless) — runs on your Max plan, no API key needed.

Usage:
  python3 enrich-topics.py                    # auto mode
  python3 enrich-topics.py "ai-agents"        # manual mode for specific topic
  python3 enrich-topics.py --note path/to/note.md  # enrich a single note
"""
import os
import re
import sys
import subprocess
import time
import argparse
from pathlib import Path
from datetime import date

import frontmatter
import requests

# --- Config ---
VAULT = Path(__file__).parent.parent
TOPIC_FREQ = VAULT / "_meta/topic-frequencies.md"
ENRICHMENT_DIR = VAULT / "notes/references/briefs"
ENRICHMENT_LOG = VAULT / "_meta/enrichment-log.md"
THRESHOLD = 5

CF_ACCOUNT_ID = os.environ.get("CF_ACCOUNT_ID", "")
CF_API_TOKEN = os.environ.get("CF_API_TOKEN", "")
CF_CRAWL_URL = f"https://api.cloudflare.com/client/v4/accounts/{CF_ACCOUNT_ID}/browser-rendering/crawl"

MAX_CRAWL_PAGES = 20
CRAWL_POLL_INTERVAL = 10
CRAWL_POLL_MAX = 60

EXCLUDE_DIRS = {"_rag", "_meta", "templates", ".git", ".obsidian", "_scripts", ".claude"}


# --- Claude via CLI ---

def ask_claude(prompt: str) -> str | None:
    """Run a prompt through claude -p and return the response."""
    env = os.environ.copy()
    env["CLAUDECODE"] = ""
    try:
        result = subprocess.run(
            ["claude", "-p", prompt, "--dangerously-skip-permissions"],
            capture_output=True, text=True, timeout=600, env=env,
        )
        if result.returncode == 0 and result.stdout.strip():
            return result.stdout.strip()
        if result.stderr:
            print(f"    claude stderr: {result.stderr[:200]}", file=sys.stderr)
        return None
    except subprocess.TimeoutExpired:
        print("    claude -p timed out", file=sys.stderr)
        return None
    except Exception as e:
        print(f"    claude -p error: {e}", file=sys.stderr)
        return None


# --- Cloudflare /crawl ---

def crawl_url(url: str, limit: int = MAX_CRAWL_PAGES, depth: int = 2) -> list[dict] | None:
    if not CF_ACCOUNT_ID or not CF_API_TOKEN:
        print("  ⚠ CF_ACCOUNT_ID / CF_API_TOKEN not set, skipping crawl", file=sys.stderr)
        return None

    headers = {
        "Authorization": f"Bearer {CF_API_TOKEN}",
        "Content-Type": "application/json",
    }
    payload = {
        "url": url,
        "limit": limit,
        "depth": depth,
        "formats": ["markdown"],
        "render": False,
    }

    resp = requests.post(CF_CRAWL_URL, headers=headers, json=payload)
    if not resp.ok:
        print(f"  ✗ Crawl start failed: {resp.status_code} {resp.text}", file=sys.stderr)
        return None

    result = resp.json().get("result", "")
    job_id = result.get("id") if isinstance(result, dict) else result
    if not job_id:
        print(f"  ✗ No job ID returned", file=sys.stderr)
        return None

    print(f"  Crawl job started: {job_id}")

    for i in range(CRAWL_POLL_MAX):
        time.sleep(CRAWL_POLL_INTERVAL)
        status_resp = requests.get(
            f"{CF_CRAWL_URL}/{job_id}",
            headers={"Authorization": f"Bearer {CF_API_TOKEN}"},
        )
        if not status_resp.ok:
            continue
        data = status_resp.json().get("result", {})
        status = data.get("status", "")
        finished = data.get("finished", 0)
        total = data.get("total", 0)
        print(f"  Poll {i+1}: {status} ({finished}/{total})")

        if status in ("completed", "cancelled_due_to_timeout", "cancelled_due_to_limits"):
            records = data.get("records", [])
            return [r for r in records if r.get("status") == "completed"]
        if status == "errored":
            print(f"  ✗ Crawl errored", file=sys.stderr)
            return None

    print(f"  ✗ Crawl timed out after {CRAWL_POLL_MAX} polls", file=sys.stderr)
    return None


# --- Vault reading ---

def read_topic_frequencies() -> list[dict]:
    if not TOPIC_FREQ.exists():
        return []
    text = TOPIC_FREQ.read_text()
    topics = []
    for line in text.strip().split("\n"):
        if line.startswith("|") and "topic" not in line and "---" not in line:
            parts = [p.strip() for p in line.strip("|").split("|")]
            if len(parts) >= 5:
                topics.append({
                    "topic": parts[0],
                    "first_seen": parts[1],
                    "last_seen": parts[2],
                    "count": int(parts[3]),
                    "trend": parts[4],
                })
    return topics


def get_enriched_topics() -> set[str]:
    if not ENRICHMENT_LOG.exists():
        return set()
    text = ENRICHMENT_LOG.read_text()
    return set(re.findall(r"^- (.+?) —", text, re.MULTILINE))


def collect_notes_for_topic(topic: str) -> list[dict]:
    notes = []
    for md in VAULT.rglob("*.md"):
        rel = md.relative_to(VAULT)
        if any(part in EXCLUDE_DIRS for part in rel.parts):
            continue
        try:
            post = frontmatter.load(md)
        except Exception:
            continue
        tags = post.get("tags", [])
        if isinstance(tags, str):
            tags = [tags]
        tag_match = any(topic in t.replace("#", "") for t in tags)
        slug_match = topic.replace("-", " ") in md.stem.replace("-", " ")
        if tag_match or slug_match:
            notes.append({
                "path": str(rel),
                "title": post.get("title", md.stem),
                "tags": tags,
                "source": post.get("source", ""),
                "excerpt": post.content[:500],
            })
    return notes


def collect_note_context(note_path: Path) -> dict:
    post = frontmatter.load(note_path)
    tags = post.get("tags", [])
    if isinstance(tags, str):
        tags = [tags]
    return {
        "path": str(note_path.relative_to(VAULT)),
        "title": post.get("title", note_path.stem),
        "tags": tags,
        "source": post.get("source", ""),
        "content": post.content[:3000],
    }


# --- Enrichment ---

def enrich_with_claude(topic: str, vault_notes: list[dict], crawled_content: str | None) -> str | None:
    notes_summary = "\n".join(
        f"- {n['title']} ({n['path']}): {n['excerpt'][:200]}..."
        for n in vault_notes
    )

    web_section = ""
    if crawled_content:
        web_section = f"\n\n## Web Research\n{crawled_content[:30000]}"

    prompt = (
        f"You are a research assistant for a personal knowledge vault. "
        f"The topic '{topic}' has reached critical mass with {len(vault_notes)} notes.\n\n"
        f"## Existing Vault Notes\n{notes_summary}\n"
        f"{web_section}\n\n"
        f"Generate the following sections in markdown. Output ONLY the markdown, no preamble. "
        f"Be specific and actionable — reference real books, real podcasts, real people, real papers. "
        f"Do NOT make up titles or authors. If unsure, say so.\n\n"
        f"### Follow-up Questions\n"
        f"5-7 open questions worth exploring based on the notes and gaps you see. "
        f"Genuinely thought-provoking, not generic.\n\n"
        f"### Related Materials\n"
        f"5-10 specific articles, papers, talks, or tools relevant to this cluster. "
        f"Include URLs where possible. Prioritize recent (2024-2026) and seminal works.\n\n"
        f"### Book & Podcast Recommendations\n"
        f"3-5 books and 3-5 podcast episodes (or series) directly relevant. "
        f"Include author/host names. Brief (1 sentence) on why each is relevant.\n\n"
        f"### Suggested Deep Dives\n"
        f"2-3 specific research directions that would strengthen this topic cluster. "
        f"For each, suggest a starting URL or search query."
    )
    return ask_claude(prompt)


def enrich_single_note(note_path: Path, crawled_content: str | None) -> str | None:
    note = collect_note_context(note_path)

    # Strip existing Research Context so Claude doesn't comment on it
    content = note["content"]
    rc_idx = content.find("## Research Context")
    if rc_idx > 0:
        content = content[:rc_idx].rstrip()

    web_section = ""
    if crawled_content:
        web_section = f"\n\n## Web Research\n{crawled_content[:30000]}"

    prompt = (
        f"You are a research assistant for a personal knowledge vault. "
        f"Enrich this note with deeper context.\n\n"
        f"## Note: {note['title']}\n"
        f"Tags: {', '.join(note['tags'])}\n"
        f"Source: {note['source']}\n\n"
        f"{content}\n"
        f"{web_section}\n\n"
        f"Generate the following sections in markdown. Output ONLY the markdown sections below — "
        f"no preamble, no commentary, no meta-observations. Start directly with the first heading.\n\n"
        f"### Follow-up Questions\n"
        f"3-5 open questions this note raises worth exploring.\n\n"
        f"### Related Materials\n"
        f"5-8 specific articles, papers, talks, or tools related to this note. "
        f"Include URLs where possible.\n\n"
        f"### Book & Podcast Recommendations\n"
        f"2-3 books and 2-3 podcast episodes directly relevant. "
        f"Include author/host names and a 1-sentence reason."
    )
    return ask_claude(prompt)


# --- Output ---

def write_enrichment_note(topic: str, vault_notes: list[dict], enrichment: str):
    today = date.today().isoformat()
    slug = topic.lower().replace(" ", "-")
    filename = f"{today}-{slug}-research-brief.md"
    out_path = ENRICHMENT_DIR / filename

    note_links = "\n".join(f"- [[{Path(n['path']).stem}]]" for n in vault_notes)

    content = (
        f"---\n"
        f"title: \"Research Brief: {topic}\"\n"
        f"tags:\n  - reference\n  - reference/brief\n  - research-brief\n  - {slug}\n"
        f"date: {today}\n"
        f"source: enrich-topics\n"
        f"---\n\n"
        f"# Research Brief: {topic}\n\n"
        f"Auto-generated enrichment for topic cluster **{topic}** "
        f"({len(vault_notes)} notes, threshold crossed).\n\n"
        f"## Vault Notes in This Cluster\n{note_links}\n\n"
        f"{enrichment}\n"
    )

    ENRICHMENT_DIR.mkdir(parents=True, exist_ok=True)
    out_path.write_text(content)
    print(f"  ✓ Written: {out_path.relative_to(VAULT)}")
    return out_path


def append_enrichment_to_note(note_path: Path, enrichment: str) -> bool:
    post = frontmatter.load(note_path)
    if "\n## Follow-up Questions" in post.content or post.content.startswith("## Follow-up Questions"):
        print(f"  Already enriched: {note_path.stem}")
        return False

    post.content = post.content.rstrip() + f"\n\n{enrichment}\n"
    post["enriched"] = date.today().isoformat()
    note_path.write_text(frontmatter.dumps(post))
    print(f"  ✓ Enriched: {note_path.relative_to(VAULT)}")
    return True


def log_enrichment(topic: str, note_path: Path):
    today = date.today().isoformat()
    ENRICHMENT_LOG.parent.mkdir(parents=True, exist_ok=True)
    line = f"- {topic} — {today} — {note_path.relative_to(VAULT)}\n"
    if ENRICHMENT_LOG.exists():
        text = ENRICHMENT_LOG.read_text()
    else:
        text = "# Enrichment Log\n\n"
    ENRICHMENT_LOG.write_text(text + line)


# --- Main ---

def enrich_topic(topic: str):
    print(f"\n▸ Enriching topic: {topic}")

    notes = collect_notes_for_topic(topic)
    if not notes:
        print(f"  No notes found for topic '{topic}'")
        return

    print(f"  Found {len(notes)} notes")

    crawled_text = None
    source_urls = [n["source"] for n in notes if n["source"] and n["source"].startswith("http")]
    if source_urls and CF_ACCOUNT_ID:
        print(f"  Crawling {len(source_urls)} source URLs...")
        all_records = []
        for url in source_urls[:3]:
            records = crawl_url(url, limit=5, depth=1)
            if records:
                all_records.extend(records)
        if all_records:
            crawled_text = "\n\n---\n\n".join(
                f"### {r.get('url', 'unknown')}\n{r.get('markdown', '')[:3000]}"
                for r in all_records[:10]
            )

    print(f"  Generating research brief...")
    enrichment = enrich_with_claude(topic, notes, crawled_text)
    if not enrichment:
        print(f"  ✗ Enrichment failed for {topic}")
        return

    out_path = write_enrichment_note(topic, notes, enrichment)
    log_enrichment(topic, out_path)


def enrich_note(note_path_str: str):
    note_path = VAULT / note_path_str if not Path(note_path_str).is_absolute() else Path(note_path_str)
    if not note_path.exists():
        print(f"  ✗ Note not found: {note_path}")
        return

    print(f"\n▸ Enriching note: {note_path.relative_to(VAULT)}")

    post = frontmatter.load(note_path)

    # Skip if already enriched by summarize-transcripts or a prior run
    if "## Research Context" in post.content:
        print(f"  Already has Research Context — skipping")
        return

    crawled_text = None
    source = post.get("source", "")
    if source and source.startswith("http") and CF_ACCOUNT_ID:
        print(f"  Crawling source: {source}")
        records = crawl_url(source, limit=10, depth=1)
        if records:
            crawled_text = "\n\n---\n\n".join(
                f"### {r.get('url', 'unknown')}\n{r.get('markdown', '')[:3000]}"
                for r in records[:10]
            )

    print(f"  Generating enrichment...")
    enrichment = enrich_single_note(note_path, crawled_text)
    if not enrichment:
        print(f"  ✗ Enrichment failed")
        return

    if append_enrichment_to_note(note_path, enrichment):
        log_enrichment(note_path.stem, note_path)


def auto_mode(threshold: int = THRESHOLD):
    topics = read_topic_frequencies()
    enriched = get_enriched_topics()

    candidates = [
        t for t in topics
        if t["count"] >= threshold and t["topic"] not in enriched
    ]

    if not candidates:
        print("No topics above threshold needing enrichment.")
        return

    print(f"Found {len(candidates)} topics to enrich: {[c['topic'] for c in candidates]}")
    for t in candidates:
        enrich_topic(t["topic"])


def main():
    parser = argparse.ArgumentParser(description="Enrich vault topics with research")
    parser.add_argument("topic", nargs="?", help="Specific topic to enrich")
    parser.add_argument("--note", help="Path to a specific note to enrich")
    parser.add_argument("--threshold", type=int, default=THRESHOLD,
                        help=f"Minimum notes for auto-enrichment (default: {THRESHOLD})")
    args = parser.parse_args()

    if args.note:
        enrich_note(args.note)
    elif args.topic:
        enrich_topic(args.topic)
    else:
        auto_mode(threshold=args.threshold)


if __name__ == "__main__":
    main()
