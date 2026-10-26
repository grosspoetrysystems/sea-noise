#!/usr/bin/env python3
"""
Scan vault for broken [[wikilinks]] and write a report to _meta/broken-links.md.
"""
import re
from pathlib import Path
from datetime import date

VAULT = Path(__file__).parent.parent
EXCLUDE_DIRS = {"_rag", ".git", ".obsidian", ".claude", "templates", "_scripts"}
WIKILINK = re.compile(r"\[\[([^\]|#]+)(?:[#|][^\]]+)?\]\]")

REPORT_PATH = VAULT / "_meta/broken-links.md"


def collect_note_stems() -> set[str]:
    """Collect all note stems (lowercased) in the vault."""
    stems = set()
    for md in VAULT.rglob("*.md"):
        if any(part in EXCLUDE_DIRS for part in md.relative_to(VAULT).parts):
            continue
        stems.add(md.stem.lower())
    return stems


def scan_links() -> list[tuple[str, str]]:
    """Return list of (source_file, broken_target) tuples."""
    all_stems = collect_note_stems()
    broken = []

    for md in VAULT.rglob("*.md"):
        rel = md.relative_to(VAULT)
        if any(part in EXCLUDE_DIRS for part in rel.parts):
            continue

        text = md.read_text(errors="replace")
        for match in WIKILINK.finditer(text):
            target = match.group(1).strip()
            if target.lower() not in all_stems:
                broken.append((str(rel), target))

    return broken


def main():
    broken = scan_links()
    today = date.today().isoformat()

    lines = [f"# Broken Links Audit — {today}\n"]
    if not broken:
        lines.append("No broken links found.\n")
    else:
        lines.append(f"Found **{len(broken)}** broken wikilinks:\n")
        grouped: dict[str, list[str]] = {}
        for source, target in broken:
            grouped.setdefault(source, []).append(target)
        for source in sorted(grouped):
            lines.append(f"\n### {source}")
            for target in sorted(grouped[source]):
                lines.append(f"- [[{target}]]")
        lines.append("")

    REPORT_PATH.parent.mkdir(parents=True, exist_ok=True)
    REPORT_PATH.write_text("\n".join(lines))
    print(f"Audit complete: {len(broken)} broken links → {REPORT_PATH.relative_to(VAULT)}")


if __name__ == "__main__":
    main()
