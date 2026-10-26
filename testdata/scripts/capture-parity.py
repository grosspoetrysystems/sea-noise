#!/usr/bin/env python3
"""Capture parity-critical extractions from the fixture vault.

Mirrors the Python prototype's behavior verbatim so the Go port can
diff its output against the JSON files this script produces. Each
surface (frontmatter, wikilinks) is captured as one JSON file per
fixture note, plus an index of all notes per surface.

This script uses pure stdlib by design — the parity surface relevant
to P1-W3 (T03 markdown engine) does not need lancedb / sentence-
transformers / etc. When the heavier scripts get ported in later
waves, this script's coverage will extend to those surfaces too.

Sources for the captured behaviors:
- parse_frontmatter, walk_vault: references/scripts/_rag/indexer.py
- WIKILINK regex, link-walk exclude set: references/scripts/audit-links.py

If the prototype's sources change, update this script to match — the
golden files are the parity contract, and they should always reflect
what the prototype WOULD produce on the fixture vault today.
"""
from __future__ import annotations

import json
import re
import sys
from datetime import datetime
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
FIXTURE_VAULT = SCRIPT_DIR.parent / "fixture-vault"
EXPECTED_ROOT = SCRIPT_DIR.parent / "expected"


# === parse_frontmatter — copied from references/scripts/_rag/indexer.py ===

def parse_frontmatter(content: str) -> tuple[str, str, str]:
    """Extract title and tags from YAML frontmatter. Returns (title, tags, body).

    Reproduces references/scripts/_rag/indexer.py:parse_frontmatter byte-for-byte.
    """
    title = ""
    tags = ""
    body = content

    if content.startswith("---"):
        end = content.find("\n---", 3)
        if end != -1:
            fm = content[3:end]
            body = content[end + 4:].lstrip()

            for line in fm.splitlines():
                if line.startswith("title:"):
                    title = line[6:].strip().strip('"').strip("'")
                elif line.startswith("tags:"):
                    raw = line[5:].strip()
                    raw = raw.strip("[]")
                    tags = " ".join(t.strip() for t in raw.split(",") if t.strip())

    if not title:
        m = re.search(r"^#\s+(.+)", body, re.MULTILINE)
        if m:
            title = m.group(1).strip()

    return title, tags, body


# === walk_vault rules — from references/scripts/_rag/indexer.py ===
INDEXER_EXCLUDE_DIRS = {"_rag", "_meta", "templates", ".git", ".obsidian", "signal", "bloom"}

# === WIKILINK regex and walk rules — from references/scripts/audit-links.py ===
WIKILINK = re.compile(r"\[\[([^\]|#]+)(?:[#|][^\]]+)?\]\]")
LINKS_EXCLUDE_DIRS = {"_rag", ".git", ".obsidian", ".claude", "templates", "_scripts"}


def walk(root: Path, exclude_dirs: set[str]):
    for path in sorted(root.rglob("*.md")):
        rel = path.relative_to(root)
        if any(part in exclude_dirs for part in rel.parts):
            continue
        yield path


def title_fallback_from_stem(path: Path) -> str:
    """Filename-stem fallback per indexer.py main()."""
    return path.stem.replace("-", " ").replace("_", " ")


def capture_frontmatter():
    target = EXPECTED_ROOT / "frontmatter"
    target.mkdir(parents=True, exist_ok=True)

    records = []
    for path in walk(FIXTURE_VAULT, INDEXER_EXCLUDE_DIRS):
        rel = str(path.relative_to(FIXTURE_VAULT))
        try:
            content = path.read_text(encoding="utf-8", errors="ignore")
        except Exception:
            continue
        if not content.strip():
            continue

        title, tags, body = parse_frontmatter(content)
        if not title:
            title = title_fallback_from_stem(path)

        record = {
            "id": rel,
            "path": rel,
            "title": title,
            "tags": tags,
            "body_preview": body[:4000],
        }
        records.append(record)

    (target / "index.json").write_text(
        json.dumps(records, indent=2, ensure_ascii=False, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print(f"frontmatter: {len(records)} records -> {target}/index.json")


def capture_links():
    target = EXPECTED_ROOT / "links"
    target.mkdir(parents=True, exist_ok=True)

    all_stems = set()
    for path in walk(FIXTURE_VAULT, LINKS_EXCLUDE_DIRS):
        all_stems.add(path.stem.lower())

    records = []
    broken = []
    for path in walk(FIXTURE_VAULT, LINKS_EXCLUDE_DIRS):
        rel = str(path.relative_to(FIXTURE_VAULT))
        try:
            text = path.read_text(encoding="utf-8", errors="replace")
        except Exception:
            continue

        targets = []
        for match in WIKILINK.finditer(text):
            t = match.group(1).strip()
            targets.append(t)
            if t.lower() not in all_stems:
                broken.append({"source": rel, "target": t})

        records.append({"source": rel, "links": targets})

    (target / "per-note.json").write_text(
        json.dumps(records, indent=2, ensure_ascii=False, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    (target / "broken.json").write_text(
        json.dumps(broken, indent=2, ensure_ascii=False, sort_keys=True) + "\n",
        encoding="utf-8",
    )

    total_links = sum(len(r["links"]) for r in records)
    print(f"links: {len(records)} notes, {total_links} links, {len(broken)} broken -> {target}/")


def main():
    if not FIXTURE_VAULT.is_dir():
        print(f"FIXTURE_VAULT does not exist: {FIXTURE_VAULT}", file=sys.stderr)
        sys.exit(1)
    capture_frontmatter()
    capture_links()


if __name__ == "__main__":
    main()


# === audit-links report — copied from references/scripts/audit-links.py ===

AUDIT_REPORT_DATE = "2026-05-21"  # fixed for golden determinism; bump deliberately


def capture_audit_links_report():
    """Capture the audit-links report content (without writing to vault)."""
    target = EXPECTED_ROOT / "audit-links"
    target.mkdir(parents=True, exist_ok=True)

    # Reuse the walk + LINKS_EXCLUDE_DIRS already defined above.
    all_stems = set()
    for path in walk(FIXTURE_VAULT, LINKS_EXCLUDE_DIRS):
        all_stems.add(path.stem.lower())

    broken = []
    for path in walk(FIXTURE_VAULT, LINKS_EXCLUDE_DIRS):
        rel = str(path.relative_to(FIXTURE_VAULT))
        try:
            text = path.read_text(encoding="utf-8", errors="replace")
        except Exception:
            continue
        for match in WIKILINK.finditer(text):
            target_stem = match.group(1).strip()
            if target_stem.lower() not in all_stems:
                broken.append((rel, target_stem))

    lines = [f"# Broken Links Audit — {AUDIT_REPORT_DATE}\n"]
    if not broken:
        lines.append("No broken links found.\n")
    else:
        lines.append(f"Found **{len(broken)}** broken wikilinks:\n")
        grouped = {}
        for source, t in broken:
            grouped.setdefault(source, []).append(t)
        for source in sorted(grouped):
            lines.append(f"\n### {source}")
            for t in sorted(grouped[source]):
                lines.append(f"- [[{t}]]")
        lines.append("")

    (target / "report.md").write_text("\n".join(lines), encoding="utf-8")
    print(f"audit-links: {len(broken)} broken -> {target}/report.md")


if __name__ == "__main__":
    capture_audit_links_report()


# === generate-signal queue — copied from references/scripts/generate-signal.py ===

SIGNAL_REPORT_DATE = "2026-05-21"  # fixed for golden determinism; bump deliberately

# Tunables — mirror generate-signal.py module constants.
WPM_ARTICLE = 250
WPM_TRANSCRIPT = 150
STALE_DAYS = 14

SIGNAL_SCAN_DIRS = ["notes/fleeting", "notes/references"]
SIGNAL_ASSERTIONS_DIR = "notes/assertions"
SIGNAL_TOPIC_FREQ_PATH = "_meta/topic-frequencies.md"


def _word_count(text: str) -> int:
    return len(text.split())


def _estimate_minutes(content: str) -> int:
    wc = _word_count(content)
    has_transcript = "## Transcript" in content
    wpm = WPM_TRANSCRIPT if has_transcript else WPM_ARTICLE
    return max(1, round(wc / wpm))


def _get_assertion_tags(vault):
    import frontmatter
    tags = set()
    adir = vault / SIGNAL_ASSERTIONS_DIR
    if not adir.exists():
        return tags
    # Sort for deterministic ordering (prototype uses adir.glob which is FS-order).
    for path in sorted(adir.glob("*.md")):
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
            if "/" in t:
                tags.add(t)
    return tags


def _get_rising_topics(vault):
    rising = set()
    path = vault / SIGNAL_TOPIC_FREQ_PATH
    if not path.exists():
        return rising
    content = path.read_text(encoding="utf-8", errors="ignore")
    for line in content.splitlines():
        if "rising" in line.lower():
            parts = [p.strip() for p in line.split("|") if p.strip()]
            if parts:
                tag = parts[0].lstrip("#").strip()
                if tag:
                    rising.add(tag)
    return rising


def _score_note(post, assertion_tags, rising_topics, today):
    score = 0
    note_tags = post.get("tags", [])
    if isinstance(note_tags, str):
        note_tags = [t.strip() for t in note_tags.split(",")]
    normalized = {t.strip().lstrip("#") for t in note_tags}
    for t in normalized:
        if t in assertion_tags:
            score += 3
    for t in normalized:
        if t in rising_topics:
            score += 2
    date_str = str(post.get("date", ""))
    try:
        note_date = datetime.strptime(date_str, "%Y-%m-%d")
        if (today - note_date).days <= 7:
            score += 1
    except (ValueError, TypeError):
        pass
    return score


def _format_signal_item(title, path, minutes, score, tags, source):
    tag_str = " ".join(f"`{t}`" for t in tags[:3] if t)
    source_note = ""
    if source:
        source_note = f" — [source]({source})"
    from pathlib import Path as _P
    stem = _P(path).stem
    return f"- [ ] [[{stem}|{title}]] (~{minutes} min) {tag_str}{source_note}"


def capture_signal_queue():
    """Capture generate-signal.py output against the fixture vault."""
    import frontmatter

    target = EXPECTED_ROOT / "signal"
    target.mkdir(parents=True, exist_ok=True)

    vault = FIXTURE_VAULT
    today = datetime.strptime(SIGNAL_REPORT_DATE, "%Y-%m-%d")
    assertion_tags = _get_assertion_tags(vault)
    rising_topics = _get_rising_topics(vault)

    items = []  # (score, minutes, title, rel_path, tags, source, date_str, bucket)

    for scan_dir_rel in SIGNAL_SCAN_DIRS:
        scan_dir = vault / scan_dir_rel
        if not scan_dir.exists():
            continue
        # Sort for deterministic capture (prototype uses rglob, FS-order).
        for path in sorted(scan_dir.rglob("*.md")):
            try:
                post = frontmatter.load(path)
            except Exception:
                continue

            consumed = post.get("consumed")
            if consumed is not False:
                continue
            if post.get("low-signal") is True:
                continue
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

            minutes = _estimate_minutes(post.content)
            score = _score_note(post, assertion_tags, rising_topics, today)
            date_str = str(post.get("date", ""))
            rel_path = str(path.relative_to(vault))

            ct = str(post.get("content-type", ""))
            if not ct:
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

    items.sort(key=lambda x: (-x[0], x[6]), reverse=False)
    items.sort(key=lambda x: -x[0])

    deep_reads = [i for i in items if i[7] == "deep-read"]
    listen_watch = [i for i in items if i[7] == "listen-watch"]
    tool_ref = [i for i in items if i[7] == "tool-reference"]
    priority = [i for i in items if i[0] >= 3]

    stale = []
    for i in items:
        try:
            d = datetime.strptime(i[6], "%Y-%m-%d")
            if (today - d).days > STALE_DAYS:
                stale.append(i)
        except (ValueError, TypeError):
            pass

    domain_counts = {}
    for i in items:
        for t in i[4]:
            t = t.strip().lstrip("#")
            if "/" in t:
                domain_counts[t] = domain_counts.get(t, 0) + 1

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
            lines.append(_format_signal_item(i[2], i[3], i[1], i[0], i[4], i[5]))
        lines.append("")

    if deep_reads:
        lines.append(f"## Deep Reads — {len(deep_reads)} items")
        lines.append("")
        for i in deep_reads[:20]:
            lines.append(_format_signal_item(i[2], i[3], i[1], i[0], i[4], i[5]))
        lines.append("")

    if listen_watch:
        lines.append(f"## Listen / Watch — {len(listen_watch)} items")
        lines.append("")
        for i in listen_watch[:20]:
            lines.append(_format_signal_item(i[2], i[3], i[1], i[0], i[4], i[5]))
        lines.append("")

    if tool_ref:
        lines.append(f"## Tools & Reference — {len(tool_ref)} items")
        lines.append("")
        for i in tool_ref[:20]:
            lines.append(_format_signal_item(i[2], i[3], i[1], i[0], i[4], i[5]))
        lines.append("")

    if domain_counts:
        lines.append("## By Domain")
        lines.append("")
        lines.append("| Domain | Count |")
        lines.append("|--------|-------|")
        for domain, count in sorted(domain_counts.items(), key=lambda x: -x[1]):
            lines.append(f"| {domain} | {count} |")
        lines.append("")

    if stale:
        lines.append(f"## Stale Queue (>{STALE_DAYS} days unconsumed) — {len(stale)} items")
        lines.append("")
        for i in stale[:10]:
            lines.append(_format_signal_item(i[2], i[3], i[1], i[0], i[4], i[5]))
        lines.append("")

    if not items:
        lines.append("*Queue empty — nothing unconsumed.*")
        lines.append("")

    (target / "queue.md").write_text("\n".join(lines), encoding="utf-8")
    print(f"signal: {len(items)} items -> {target}/queue.md")


if __name__ == "__main__":
    capture_signal_queue()
