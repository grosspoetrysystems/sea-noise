#!/usr/bin/env python3
"""
Enrich vault notes with research context using Claude Code (claude -p).

Handles two types of content:
  1. YouTube transcripts (## Transcript section)
  2. Web clips with substantial body content (≥500 chars, has source URL)

Adds a ## Research Context section with key takeaways, follow-up questions,
related materials, and book/podcast recommendations.

Skips short notes, memes, and notes already enriched.
Uses claude -p (headless) so it runs on your Max plan — no API key needed.
"""
import os
import subprocess
import sys
import tempfile

import frontmatter
from pathlib import Path

VAULT = Path(__file__).parent.parent
SEARCH_DIRS = [VAULT / "notes/fleeting", VAULT / "notes/references", VAULT / "inbox"]
MAX_CONTENT_CHARS = 80_000
MIN_CONTENT_CHARS = 500  # skip memes, short clips, oddities


def ask_claude(prompt: str) -> str | None:
    """Run a prompt through claude -p and return the response."""
    env = os.environ.copy()
    env["CLAUDECODE"] = ""  # allow nested invocation
    try:
        result = subprocess.run(
            ["claude", "-p", prompt, "--dangerously-skip-permissions"],
            capture_output=True, text=True, timeout=300, env=env,
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


def enrich(content_text: str, title: str = "", content_type: str = "article") -> str | None:
    trimmed = content_text[:MAX_CONTENT_CHARS]
    context = f" about '{title}'" if title else ""

    if content_type == "transcript":
        source_desc = "a YouTube video transcript"
    else:
        source_desc = "a web article / clipping"

    prompt = (
        f"You are summarizing {source_desc}{context} for a personal knowledge base.\n\n"
        "Generate the following sections in markdown. Output ONLY the markdown, no preamble.\n\n"
        "### Key Takeaways\n"
        "5-8 concise bullet points. Focus on actionable insights, novel ideas, "
        "and memorable quotes. No filler or generic observations.\n\n"
        "### Follow-up Questions\n"
        "3-5 genuinely thought-provoking questions this content raises.\n\n"
        "### Related Materials\n"
        "3-5 specific articles, papers, talks, tools, or people mentioned or "
        "directly relevant. Include URLs where possible. Do NOT make up titles.\n\n"
        "### Book & Podcast Recommendations\n"
        "2-3 books and 2-3 podcasts/episodes relevant to the topics discussed. "
        "Include author/host. 1 sentence on why each is relevant. "
        "Only recommend real, verifiable titles.\n\n"
        f"Content:\n{trimmed}"
    )
    return ask_claude(prompt)


def is_already_enriched(content: str) -> bool:
    return "## Research Context" in content or "## Key Takeaways" in content


# --- Tier 2 domain tags (from Tag Taxonomy) ---
TIER2_TAXONOMY = """
- tech/ai, tech/ai-agents, tech/context-engineering, tech/dev-tools,
  tech/rag, tech/knowledge-graphs, tech/software-design, tech/software-engineering
- life/cooking, life/health, life/fitness, life/sports,
  life/music, life/personal-growth
- culture/criticism, culture/rationalism, culture/tech
- science (expand as needed)
- learning (expand as needed)
- business/strategy, business/ops
"""


def extract_domain_tags(content: str, existing_tags: list) -> list[str]:
    """Extract Tier 2 domain tags from note content via Claude."""
    prompt = (
        "Extract 2-5 domain tags for this note. Use slash-separated nested tags.\n"
        f"Known taxonomy:\n{TIER2_TAXONOMY}\n"
        "You may propose new subtags under existing domains if nothing fits.\n"
        "Return ONLY a JSON array of tag strings, e.g. [\"tech/ai\", \"tech/rag\"]\n"
        "No preamble, no commentary, just the JSON array.\n\n"
        f"Existing tags: {existing_tags}\n\n"
        f"Content:\n{content[:5000]}"
    )
    result = ask_claude(prompt)
    if not result:
        return []
    import json
    try:
        match = __import__("re").search(r'\[.*\]', result, __import__("re").DOTALL)
        if match:
            tags = json.loads(match.group())
            return [t for t in tags if isinstance(t, str) and "/" in t]
    except (json.JSONDecodeError, TypeError):
        pass
    return []


def enrich_tags(note_path: Path, post: frontmatter.Post):
    """Add Tier 2 domain tags to a note's frontmatter."""
    existing = post.get("tags", [])
    if isinstance(existing, str):
        existing = [existing]

    # Skip if already has Tier 2 tags (contain slash)
    if any("/" in t for t in existing if t):
        return

    print(f"  Extracting domain tags: {note_path.stem}...")
    new_tags = extract_domain_tags(post.content, existing)
    if new_tags:
        merged = list(dict.fromkeys(existing + new_tags))
        post["tags"] = merged
        note_path.write_text(frontmatter.dumps(post))
        print(f"  ✓ tags: {new_tags}")


def process_transcript_note(note_path: Path, post: frontmatter.Post) -> bool:
    """Enrich a note that has a ## Transcript section."""
    content = post.content
    if "## Transcript" not in content:
        return False
    if is_already_enriched(content):
        return False

    idx = content.index("## Transcript")
    transcript = content[idx + len("## Transcript"):].strip()
    if len(transcript) < MIN_CONTENT_CHARS:
        return False

    title = post.get("title", note_path.stem)
    print(f"  Summarizing transcript: {note_path.stem}...")
    summary = enrich(transcript, title=title, content_type="transcript")
    if summary:
        before = content[:idx].rstrip()
        after = content[idx:]
        post.content = f"{before}\n\n## Research Context\n\n{summary}\n\n{after}\n"
        note_path.write_text(frontmatter.dumps(post))
        enrich_tags(note_path, post)
        print(f"  ✓ {note_path.stem}")
        return True
    print(f"  ✗ {note_path.stem} — enrichment failed")
    return False


def process_webclip_note(note_path: Path, post: frontmatter.Post) -> bool:
    """Enrich a web-clipped note with substantial content."""
    content = post.content
    if is_already_enriched(content):
        return False

    source = str(post.get("source", ""))
    tags = post.get("tags", [])
    if isinstance(tags, str):
        tags = [tags]
    is_webclip = (
        source.startswith("http")
        or "web-clipper" in source
        or "clippings" in tags
        or "#clippings" in tags
    )
    if not is_webclip:
        return False

    if "## Transcript" in content:
        return False

    body = content.strip()
    raw_idx = body.find("## Raw")
    meaningful = body[:raw_idx].strip() if raw_idx > 0 else body
    if len(meaningful) < MIN_CONTENT_CHARS:
        return False

    title = post.get("title", note_path.stem)
    print(f"  Summarizing web clip: {note_path.stem}...")
    summary = enrich(meaningful, title=title, content_type="article")
    if summary:
        post.content = content.rstrip() + f"\n\n## Research Context\n\n{summary}\n"
        note_path.write_text(frontmatter.dumps(post))
        enrich_tags(note_path, post)
        print(f"  ✓ {note_path.stem}")
        return True
    print(f"  ✗ {note_path.stem} — enrichment failed")
    return False


def main():
    updated = 0
    skipped = 0

    for search_dir in SEARCH_DIRS:
        if not search_dir.exists():
            continue
        for note_path in search_dir.glob("*.md"):
            post = frontmatter.load(note_path)

            if process_transcript_note(note_path, post):
                updated += 1
            elif process_webclip_note(note_path, post):
                updated += 1
            else:
                skipped += 1

    print(f"\nDone: {updated} enriched, {skipped} skipped")


if __name__ == "__main__":
    main()
