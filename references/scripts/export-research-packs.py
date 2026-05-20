#!/usr/bin/env python3
"""
Export assertion-driven research packs for external agents (ZeroClaw, NotebookLM).

Detects strong/emerging themes from the vault, then for each theme builds a
self-contained research pack that an external agent can consume without touching
the vault. The vault stays pristine — packs are read-only snapshots.

Output: export/research-packs/<theme>/
  - context.md        — user's thinking + evidence so far
  - sources.md        — URLs for NotebookLM or agent ingestion
  - brief.md          — gap analysis, open questions, suggested angles
  - agent-prompt.md   — ready-to-use prompt for a ZeroClaw agent

Pure Python — no Claude calls. Designed to run in weekly chain or on-demand.

Usage:
    python3 _scripts/export-research-packs.py                    # auto-detect top themes
    python3 _scripts/export-research-packs.py --theme governance  # specific theme
    python3 _scripts/export-research-packs.py --assertion path    # from assertion file
    python3 _scripts/export-research-packs.py --top 5             # top N themes
    python3 _scripts/export-research-packs.py --agent researcher  # set agent persona
"""
import argparse
import re
from datetime import datetime
from pathlib import Path

import frontmatter

VAULT = Path(__file__).parent.parent
EXPORT_DIR = VAULT / "export" / "research-packs"
NOTES_DIR = VAULT / "notes"
ATLAS_DIR = VAULT / "atlas"
BLOOM_DIR = VAULT / "bloom"
ASSERTIONS_DIR = NOTES_DIR / "assertions"
BRIEFS_DIR = NOTES_DIR / "references" / "briefs"
TOPIC_FREQ_PATH = VAULT / "_meta" / "topic-frequencies.md"
SIGNAL_PATH = VAULT / "signal" / "queue.md"

STRIP_SECTIONS = {"## Raw"}

# Agent personas — each shapes the brief.md and agent-prompt.md differently
AGENT_PERSONAS = {
    "researcher": {
        "role": "Deep Research Agent",
        "goal": "Find primary sources, academic papers, counter-arguments, and expert perspectives that deepen understanding of this theme. Produce a structured research report with citations.",
        "output": "A research report with: key findings, primary sources (with URLs), counter-arguments, expert quotes, and 3-5 follow-up threads worth pursuing.",
    },
    "teacher": {
        "role": "Learning Path Designer",
        "goal": "Design a structured learning curriculum for this theme. Identify foundational concepts, progression milestones, hands-on exercises, and recommended resources (books, courses, videos, projects).",
        "output": "A learning path with: prerequisites, 4-6 progressive modules, hands-on exercises per module, recommended resources (mix of media types), and a capstone project.",
    },
    "coach": {
        "role": "Startup / Project Coach",
        "goal": "Analyze this theme as a product or project opportunity. Identify market dynamics, competitive landscape, user needs, and concrete next steps. Challenge assumptions.",
        "output": "A coaching brief with: opportunity assessment, competitive scan, user personas, risks/assumptions to test, and a 2-week sprint plan for validation.",
    },
    "critic": {
        "role": "Devil's Advocate",
        "goal": "Challenge every assumption in the user's thinking on this theme. Find the strongest counter-arguments, identify blind spots, and stress-test the thesis.",
        "output": "A critique with: the strongest version of the opposing argument, blind spots in current thinking, evidence that contradicts the thesis, and steelman alternatives.",
    },
    "connector": {
        "role": "Cross-Domain Connector",
        "goal": "Find unexpected connections between this theme and adjacent fields. Look for analogies, patterns, and frameworks from other domains that illuminate this one.",
        "output": "A connections map with: 3-5 cross-domain analogies (with sources), transferable frameworks, surprising parallels, and synthesis opportunities.",
    },
}


def read_note(path: Path) -> frontmatter.Post | None:
    try:
        return frontmatter.load(str(path))
    except Exception:
        return None


def extract_wikilinks(text: str) -> list[str]:
    return re.findall(r"\[\[([^\]|]+?)(?:\|[^\]]+?)?\]\]", text)


def find_note_by_stem(stem: str) -> Path | None:
    stem_lower = stem.lower()
    for d in [NOTES_DIR, BLOOM_DIR, ATLAS_DIR]:
        if not d.exists():
            continue
        for f in d.rglob("*.md"):
            if f.stem.lower() == stem_lower:
                return f
    return None


def clean_body(content: str) -> str:
    """Strip noisy sections from note content."""
    body = content
    for section in STRIP_SECTIONS:
        pattern = rf"(?m)^{re.escape(section)}\s*\n[\s\S]*?(?=^## |\Z)"
        body = re.sub(pattern, "", body)
    return body.strip()


def parse_topic_frequencies() -> list[dict]:
    """Parse topic-frequencies.md into structured data."""
    topics = []
    if not TOPIC_FREQ_PATH.exists():
        return topics
    content = TOPIC_FREQ_PATH.read_text(encoding="utf-8", errors="ignore")
    for line in content.splitlines():
        parts = [p.strip() for p in line.split("|") if p.strip()]
        if len(parts) >= 4 and parts[0] != "topic" and "---" not in parts[0]:
            topics.append({
                "name": parts[0],
                "first_seen": parts[1],
                "last_seen": parts[2],
                "count": int(parts[3]) if parts[3].isdigit() else 0,
                "trend": parts[4] if len(parts) > 4 else "stable",
            })
    return topics


def load_assertions() -> list[dict]:
    """Load all active assertions with their evidence."""
    assertions = []
    if not ASSERTIONS_DIR.exists():
        return assertions
    for path in sorted(ASSERTIONS_DIR.glob("*.md")):
        post = read_note(path)
        if not post:
            continue
        status = str(post.get("status", "active")).lower()
        if status != "active":
            continue
        tags = post.get("tags", []) or []
        if isinstance(tags, str):
            tags = [t.strip() for t in tags.split(",")]
        evidence_links = extract_wikilinks(post.content)
        assertions.append({
            "path": path,
            "title": post.get("title", path.stem),
            "tags": [t.strip().lstrip("#") for t in tags],
            "content": post.content,
            "evidence_links": evidence_links,
            "status": status,
        })
    return assertions


def detect_themes(top_n: int = 3) -> list[dict]:
    """
    Auto-detect the strongest themes by combining:
    - Active assertions (strongest signal — user declared intent)
    - Rising topics with high count
    - New topics (emerging clusters)
    """
    themes = []
    topics = parse_topic_frequencies()
    assertions = load_assertions()

    # Assertion-backed themes (highest priority)
    for a in assertions:
        domain_tags = [t for t in a["tags"] if "/" in t]
        related_topics = []
        for t in topics:
            for dt in domain_tags:
                if t["name"] in dt or dt.split("/")[-1] in t["name"]:
                    related_topics.append(t)
        themes.append({
            "name": a["title"],
            "slug": a["path"].stem,
            "type": "assertion",
            "assertion": a,
            "topics": related_topics,
            "score": 100 + sum(t["count"] for t in related_topics),
            "tags": domain_tags,
        })

    # Rising topics not covered by assertions
    assertion_topic_names = set()
    for a in assertions:
        for t in a["tags"]:
            assertion_topic_names.add(t.split("/")[-1] if "/" in t else t)

    for t in topics:
        if t["trend"] in ("rising", "new") and t["count"] >= 5:
            if t["name"] not in assertion_topic_names:
                themes.append({
                    "name": t["name"].replace("-", " ").title(),
                    "slug": t["name"],
                    "type": "rising-topic",
                    "assertion": None,
                    "topics": [t],
                    "score": t["count"] + (10 if t["trend"] == "new" else 5),
                    "tags": [t["name"]],
                })

    themes.sort(key=lambda x: -x["score"])
    return themes[:top_n]


def gather_vault_notes(theme: dict) -> list[tuple[Path, frontmatter.Post]]:
    """Gather all vault notes relevant to a theme."""
    notes = []
    seen = set()

    # From assertion evidence links
    if theme["assertion"]:
        for stem in theme["assertion"]["evidence_links"]:
            path = find_note_by_stem(stem)
            if path and path not in seen:
                post = read_note(path)
                if post:
                    notes.append((path, post))
                    seen.add(path)

    # From atlas MOCs — find sections matching theme tags
    for moc_path in ATLAS_DIR.glob("*.md"):
        if moc_path.name == "index.md":
            continue
        moc = read_note(moc_path)
        if not moc:
            continue
        moc_links = extract_wikilinks(moc.content)
        for stem in moc_links:
            path = find_note_by_stem(stem)
            if not path or path in seen:
                continue
            post = read_note(path)
            if not post:
                continue
            note_tags = post.get("tags", []) or []
            if isinstance(note_tags, str):
                note_tags = [note_tags]
            note_tags_clean = {t.strip().lstrip("#") for t in note_tags}
            # Check if note's tags overlap with theme's tags
            for theme_tag in theme["tags"]:
                if any(theme_tag in nt or nt in theme_tag for nt in note_tags_clean):
                    notes.append((path, post))
                    seen.add(path)
                    break

    # From briefs
    if BRIEFS_DIR.exists():
        for brief_path in BRIEFS_DIR.glob("*.md"):
            for tag in theme["tags"]:
                tag_slug = tag.split("/")[-1] if "/" in tag else tag
                if tag_slug in brief_path.stem:
                    if brief_path not in seen:
                        post = read_note(brief_path)
                        if post:
                            notes.append((brief_path, post))
                            seen.add(brief_path)

    return notes


def extract_source_urls(notes: list[tuple[Path, frontmatter.Post]]) -> list[dict]:
    """Extract all source URLs from notes for NotebookLM ingestion."""
    urls = []
    seen = set()
    for path, post in notes:
        source = str(post.get("source", ""))
        if source.startswith("http") and source not in seen:
            title = post.get("title", path.stem)
            content_type = str(post.get("content-type", "deep-read"))
            consumed = post.get("consumed", None)
            urls.append({
                "url": source,
                "title": title,
                "content_type": content_type,
                "consumed": consumed,
            })
            seen.add(source)

    # Also extract URLs from research brief "Related Materials" sections
    for path, post in notes:
        if "briefs" in str(path):
            for match in re.finditer(r'\[([^\]]+)\]\((https?://[^\)]+)\)', post.content):
                title, url = match.group(1), match.group(2)
                if url not in seen:
                    urls.append({
                        "url": url,
                        "title": title,
                        "content_type": "external-recommendation",
                        "consumed": None,
                    })
                    seen.add(url)

    return urls


def extract_open_questions(theme: dict, notes: list[tuple[Path, frontmatter.Post]]) -> list[str]:
    """Collect all open questions from assertion + MOCs + briefs."""
    questions = []

    # From assertion
    if theme["assertion"]:
        content = theme["assertion"]["content"]
        oq_match = re.search(r"## Open Questions\s*\n([\s\S]*?)(?=^## |\Z)",
                            content, re.MULTILINE)
        if oq_match:
            for line in oq_match.group(1).splitlines():
                line = line.strip().lstrip("- ")
                if line:
                    questions.append(line)

    # From MOCs
    for moc_path in ATLAS_DIR.glob("*.md"):
        if moc_path.name == "index.md":
            continue
        moc = read_note(moc_path)
        if not moc:
            continue
        oq_match = re.search(r"## Open Questions\s*\n([\s\S]*?)(?=^## |\Z)",
                            moc.content, re.MULTILINE)
        if oq_match:
            for line in oq_match.group(1).splitlines():
                line = line.strip().lstrip("- ")
                if line and any(tag.split("/")[-1] in moc_path.stem
                               for tag in theme["tags"]):
                    questions.append(line)

    # From briefs (Follow-up Questions sections)
    for path, post in notes:
        if "briefs" in str(path):
            fq_match = re.search(r"## Follow-up Questions\s*\n([\s\S]*?)(?=^## |\Z)",
                                post.content, re.MULTILINE)
            if fq_match:
                for line in fq_match.group(1).splitlines():
                    line = line.strip().lstrip("- ").lstrip("0123456789. ")
                    if line:
                        questions.append(line)

    return list(dict.fromkeys(questions))  # dedupe preserving order


def build_context_md(theme: dict, notes: list[tuple[Path, frontmatter.Post]]) -> str:
    """Build context.md — user's thinking + evidence so far."""
    lines = [f"# {theme['name']}", ""]
    lines.append(f"*Theme type: {theme['type']}*")
    lines.append(f"*Exported: {datetime.now().strftime('%Y-%m-%d %H:%M')}*")
    lines.append("")

    # Assertion context
    if theme["assertion"]:
        a = theme["assertion"]
        lines.append("## Assertion")
        lines.append("")
        # Extract just the assertion and why sections
        for section in ("## Assertion", "## Why"):
            match = re.search(rf"(?m)^{re.escape(section)}\s*\n([\s\S]*?)(?=^## |\Z)",
                            a["content"])
            if match:
                lines.append(f"{section}")
                lines.append(match.group(1).strip())
                lines.append("")

    # Separate ideas from references
    ideas = []
    references = []
    for path, post in notes:
        tags = post.get("tags", []) or []
        if isinstance(tags, str):
            tags = [tags]
        if "idea" in tags:
            ideas.append((path, post))
        elif "briefs" not in str(path):
            references.append((path, post))

    if ideas:
        lines.append("## Original Ideas")
        lines.append("")
        for path, post in ideas:
            title = post.get("title", path.stem)
            body = clean_body(post.content)
            lines.append(f"### {title}")
            lines.append("")
            lines.append(body[:2000])
            lines.append("")
            lines.append("---")
            lines.append("")

    if references:
        lines.append("## Collected References")
        lines.append("")
        for path, post in references:
            title = post.get("title", path.stem)
            source = post.get("source", "")
            body = clean_body(post.content)
            lines.append(f"### {title}")
            if source:
                lines.append(f"Source: {source}")
            lines.append("")
            # Truncate to key sections (Research Context preferred, else first 1500 chars)
            rc_match = re.search(r"## Research Context\s*\n([\s\S]*?)(?=^## |\Z)",
                                body, re.MULTILINE)
            if rc_match:
                lines.append(rc_match.group(1).strip()[:1500])
            else:
                lines.append(body[:1500])
            lines.append("")
            lines.append("---")
            lines.append("")

    return "\n".join(lines)


def build_sources_md(theme: dict, urls: list[dict]) -> str:
    """Build sources.md — URLs for NotebookLM or agent ingestion."""
    lines = [f"# Sources: {theme['name']}", ""]
    lines.append("Add these URLs as sources in NotebookLM, or feed them to a research agent.")
    lines.append("")

    # Group by content type
    by_type: dict[str, list[dict]] = {}
    for u in urls:
        ct = u["content_type"]
        by_type.setdefault(ct, []).append(u)

    # Unconsumed first (highest value — user hasn't read these yet)
    unconsumed = [u for u in urls if u["consumed"] is False]
    consumed = [u for u in urls if u["consumed"] is not False]

    if unconsumed:
        lines.append(f"## Unread ({len(unconsumed)} sources)")
        lines.append("")
        lines.append("These are in the signal queue but haven't been consumed yet.")
        lines.append("High-value targets for research agents or NotebookLM podcast generation.")
        lines.append("")
        for u in unconsumed:
            lines.append(f"- [{u['title']}]({u['url']}) `{u['content_type']}`")
        lines.append("")

    if consumed:
        lines.append(f"## Already Reviewed ({len(consumed)} sources)")
        lines.append("")
        for u in consumed:
            lines.append(f"- [{u['title']}]({u['url']}) `{u['content_type']}`")
        lines.append("")

    # External recommendations (from research briefs)
    external = [u for u in urls if u["content_type"] == "external-recommendation"]
    if external:
        lines.append(f"## External Recommendations ({len(external)} sources)")
        lines.append("")
        lines.append("Suggested by research briefs — not yet in the vault.")
        lines.append("")
        for u in external:
            lines.append(f"- [{u['title']}]({u['url']})")
        lines.append("")

    return "\n".join(lines)


def build_brief_md(theme: dict, questions: list[str],
                   notes: list[tuple[Path, frontmatter.Post]],
                   urls: list[dict]) -> str:
    """Build brief.md — gap analysis + suggested angles."""
    lines = [f"# Research Brief: {theme['name']}", ""]

    # Stats
    idea_count = sum(1 for _, p in notes if "idea" in str(p.get("tags", [])))
    ref_count = len(notes) - idea_count
    url_count = len(urls)
    unconsumed = sum(1 for u in urls if u["consumed"] is False)

    lines.append("## Current State")
    lines.append("")
    lines.append(f"- **{idea_count}** original ideas")
    lines.append(f"- **{ref_count}** collected references")
    lines.append(f"- **{url_count}** source URLs ({unconsumed} unread)")
    if theme["assertion"]:
        evidence = len(theme["assertion"]["evidence_links"])
        lines.append(f"- **{evidence}** evidence links on assertion")
    lines.append("")

    # Ideas-to-references ratio
    if ref_count > 0:
        ratio = idea_count / ref_count
        lines.append("## Thinking vs. Collecting")
        lines.append("")
        if ratio < 0.15:
            lines.append(f"Ratio: {idea_count}:{ref_count} (ideas:references). "
                        "Heavy collection, light thinking. The user has been "
                        "gathering material but hasn't formed strong opinions yet. "
                        "Push for original synthesis.")
        elif ratio < 0.4:
            lines.append(f"Ratio: {idea_count}:{ref_count}. Moderate. Some original "
                        "thinking but still reference-heavy. Look for gaps where "
                        "the user has references but no take.")
        else:
            lines.append(f"Ratio: {idea_count}:{ref_count}. Strong original thinking. "
                        "Focus on deepening and challenging existing ideas rather "
                        "than adding more references.")
        lines.append("")

    # Open questions
    if questions:
        lines.append("## Open Questions")
        lines.append("")
        lines.append("These are gaps identified by the vault — questions raised by ")
        lines.append("assertions, MOCs, and research briefs that don't yet have answers.")
        lines.append("")
        for q in questions:
            lines.append(f"- {q}")
        lines.append("")

    # Gap analysis
    lines.append("## Suggested Research Angles")
    lines.append("")
    lines.append("Based on what's collected vs. what's missing:")
    lines.append("")

    if theme["assertion"]:
        lines.append("- **Assertion stress-test:** Find the strongest counter-argument "
                    "to the core assertion. What evidence would disprove it?")
    if unconsumed > 5:
        lines.append(f"- **Queue triage:** {unconsumed} unread sources exist. "
                    "Prioritize the 3-5 most likely to advance understanding.")
    if idea_count < 3:
        lines.append("- **Original synthesis needed:** Not enough original thinking. "
                    "What does the user actually believe? Where do they disagree "
                    "with the collected references?")
    lines.append("- **Cross-domain:** What adjacent fields illuminate this theme? "
                "Look for analogies, borrowed frameworks, unexpected parallels.")
    lines.append("- **People:** Who are the key thinkers? Are any missing from "
                "the vault? Who disagrees with the current framing?")
    lines.append("")

    return "\n".join(lines)


def build_agent_prompt_md(theme: dict, persona: dict, questions: list[str],
                          notes_count: int, urls_count: int) -> str:
    """Build agent-prompt.md — ready-to-use prompt for a ZeroClaw agent."""
    lines = [f"# Agent Prompt: {persona['role']}", ""]
    lines.append(f"**Theme:** {theme['name']}")
    lines.append(f"**Generated:** {datetime.now().strftime('%Y-%m-%d %H:%M')}")
    lines.append("")

    lines.append("## System")
    lines.append("")
    lines.append(f"You are a {persona['role']}. Your job: {persona['goal']}")
    lines.append("")
    lines.append("You have been given a research pack from a personal knowledge vault. "
                "The pack contains the user's existing thinking, collected references, "
                "source URLs, and identified gaps. Work from this material to produce "
                "your output. Do NOT modify the vault — your output is a standalone "
                "deliverable.")
    lines.append("")

    lines.append("## Context")
    lines.append("")
    if theme["assertion"]:
        a = theme["assertion"]
        lines.append(f"**Core assertion:** {a['title']}")
        lines.append("")
        # Extract assertion text
        match = re.search(r"## Assertion\s*\n([\s\S]*?)(?=^## |\Z)",
                         a["content"], re.MULTILINE)
        if match:
            lines.append(f"> {match.group(1).strip()}")
            lines.append("")
    lines.append(f"The user has {notes_count} notes and {urls_count} source URLs "
                "on this theme. See context.md for their thinking so far and "
                "sources.md for URLs to research.")
    lines.append("")

    if questions:
        lines.append("## Open Questions (from the vault)")
        lines.append("")
        for q in questions[:10]:
            lines.append(f"- {q}")
        lines.append("")

    lines.append("## Your Output")
    lines.append("")
    lines.append(persona["output"])
    lines.append("")

    lines.append("## Constraints")
    lines.append("")
    lines.append("- Work from the provided context + your own research")
    lines.append("- Cite sources with URLs where possible")
    lines.append("- Flag where you disagree with the user's current framing")
    lines.append("- Be specific — name papers, people, frameworks, not generalities")
    lines.append("- Output as markdown")
    lines.append("")

    return "\n".join(lines)


def export_theme(theme: dict, persona_name: str) -> dict:
    """Export a complete research pack for one theme."""
    notes = gather_vault_notes(theme)
    urls = extract_source_urls(notes)
    questions = extract_open_questions(theme, notes)
    persona = AGENT_PERSONAS.get(persona_name, AGENT_PERSONAS["researcher"])

    # Create output directory
    theme_dir = EXPORT_DIR / theme["slug"]
    theme_dir.mkdir(parents=True, exist_ok=True)

    # Write all pack files
    context = build_context_md(theme, notes)
    (theme_dir / "context.md").write_text(context, encoding="utf-8")

    sources = build_sources_md(theme, urls)
    (theme_dir / "sources.md").write_text(sources, encoding="utf-8")

    brief = build_brief_md(theme, questions, notes, urls)
    (theme_dir / "brief.md").write_text(brief, encoding="utf-8")

    prompt = build_agent_prompt_md(theme, persona, questions, len(notes), len(urls))
    (theme_dir / "agent-prompt.md").write_text(prompt, encoding="utf-8")

    stats = {
        "theme": theme["name"],
        "slug": theme["slug"],
        "type": theme["type"],
        "notes": len(notes),
        "urls": len(urls),
        "questions": len(questions),
        "persona": persona_name,
        "dir": str(theme_dir),
    }
    return stats


def main():
    parser = argparse.ArgumentParser(
        description="Export assertion-driven research packs")
    parser.add_argument("--theme", help="Filter to theme matching keyword")
    parser.add_argument("--assertion", help="Path to specific assertion file")
    parser.add_argument("--top", type=int, default=3,
                       help="Number of top themes to export (default: 3)")
    parser.add_argument("--agent", default="researcher",
                       choices=list(AGENT_PERSONAS.keys()),
                       help="Agent persona for prompts (default: researcher)")
    parser.add_argument("--all-personas", action="store_true",
                       help="Generate prompts for ALL agent personas")
    args = parser.parse_args()

    EXPORT_DIR.mkdir(parents=True, exist_ok=True)

    # Detect or filter themes
    if args.assertion:
        # Single assertion mode
        path = Path(args.assertion)
        if not path.is_absolute():
            path = VAULT / path
        post = read_note(path)
        if not post:
            print(f"  ✗ Could not read {path}")
            return
        tags = post.get("tags", []) or []
        if isinstance(tags, str):
            tags = [tags]
        themes = [{
            "name": post.get("title", path.stem),
            "slug": path.stem,
            "type": "assertion",
            "assertion": {
                "path": path,
                "title": post.get("title", path.stem),
                "tags": [t.strip().lstrip("#") for t in tags],
                "content": post.content,
                "evidence_links": extract_wikilinks(post.content),
                "status": "active",
            },
            "topics": [],
            "score": 100,
            "tags": [t.strip().lstrip("#") for t in tags if "/" in t],
        }]
    else:
        themes = detect_themes(top_n=args.top)
        if args.theme:
            themes = [t for t in themes
                     if args.theme.lower() in t["name"].lower()
                     or args.theme.lower() in t["slug"].lower()]

    if not themes:
        print("  No themes detected. Try --theme or --assertion.")
        return

    # Export each theme
    all_stats = []
    for theme in themes:
        if args.all_personas:
            for persona_name in AGENT_PERSONAS:
                stats = export_theme(theme, persona_name)
                # Write persona-specific prompt
                persona = AGENT_PERSONAS[persona_name]
                questions = extract_open_questions(theme,
                    gather_vault_notes(theme))
                prompt = build_agent_prompt_md(theme, persona, questions,
                    stats["notes"], stats["urls"])
                prompt_path = EXPORT_DIR / theme["slug"] / f"agent-prompt-{persona_name}.md"
                prompt_path.write_text(prompt, encoding="utf-8")
            stats = export_theme(theme, args.agent)  # default prompt stays
        else:
            stats = export_theme(theme, args.agent)
        all_stats.append(stats)
        print(f"  ✓ {stats['theme']}")
        print(f"    {stats['notes']} notes, {stats['urls']} URLs, "
              f"{stats['questions']} open questions")
        print(f"    → {stats['dir']}/")

    # Write manifest
    now = datetime.now().strftime("%Y-%m-%d %H:%M")
    manifest_lines = [f"# Research Packs — {now}", ""]
    manifest_lines.append("Upload context.md to NotebookLM. Add URLs from sources.md.")
    manifest_lines.append("Feed agent-prompt.md + context.md to ZeroClaw agents.")
    manifest_lines.append("")
    for s in all_stats:
        manifest_lines.append(f"## {s['theme']} ({s['type']})")
        manifest_lines.append(f"- Notes: {s['notes']}, URLs: {s['urls']}, "
                            f"Questions: {s['questions']}")
        manifest_lines.append(f"- Persona: {s['persona']}")
        manifest_lines.append(f"- Path: `{s['dir']}/`")
        manifest_lines.append("")
    (EXPORT_DIR / "MANIFEST.md").write_text("\n".join(manifest_lines), encoding="utf-8")

    print(f"\n  {len(all_stats)} research packs exported → {EXPORT_DIR}/")


if __name__ == "__main__":
    main()
