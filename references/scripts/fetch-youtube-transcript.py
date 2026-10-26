#!/usr/bin/env python3
"""
Fetch YouTube transcripts for web-clipped notes.
Scans notes for youtube.com source URLs missing a transcript,
fetches the transcript, and appends it under ## Transcript.
"""
import re
import sys
from pathlib import Path

import frontmatter
from youtube_transcript_api import YouTubeTranscriptApi

VAULT = Path(__file__).parent.parent
SEARCH_DIRS = [VAULT / "notes/fleeting", VAULT / "notes/references", VAULT / "inbox"]

YT_PATTERN = re.compile(r"(?:youtube\.com/watch\?v=|youtu\.be/)([\w-]{11})")


def extract_video_id(url: str) -> str | None:
    m = YT_PATTERN.search(url)
    return m.group(1) if m else None


def fetch_transcript(video_id: str) -> str | None:
    try:
        ytt_api = YouTubeTranscriptApi()
        transcript = ytt_api.fetch(video_id, languages=["en"])
        lines = [entry.text for entry in transcript.snippets]
        return " ".join(lines)
    except Exception as e:
        print(f"    Could not fetch transcript: {e}", file=sys.stderr)
        return None


def main():
    updated = 0
    skipped = 0
    failed = 0

    for search_dir in SEARCH_DIRS:
        if not search_dir.exists():
            continue
        for note_path in search_dir.glob("*.md"):
            post = frontmatter.load(note_path)
            source = post.get("source", "")
            if not source or "youtube.com" not in source and "youtu.be" not in source:
                continue

            content = post.content
            if "## Transcript" in content:
                skipped += 1
                continue

            video_id = extract_video_id(source)
            if not video_id:
                failed += 1
                print(f"  ✗ {note_path.stem} — no video ID found in {source}")
                continue

            print(f"  Fetching: {note_path.stem} ({video_id})...")
            transcript = fetch_transcript(video_id)
            if transcript:
                post.content = content.rstrip() + f"\n\n## Transcript\n\n{transcript}\n"
                note_path.write_text(frontmatter.dumps(post))
                updated += 1
                print(f"  ✓ {note_path.stem}")
            else:
                failed += 1

    print(f"\nDone: {updated} transcripts added, {skipped} already had transcript, {failed} failed")


if __name__ == "__main__":
    main()
