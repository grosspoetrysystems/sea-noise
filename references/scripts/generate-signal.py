#!/usr/bin/env python3
"""
Generate signal/queue.md — a rolling read/watch/listen queue.

Scans notes with consumed: false, scores by assertion overlap, rising topics,
and recency, then buckets by estimated read time.

Pure Python — no Claude calls. Designed to run fast in the daily chain.
"""
import re
from datetime import datetime, timedelta
from pathlib import Path

import frontmatter

VAULT = Path(__file__).parent.parent
SIGNAL_DIR = VAULT / "signal"
QUEUE_PATH = SIGNAL_DIR / "queue.md"
SCAN_DIRS = [VAULT / "notes" / "fleeting", VAULT / "notes" / "references"]
ASSERTIONS_DIR = VAULT / "notes" / "assertions"
TOPIC_FREQ_PATH = VAULT / "_meta" / "topic-frequencies.md"

# Read speed assumptions (words per minute)
WPM_ARTICLE = 250
WPM_TRANSCRIPT = 150
STALE_DAYS = 14


def word_count(text: str) -> int:
    return len(text.split())


def estimate_minutes(post: frontmatter.Post) -> int:
    """Estimate read time in minutes."""
    wc = word_count(post.content)
    # Transcripts are slower to parse
    has_transcript = "## Transcript" in post.content
    wpm = WPM_TRANSCRIPT if has_transcript else WPM_ARTICLE
    return max(1, round(wc / wpm))


def get_assertion_tags() -> set[str]:
    """Collect all tier-2 domain tags from active assertions."""
    tags = set()
    if not ASSERTIONS_DIR.exists():
        return tags
    for path in ASSERTIONS_DIR.glob("*.md"):
        try:
            post = frontmatter.load(path)
        except Exception:
            continue
        status = str(post.get("status", "active")).lower()
        if status != "active":
            continue
        note_tags = post.get("tags", [])
        if isinstance(note_tags, str):
            note_tags = [t.strip() for t in note_tags.split(",")]
        for t in note_tags:
            t = t.strip().lstrip("#")
            if "/" in t:  # tier-2 domain tags
                tags.add(t)
    return tags


def get_rising_topics() -> set[str]:
    """Parse _meta/topic-frequencies.md for topics marked as rising."""
    rising = set()
    if not TOPIC_FREQ_PATH.exists():
        return rising
    content = TOPIC_FREQ_PATH.read_text(encoding="utf-8", errors="ignore")
    # Look for lines with "rising" trend
    for line in content.splitlines():
        if "rising" in line.lower():
            # Extract tag from table row: | tag | count | last_seen | trend |
            parts = [p.strip() for p in line.split("|") if p.strip()]
            if parts:
                tag = parts[0].lstrip("#").strip()
                if tag:
                    rising.add(tag)
    return rising


def score_note(post: frontmatter.Post, assertion_tags: set[str],
               rising_topics: set[str], today: datetime) -> int:
    """Score a note for queue priority."""
    score = 0
    note_tags = post.get("tags", [])
    if isinstance(note_tags, str):
        note_tags = [t.strip() for t in note_tags.split(",")]
    normalized = {t.strip().lstrip("#") for t in note_tags}

    # Assertion tag overlap: +3 per matching domain tag
    for t in normalized:
        if t in assertion_tags:
            score += 3

    # Rising topic: +2 per match
    for t in normalized:
        if t in rising_topics:
            score += 2

    # Recency: +1 if from last 7 days
    date_str = str(post.get("date", ""))
    try:
        note_date = datetime.strptime(date_str, "%Y-%m-%d")
        if (today - note_date).days <= 7:
            score += 1
    except (ValueError, TypeError):
        pass

    return score


def format_item(title: str, path: str, minutes: int, score: int,
                tags: list[str], source: str) -> str:
    """Format a single queue item as a markdown line."""
    tag_str = " ".join(f"`{t}`" for t in tags[:3] if t)
    source_note = ""
    if source:
        source_note = f" — [source]({source})"
    return f"- [ ] [[{Path(path).stem}|{title}]] (~{minutes} min) {tag_str}{source_note}"


def main():
    today = datetime.now()
    assertion_tags = get_assertion_tags()
    rising_topics = get_rising_topics()

    items = []  # (score, minutes, title, rel_path, tags, source, date_str, bucket)

    for scan_dir in SCAN_DIRS:
        if not scan_dir.exists():
            continue
        for path in scan_dir.rglob("*.md"):
            try:
                post = frontmatter.load(path)
            except Exception:
                continue

            consumed = post.get("consumed")
            if consumed is not False:  # only include explicitly unconsumed
                continue

            # Skip low-signal content (profile pages, stubs, landing pages)
            if post.get("low-signal") is True:
                continue

            # Skip mood/anchor items — already consumed by definition
            if post.get("mood") is True:
                continue
            if post.get("content-type") == "anchor":
                continue

            title = post.get("title", path.stem.replace("-", " "))
            source = str(post.get("source", ""))
            if source and not source.startswith("http"):
                source = ""
            note_tags = post.get("tags", [])
            if isinstance(note_tags, str):
                note_tags = [t.strip() for t in note_tags.split(",")]

            minutes = estimate_minutes(post)
            score = score_note(post, assertion_tags, rising_topics, today)
            date_str = str(post.get("date", ""))
            rel_path = str(path.relative_to(VAULT))

            # Determine content type
            ct = str(post.get("content-type", ""))
            if not ct:
                # Infer from source URL and content
                src = source.lower()
                if any(h in src for h in ("youtube.com", "youtu.be", "spotify.com",
                                           "podcasts.apple", "overcast.fm")):
                    ct = "listen-watch"
                elif "## Transcript" in post.content:
                    ct = "listen-watch"
                elif any(k in post.content.lower()[:500] for k in
                         ("api reference", "documentation", "manual", "getting started",
                          "installation", "## usage", "```")):
                    ct = "tool-reference"
                else:
                    ct = "deep-read"

            items.append((score, minutes, title, rel_path, note_tags,
                          source, date_str, ct))

    # Sort by score descending, then recency
    items.sort(key=lambda x: (-x[0], x[6]), reverse=False)
    items.sort(key=lambda x: -x[0])

    # Group by content type
    deep_reads = [i for i in items if i[7] == "deep-read"]
    listen_watch = [i for i in items if i[7] == "listen-watch"]
    tool_ref = [i for i in items if i[7] == "tool-reference"]

    # Priority items (score >= 3, assertion-aligned)
    priority = [i for i in items if i[0] >= 3]

    # Stale items (>14 days unconsumed)
    stale = []
    for i in items:
        try:
            d = datetime.strptime(i[6], "%Y-%m-%d")
            if (today - d).days > STALE_DAYS:
                stale.append(i)
        except (ValueError, TypeError):
            pass

    # Domain summary
    domain_counts: dict[str, int] = {}
    for i in items:
        for t in i[4]:
            t = t.strip().lstrip("#")
            if "/" in t:
                domain_counts[t] = domain_counts.get(t, 0) + 1

    # Build output
    lines = [
        "---",
        "generated: true",
        f"updated: {today.strftime('%Y-%m-%d %H:%M')}",
        "---",
        "",
        f"# Signal Queue ({len(items)} unconsumed)",
        "",
    ]

    if priority:
        lines.append("## Priority (assertion-aligned)")
        lines.append("")
        for i in priority[:10]:
            lines.append(format_item(i[2], i[3], i[1], i[0], i[4], i[5]))
        lines.append("")

    if deep_reads:
        lines.append(f"## Deep Reads — {len(deep_reads)} items")
        lines.append("")
        for i in deep_reads[:20]:
            lines.append(format_item(i[2], i[3], i[1], i[0], i[4], i[5]))
        lines.append("")

    if listen_watch:
        lines.append(f"## Listen / Watch — {len(listen_watch)} items")
        lines.append("")
        for i in listen_watch[:20]:
            lines.append(format_item(i[2], i[3], i[1], i[0], i[4], i[5]))
        lines.append("")

    if tool_ref:
        lines.append(f"## Tools & Reference — {len(tool_ref)} items")
        lines.append("")
        for i in tool_ref[:20]:
            lines.append(format_item(i[2], i[3], i[1], i[0], i[4], i[5]))
        lines.append("")

    if domain_counts:
        lines.append("## By Domain")
        lines.append("")
        lines.append("| Domain | Count |")
        lines.append("|--------|-------|")
        for domain, count in sorted(domain_counts.items(),
                                     key=lambda x: -x[1]):
            lines.append(f"| {domain} | {count} |")
        lines.append("")

    if stale:
        lines.append(f"## Stale Queue (>{STALE_DAYS} days unconsumed) — {len(stale)} items")
        lines.append("")
        for i in stale[:10]:
            lines.append(format_item(i[2], i[3], i[1], i[0], i[4], i[5]))
        lines.append("")

    if not items:
        lines.append("*Queue empty — nothing unconsumed.*")
        lines.append("")

    SIGNAL_DIR.mkdir(exist_ok=True)
    QUEUE_PATH.write_text("\n".join(lines), encoding="utf-8")
    print(f"Signal queue generated: {len(items)} items → {QUEUE_PATH}")


if __name__ == "__main__":
    main()
