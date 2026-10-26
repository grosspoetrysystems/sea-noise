#!/usr/bin/env python3
"""Capture LLM envelope for the enrich-topics --note (per-note) parity test.

Single LLM call against a synthetic fixture note. The prototype runs in a
temporary copy, never against a live vault.
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
PROTOTYPE_SCRIPT = REPO_ROOT / "references" / "scripts" / "enrich-topics.py"
SHIM = REPO_ROOT / "testdata" / "scripts" / "claude-shim.py"

NOTE_SKILL = REPO_ROOT / ".agents" / "shared" / "skills" / "inference" / "enrich-single-note.md"
SKILLS_DIR = NOTE_SKILL.parent

SEA_NOISE_BIN = REPO_ROOT / ".tempor" / "spike" / "bin" / "sea-noise"
VENV_PYTHON = REPO_ROOT / ".venv" / "bin" / "python"

ENVELOPE_OUT = REPO_ROOT / "testdata" / "envelopes" / "enrich-note"
EXPECTED_OUT = REPO_ROOT / "testdata" / "expected" / "enrich-note" / "post-state"

TARGET_NOTE_REL = "notes/references/2026-03-09-roasted-vegetables.md"


def hash_prompt(prompt: str) -> str:
    return hashlib.sha256(prompt.encode("utf-8")).hexdigest()[:16]


def file_sha(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def run(cmd: list[str], **kw) -> subprocess.CompletedProcess:
    print(f"  $ {' '.join(str(c) for c in cmd)}")
    return subprocess.run(cmd, check=True, **kw)


def build_binary() -> None:
    SEA_NOISE_BIN.parent.mkdir(parents=True, exist_ok=True)
    run(["go", "build", "-o", str(SEA_NOISE_BIN), "./cmd/sea-noise/"], cwd=REPO_ROOT)


def discover_go_prompt() -> str:
    result = run(
        [
            str(SEA_NOISE_BIN),
            "parity-discover",
            "--skill", "enrich-note",
            "--vault", str(FIXTURE_VAULT),
            "--note", TARGET_NOTE_REL,
            "--skills-dir", str(SKILLS_DIR),
        ],
        capture_output=True,
        text=True,
    )
    arr = json.loads(result.stdout)
    if len(arr) != 1:
        raise RuntimeError(f"expected 1 prompt, got {arr!r}")
    return arr[0]["prompt"]


def setup_temp_vault(tmp: Path) -> Path:
    vault = tmp / "parity-capture"
    shutil.copytree(FIXTURE_VAULT, vault)
    scripts_dir = vault / "scripts"
    scripts_dir.mkdir(exist_ok=True)
    shutil.copy(PROTOTYPE_SCRIPT, scripts_dir / "enrich-topics.py")
    return vault


def run_prototype(vault: Path, shim_bin_dir: Path, capture_dir: Path, real_claude: str) -> None:
    env = os.environ.copy()
    env["PATH"] = f"{shim_bin_dir}:{env['PATH']}"
    env["SEA_NOISE_CAPTURE_DIR"] = str(capture_dir)
    env["SEA_NOISE_CAPTURE_SKILL"] = "enrich-single-note"
    env["SEA_NOISE_CAPTURE_SKILL_PATH"] = str(NOTE_SKILL)
    env["SEA_NOISE_REAL_CLAUDE"] = real_claude
    env["CLAUDECODE"] = ""

    python = str(VENV_PYTHON) if VENV_PYTHON.exists() else "python3"
    run(
        [python, str(vault / "scripts" / "enrich-topics.py"), "--note", TARGET_NOTE_REL],
        env=env,
        cwd=vault,
    )


def load_raw_envelopes(capture_dir: Path) -> list[dict]:
    return [json.loads(p.read_text()) for p in sorted(capture_dir.glob("*.json"))]


def write_envelope(go_prompt: str, response: str) -> str:
    envelope = {
        "skill_name": "enrich-single-note",
        "skill_sha": file_sha(NOTE_SKILL),
        "prompt": go_prompt,
        "response": response,
        "captured_at": datetime.now(timezone.utc).isoformat(),
        "prompt_hash": hash_prompt(go_prompt),
    }
    out = ENVELOPE_OUT / f"{envelope['prompt_hash']}.json"
    out.write_text(json.dumps(envelope, indent=2, sort_keys=True) + "\n")
    return out.name


def snapshot_post_state(vault: Path) -> None:
    if EXPECTED_OUT.exists():
        shutil.rmtree(EXPECTED_OUT)
    EXPECTED_OUT.parent.mkdir(parents=True, exist_ok=True)
    shutil.copytree(vault, EXPECTED_OUT, ignore=shutil.ignore_patterns("scripts"))


def main() -> int:
    real_claude = os.environ.get("REAL_CLAUDE") or shutil.which("claude")
    if not real_claude:
        print("error: no claude binary on PATH; set REAL_CLAUDE", file=sys.stderr)
        return 1

    print("==> building sea-noise binary")
    build_binary()

    print("==> discovering Go-side prompt")
    go_prompt = discover_go_prompt()

    ENVELOPE_OUT.mkdir(parents=True, exist_ok=True)
    for old in ENVELOPE_OUT.glob("*.json"):
        old.unlink()

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

        raw_envs = load_raw_envelopes(capture_dir)
        print(f"    captured {len(raw_envs)} raw envelope(s) from prototype")
        if len(raw_envs) != 1:
            print(f"error: expected 1 envelope, got {len(raw_envs)}", file=sys.stderr)
            return 1

        name = write_envelope(go_prompt, raw_envs[0]["response"])
        print(f"  wrote envelope: {TARGET_NOTE_REL} → {name}")

        print("==> snapshotting post-state vault")
        snapshot_post_state(vault)

    print(f"==> done. 1 envelope at {ENVELOPE_OUT}")
    print(f"==> post-state snapshot at {EXPECTED_OUT}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
