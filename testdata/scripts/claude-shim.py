#!/usr/bin/env python3
"""PATH-first shim for `claude -p` that captures (prompt, response) envelopes.

Intercepts the prototype's `subprocess.run(["claude", "-p", <prompt>, "--dangerously-skip-permissions"], ...)`
call by being earlier on PATH than the real claude binary. Records the
envelope to $SEA_NOISE_CAPTURE_DIR/<prompt-hash>.json, then delegates to
the real claude binary (or returns a stub if SEA_NOISE_CAPTURE_STUB is set).

Envelope format matches internal/parity.Envelope:
    {skill_name, skill_sha, prompt, response, captured_at, prompt_hash}

Prompt hash = sha256(prompt) hex, truncated to 16 chars — matches
internal/parity.HashPrompt.

Env vars:
    SEA_NOISE_CAPTURE_DIR   (required) — where to write envelope JSON
    SEA_NOISE_CAPTURE_SKILL (required) — skill_name to record
    SEA_NOISE_CAPTURE_SKILL_PATH (required) — path to skill file for SHA
    SEA_NOISE_REAL_CLAUDE   (required) — absolute path to real claude binary

This is the SPIKE artifact — proves the wrapping mechanism. Production
version moves to testdata/scripts/ in T-PROVE-CORPUS.
"""
from __future__ import annotations

import hashlib
import json
import os
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path


def hash_prompt(prompt: str) -> str:
    return hashlib.sha256(prompt.encode("utf-8")).hexdigest()[:16]


def skill_sha(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main(argv: list[str]) -> int:
    if len(argv) < 3 or argv[1] != "-p":
        print(f"claude-shim: unexpected argv: {argv!r}", file=sys.stderr)
        return 2

    prompt = argv[2]
    rest = argv[3:]

    capture_dir = Path(os.environ["SEA_NOISE_CAPTURE_DIR"])
    skill_name = os.environ["SEA_NOISE_CAPTURE_SKILL"]
    skill_path = Path(os.environ["SEA_NOISE_CAPTURE_SKILL_PATH"])
    real_claude = os.environ["SEA_NOISE_REAL_CLAUDE"]

    capture_dir.mkdir(parents=True, exist_ok=True)

    # Delegate to real claude with the same argv tail.
    result = subprocess.run(
        [real_claude, "-p", prompt, *rest],
        capture_output=True,
        text=True,
        env={**os.environ, "CLAUDECODE": ""},
    )
    response = result.stdout

    envelope = {
        "skill_name": skill_name,
        "skill_sha": skill_sha(skill_path),
        "prompt": prompt,
        "response": response,
        "captured_at": datetime.now(timezone.utc).isoformat(),
        "prompt_hash": hash_prompt(prompt),
    }
    out_path = capture_dir / f"{envelope['prompt_hash']}.json"
    out_path.write_text(json.dumps(envelope, indent=2, sort_keys=True))

    # Pass through to the prototype.
    sys.stdout.write(response)
    if result.stderr:
        sys.stderr.write(result.stderr)
    return result.returncode


if __name__ == "__main__":
    sys.exit(main(sys.argv))
