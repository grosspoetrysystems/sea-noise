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


# Re-bind main to include the new capture.
_prev_main = main


def main():  # noqa: F811
    _prev_main()
    capture_audit_links_report()

if __name__ == "__main__":
    capture_audit_links_report()
