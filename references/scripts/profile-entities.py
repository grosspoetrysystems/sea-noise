#!/usr/bin/env python3
"""
Auto-generate profiles for people, companies, and concepts that appear
frequently across vault notes.

Uses claude -p (headless) — runs on your Max plan, no API key needed.

Usage:
  python3 profile-entities.py                  # auto: profile entities with 3+ mentions
  python3 profile-entities.py "Sam Kriss"      # manual: profile a specific entity
  python3 profile-entities.py --threshold 5    # raise the auto threshold
"""
import os
import re
import sys
import subprocess
import argparse
from pathlib import Path
from datetime import date
from collections import Counter

import frontmatter

VAULT = Path(__file__).parent.parent
PROFILES_DIR = VAULT / "notes/entities"
PROFILE_LOG = VAULT / "_meta/entity-profiles-log.md"
THRESHOLD = 5
MAX_PROFILES_PER_RUN = 5

EXCLUDE_DIRS = {"_rag", "_meta", "templates", ".git", ".obsidian", "_scripts", ".claude"}

# Words that appear in capitalized phrases but aren't entities worth profiling
STOPWORDS = {
    "the", "and", "for", "with", "that", "this", "from", "are", "was",
    "has", "have", "not", "but", "can", "will", "how", "what", "why",
    "who", "when", "where", "your", "you", "all", "its", "use", "new",
    "one", "two", "more", "about", "into", "just", "also", "like",
    "them", "than", "been", "some", "could", "would", "should", "their",
    "each", "which", "these", "other", "only", "over", "such", "very",
    "does", "most", "being", "between", "after", "before", "using",
    "need", "code", "data", "note", "notes", "vault", "idea", "ideas",
    "reference", "todo", "work", "personal", "raw", "tags", "source",
}



def ask_claude(prompt: str) -> str | None:
    """Run a prompt through claude -p and return the response."""
    env = os.environ.copy()
    env["CLAUDECODE"] = ""
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


def collect_vault_notes() -> list[tuple[Path, frontmatter.Post]]:
    notes = []
    for md in VAULT.rglob("*.md"):
        rel = md.relative_to(VAULT)
        if any(part in EXCLUDE_DIRS for part in rel.parts):
            continue
        try:
            post = frontmatter.load(md)
            notes.append((md, post))
        except Exception:
            continue
    return notes


def is_filtered_entity(name: str) -> bool:
    """Return True if this entity should be excluded from profiling."""
    words = name.lower().split()
    return any(w in STOPWORDS for w in words)


def strip_headings_and_structure(text: str) -> str:
    """Remove markdown headings and frontmatter-like lines from text before NER."""
    lines = []
    for line in text.split("\n"):
        stripped = line.strip()
        # Skip markdown headings, bullet list markers at start, table rows
        if stripped.startswith("#") or stripped.startswith("|"):
            continue
        lines.append(line)
    return "\n".join(lines)


def filter_candidates_with_claude(candidates: list[tuple[str, int]]) -> list[tuple[str, int]]:
    """Use a single cheap Claude call to classify which candidates are real entities."""
    names = [name for name, _ in candidates]
    prompt = (
        "You are filtering entity candidates extracted via regex NER from a knowledge vault.\n"
        "For each name below, respond with ONLY a JSON array of objects: "
        '[{"name": "...", "keep": true/false, "type": "person|company|project|concept|other"}]\n\n'
        "Keep = true ONLY for: real people, real companies/orgs, notable projects/products, "
        "or specific frameworks/methodologies worth a dedicated profile.\n"
        "Keep = false for: places, generic concepts, markdown headings, section titles, "
        "generic tech terms, or anything too vague for a useful standalone profile.\n\n"
        f"Candidates:\n" + "\n".join(f"- {n}" for n in names)
    )
    result = ask_claude(prompt)
    if not result:
        return candidates  # fallback: keep all

    import json
    try:
        # Extract JSON from response (may have markdown fences)
        json_match = re.search(r'\[.*\]', result, re.DOTALL)
        if not json_match:
            return candidates
        filtered = json.loads(json_match.group())
        keep_set = {item["name"]: item.get("type", "concept") for item in filtered if item.get("keep")}
        return [(name, count, keep_set[name]) for name, count in candidates if name in keep_set]
    except (json.JSONDecodeError, KeyError):
        return candidates  # fallback: keep all


def extract_entities_from_notes(notes: list[tuple[Path, frontmatter.Post]]) -> Counter:
    entity_counts = Counter()
    name_pattern = re.compile(r'\b([A-Z][a-z]+(?:\s+[A-Z][a-z]+){1,3})\b')

    for path, post in notes:
        # Strip enrichment sections and markdown headings before NER
        text = post.content
        rc_idx = text.find("## Research Context")
        if rc_idx > 0:
            text = text[:rc_idx]
        text = strip_headings_and_structure(text)

        for match in name_pattern.findall(text):
            if not is_filtered_entity(match):
                entity_counts[match] += 1
        title = post.get("title", "")
        if title:
            for match in name_pattern.findall(title):
                if not is_filtered_entity(match):
                    entity_counts[match] += 1

    return entity_counts


def find_notes_mentioning(entity: str, notes: list[tuple[Path, frontmatter.Post]]) -> list[dict]:
    matches = []
    pattern = re.compile(re.escape(entity), re.IGNORECASE)
    for path, post in notes:
        text = post.content + " " + post.get("title", "")
        if pattern.search(text):
            sentences = re.split(r'[.!?\n]', post.content)
            context_sentences = [s.strip() for s in sentences if pattern.search(s)]
            matches.append({
                "path": str(path.relative_to(VAULT)),
                "stem": path.stem,
                "title": post.get("title", path.stem),
                "tags": post.get("tags", []),
                "context": context_sentences[:3],
            })
    return matches


def get_profiled_entities() -> set[str]:
    if not PROFILE_LOG.exists():
        return set()
    text = PROFILE_LOG.read_text()
    return set(re.findall(r"^- (.+?) —", text, re.MULTILINE))


def generate_profile(entity: str, mentions: list[dict]) -> str | None:
    mentions_text = "\n".join(
        f"- **{m['title']}** ({m['path']})\n  Context: {' | '.join(m['context'][:2])}"
        for m in mentions
    )

    prompt = (
        f"Generate a profile note for '{entity}' based on mentions in a personal knowledge vault.\n\n"
        f"## Vault Mentions ({len(mentions)} notes)\n{mentions_text}\n\n"
        f"Generate the following sections in markdown. Output ONLY the markdown, no preamble. "
        f"Be factual — only state things you can verify or that are evident from the vault context. "
        f"Say 'unclear from vault context' if you're not sure about something.\n\n"
        f"### Overview\n"
        f"2-3 sentences on who/what this is. Role, affiliation, domain.\n\n"
        f"### Key Ideas & Contributions\n"
        f"Bullet points of their key ideas, works, or contributions as referenced in the vault.\n\n"
        f"### Related Entities\n"
        f"People, companies, or concepts they're connected to (from vault context).\n\n"
        f"### External Links\n"
        f"Known URLs: personal site, Twitter/X, Wikipedia, company page. "
        f"Only include links you're confident are real.\n\n"
        f"### Vault Presence\n"
        f"Brief stats: first seen, mention count, which topic clusters they appear in."
    )
    return ask_claude(prompt)


def write_profile(entity: str, mentions: list[dict], profile_content: str,
                   entity_type: str = "concept") -> Path:
    today = date.today().isoformat()
    slug = re.sub(r'[^a-z0-9]+', '-', entity.lower()).strip('-')
    filename = f"{today}-profile-{slug}.md"
    out_path = PROFILES_DIR / filename

    backlinks = "\n".join(f"- [[{m['stem']}]] — {m['title']}" for m in mentions)

    content = (
        f"---\n"
        f"title: \"{entity}\"\n"
        f"tags:\n  - entity\n  - entity/{entity_type}\n  - entity-profile\n"
        f"date: {today}\n"
        f"source: profile-entities\n"
        f"entity-type: {entity_type}\n"
        f"mention-count: {len(mentions)}\n"
        f"---\n\n"
        f"# {entity}\n\n"
        f"{profile_content}\n\n"
        f"## Vault Backlinks\n{backlinks}\n"
    )

    PROFILES_DIR.mkdir(parents=True, exist_ok=True)
    out_path.write_text(content)
    print(f"  ✓ Profile written: {out_path.relative_to(VAULT)}")
    return out_path


def log_profile(entity: str, out_path: Path):
    today = date.today().isoformat()
    PROFILE_LOG.parent.mkdir(parents=True, exist_ok=True)
    line = f"- {entity} — {today} — {out_path.relative_to(VAULT)}\n"
    if PROFILE_LOG.exists():
        text = PROFILE_LOG.read_text()
    else:
        text = "# Entity Profiles Log\n\n"
    PROFILE_LOG.write_text(text + line)


def profile_entity(entity: str, notes: list[tuple[Path, frontmatter.Post]],
                    entity_type: str = "concept"):
    print(f"\n▸ Profiling: {entity} ({entity_type})")

    mentions = find_notes_mentioning(entity, notes)
    if not mentions:
        print(f"  No mentions found for '{entity}'")
        return

    print(f"  Found {len(mentions)} mentions")
    profile = generate_profile(entity, mentions)
    if not profile:
        return

    out_path = write_profile(entity, mentions, profile, entity_type=entity_type)
    log_profile(entity, out_path)


def auto_mode(threshold: int, dry_run: bool = False):
    notes = collect_vault_notes()
    entities = extract_entities_from_notes(notes)
    profiled = get_profiled_entities()

    candidates = [
        (name, count) for name, count in entities.most_common(50)
        if count >= threshold and name not in profiled
    ]

    if not candidates:
        print("No entities above threshold needing profiles.")
        return

    print(f"Found {len(candidates)} raw candidates above threshold:")
    for name, count in candidates:
        print(f"  {name}: {count} mentions")

    # Pre-filter with a single Claude call to drop false positives
    print(f"\nFiltering with Claude...")
    candidates = filter_candidates_with_claude(candidates)

    if not candidates:
        print("No entities passed the filter.")
        return

    # Cap per run to control costs
    if len(candidates) > MAX_PROFILES_PER_RUN:
        print(f"Capping at {MAX_PROFILES_PER_RUN} profiles (of {len(candidates)} candidates)")
        candidates = candidates[:MAX_PROFILES_PER_RUN]

    print(f"\n{len(candidates)} entities to profile:")
    for name, count, etype in candidates:
        print(f"  ✓ {name}: {count} mentions ({etype})")

    if dry_run:
        print("\n(dry run — no profiles generated)")
        return

    for name, count, etype in candidates:
        profile_entity(name, notes, entity_type=etype)


def main():
    parser = argparse.ArgumentParser(description="Auto-generate entity profiles")
    parser.add_argument("entity", nargs="?", help="Specific entity to profile")
    parser.add_argument("--threshold", type=int, default=THRESHOLD,
                        help=f"Min mentions for auto-profiling (default: {THRESHOLD})")
    parser.add_argument("--dry-run", action="store_true",
                        help="Show candidates without generating profiles")
    args = parser.parse_args()

    if args.entity:
        notes = collect_vault_notes()
        profile_entity(args.entity, notes)
    else:
        auto_mode(args.threshold, dry_run=args.dry_run)


if __name__ == "__main__":
    main()
