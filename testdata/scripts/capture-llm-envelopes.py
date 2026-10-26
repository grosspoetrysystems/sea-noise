#!/usr/bin/env python3
"""Capture LLM envelopes for the summarize-transcripts parity test.

Implements path (A) of the T-PROVE-CAPTURE spike (see
`.tempor/spike/T-PROVE-CAPTURE-feasibility.md`):

  1. Build the sea-noise binary.
  2. Ask `sea-noise parity-discover` for the Go-side rendered prompt
     for every transcript-bearing fixture note.
  3. Copy the fixture vault to a temp dir; drop the prototype script
     in alongside it (so the prototype's hard-coded
     `VAULT = Path(__file__).parent.parent` resolves to the temp vault).
  4. Run the prototype with a PATH-shim ahead of the real `claude`
     binary. The shim writes one envelope per LLM call.
  5. For each Go candidate, find the matching Python envelope by
     transcript content overlap, then re-emit under the Go-prompt
     hash at `testdata/envelopes/summarize-transcripts/<hash>.json`.
  6. Snapshot the post-Python vault to
     `testdata/expected/summarize-transcripts/post-state/` for the
     parity test's `Diff(expected, got)` call.

Re-run any time the fixture vault, the prototype, or the Go skill
template changes. Burns real `claude -p` tokens — keep the fixture
small.

Environment:
  REAL_CLAUDE   path to real claude binary (default: shutil.which("claude"))
"""
from __future__ import annotations

import hashlib
import json
import os
import shutil
import subprocess
import sys
import tempfile
from datetime import datetime, timezone
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent.parent
FIXTURE_VAULT = REPO_ROOT / "testdata" / "fixture-vault"
PROTOTYPE_SCRIPT = REPO_ROOT / "references" / "scripts" / "summarize-transcripts.py"
SHIM = REPO_ROOT / "testdata" / "scripts" / "claude-shim.py"
SKILL_FILE = REPO_ROOT / ".agents" / "shared" / "skills" / "inference" / "summarize-content.md"
SKILLS_DIR = SKILL_FILE.parent
SEA_NOISE_BIN = REPO_ROOT / ".tempor" / "spike" / "bin" / "sea-noise"
VENV_PYTHON = REPO_ROOT / ".venv" / "bin" / "python"

ENVELOPE_OUT = REPO_ROOT / "testdata" / "envelopes" / "summarize-transcripts"
EXPECTED_OUT = REPO_ROOT / "testdata" / "expected" / "summarize-transcripts" / "post-state"


def hash_prompt(prompt: str) -> str:
    return hashlib.sha256(prompt.encode("utf-8")).hexdigest()[:16]


def run(cmd: list[str], **kw) -> subprocess.CompletedProcess:
    print(f"  $ {' '.join(str(c) for c in cmd)}")
    return subprocess.run(cmd, check=True, **kw)


def build_binary() -> None:
    SEA_NOISE_BIN.parent.mkdir(parents=True, exist_ok=True)
    run(["go", "build", "-o", str(SEA_NOISE_BIN), "./cmd/sea-noise/"], cwd=REPO_ROOT)


def discover_go_prompts() -> list[dict]:
    result = run(
        [
            str(SEA_NOISE_BIN),
            "parity-discover",
            "--skill", "summarize-content",
            "--vault", str(FIXTURE_VAULT),
            "--skills-dir", str(SKILLS_DIR),
        ],
        capture_output=True,
        text=True,
    )
    return json.loads(result.stdout)


def setup_temp_vault(tmp: Path) -> Path:
    """Copy the fixture vault and drop the prototype script alongside.

    Returns the vault root (which doubles as VAULT in the prototype, since
    the prototype computes VAULT = Path(__file__).parent.parent).
    """
    vault = tmp / "parity-capture"
    shutil.copytree(FIXTURE_VAULT, vault)
    scripts_dir = vault / "scripts"
    scripts_dir.mkdir(exist_ok=True)
    shutil.copy(PROTOTYPE_SCRIPT, scripts_dir / "summarize-transcripts.py")
    return vault


def run_prototype(vault: Path, shim_bin_dir: Path, capture_dir: Path, real_claude: str) -> None:
    env = os.environ.copy()
    env["PATH"] = f"{shim_bin_dir}:{env['PATH']}"
    env["SEA_NOISE_CAPTURE_DIR"] = str(capture_dir)
    env["SEA_NOISE_CAPTURE_SKILL"] = "summarize-content"
    env["SEA_NOISE_CAPTURE_SKILL_PATH"] = str(SKILL_FILE)
    env["SEA_NOISE_REAL_CLAUDE"] = real_claude
    env["CLAUDECODE"] = ""

    python = str(VENV_PYTHON) if VENV_PYTHON.exists() else "python3"
    run([python, str(vault / "scripts" / "summarize-transcripts.py")], env=env, cwd=vault)


def load_python_envelopes(capture_dir: Path) -> list[dict]:
    envs = []
    for path in sorted(capture_dir.glob("*.json")):
        envs.append(json.loads(path.read_text()))
    return envs


def match_envelope(go_prompt: str, python_envs: list[dict]) -> dict | None:
    """Find the python envelope whose prompt corresponds to the same note.

    Heuristic: extract a substantial substring of the transcript content
    from the Go prompt (the part following "Content:\\n") and look for
    the same substring in each Python envelope's prompt.
    """
    marker = "\nContent:\n"
    idx = go_prompt.find(marker)
    if idx < 0:
        return None
    content_start = go_prompt[idx + len(marker):]
    # Use the first 200 chars of content as the matching key — long
    # enough to be unique across notes, short enough to survive any
    # trailing whitespace differences.
    key = content_start[:200]
    if not key.strip():
        return None
    matches = [e for e in python_envs if key in e.get("prompt", "")]
    if len(matches) != 1:
        return None
    return matches[0]


def write_final_envelopes(go_candidates: list[dict], python_envs: list[dict]) -> int:
    ENVELOPE_OUT.mkdir(parents=True, exist_ok=True)
    # Wipe old envelopes so removed candidates don't linger.
    for old in ENVELOPE_OUT.glob("*.json"):
        old.unlink()

    skill_sha = hashlib.sha256(SKILL_FILE.read_bytes()).hexdigest()
    written = 0
    for cand in go_candidates:
        go_prompt = cand["prompt"]
        py_env = match_envelope(go_prompt, python_envs)
        if py_env is None:
            print(f"  WARN: no python envelope matched {cand['rel']}", file=sys.stderr)
            continue
        envelope = {
            "skill_name": "summarize-content",
            "skill_sha": skill_sha,
            "prompt": go_prompt,
            "response": py_env["response"],
            "captured_at": datetime.now(timezone.utc).isoformat(),
            "prompt_hash": hash_prompt(go_prompt),
        }
        out = ENVELOPE_OUT / f"{envelope['prompt_hash']}.json"
        out.write_text(json.dumps(envelope, indent=2, sort_keys=True) + "\n")
        print(f"  wrote envelope: {cand['rel']} → {out.name}")
        written += 1
    return written


def snapshot_post_state(vault: Path) -> None:
    if EXPECTED_OUT.exists():
        shutil.rmtree(EXPECTED_OUT)
    EXPECTED_OUT.parent.mkdir(parents=True, exist_ok=True)
    # Snapshot everything EXCEPT the scripts/ dir we dropped in (it's
    # not part of the vault under test).
    shutil.copytree(vault, EXPECTED_OUT, ignore=shutil.ignore_patterns("scripts"))


def main() -> int:
    real_claude = os.environ.get("REAL_CLAUDE") or shutil.which("claude")
    if not real_claude:
        print("error: no claude binary on PATH; set REAL_CLAUDE", file=sys.stderr)
        return 1

    print("==> building sea-noise binary")
    build_binary()

    print("==> discovering Go-side prompts")
    go_candidates = discover_go_prompts()
    print(f"    {len(go_candidates)} candidate(s) from parity-discover")
    if not go_candidates:
        print("error: no Go candidates; the fixture has no qualifying transcript notes", file=sys.stderr)
        return 1

    with tempfile.TemporaryDirectory(prefix="sea-noise-capture-") as tmp:
        tmp_path = Path(tmp)
        shim_bin_dir = tmp_path / "shim-bin"
        shim_bin_dir.mkdir()
        shim_target = shim_bin_dir / "claude"
        shutil.copy(SHIM, shim_target)
        shim_target.chmod(0o755)

        capture_dir = tmp_path / "shim-out"
        capture_dir.mkdir()

        print("==> copying fixture vault to temp dir")
        vault = setup_temp_vault(tmp_path)

        print("==> running prototype against temp vault (shim active)")
        run_prototype(vault, shim_bin_dir, capture_dir, real_claude)

        python_envs = load_python_envelopes(capture_dir)
        print(f"    captured {len(python_envs)} python envelope(s)")

        print("==> matching python envelopes to Go candidates")
        written = write_final_envelopes(go_candidates, python_envs)
        if written != len(go_candidates):
            print(f"error: wrote {written} of {len(go_candidates)} expected envelopes", file=sys.stderr)
            return 1

        print("==> snapshotting post-state vault")
        snapshot_post_state(vault)

    print(f"==> done. {written} envelope(s) at {ENVELOPE_OUT}")
    print(f"==> post-state snapshot at {EXPECTED_OUT}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
