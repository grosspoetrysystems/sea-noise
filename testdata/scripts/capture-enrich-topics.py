#!/usr/bin/env python3
"""Capture LLM envelopes for the enrich-topics parity test.

Forks the W2 capture-llm-envelopes.py for the enrich-topics workflow.
Single-phase: one generate-research-brief LLM call per qualifying
topic. Capture flow:

  1. Build sea-noise binary.
  2. Ask parity-discover --skill enrich-topics --vault fixture-vault
     for the Go-side prompts (one per topic above threshold). Uses the
     StubCrawler reading from testdata/crawl-stubs/enrich-topics/ for
     deterministic web-section content (currently empty for the
     single-topic fixture: no matched note has a source URL).
  3. Copy fixture vault to tmp + drop prototype script alongside (so
     its `VAULT = __file__.parent.parent` resolves to the temp vault).
  4. Run prototype with the PATH-shim ahead of the real claude binary.
     The prototype hits live Cloudflare if env vars are set, or the
     NoopCrawler fallback otherwise. Either is fine — we do NOT try
     to force prototype crawl content to match the Go test's stub
     (kickoff Decision 4: parity is feature-equivalence, not
     LLM-input fidelity, for enrichment workflows).
  5. Pair each prototype envelope with its Go counterpart by topic
     name extracted from the prompt (regex on "topic '<name>' has
     reached critical mass"). Re-key under Go prompt hash; write to
     testdata/envelopes/enrich-topics/<hash>.json.
  6. Snapshot post-Python vault to
     testdata/expected/enrich-topics/post-state/.

Re-run when the fixture vault, the prototype, or the
generate-research-brief skill template changes. Burns one real
claude -p call per qualifying topic.

Environment:
  REAL_CLAUDE   path to real claude binary (default: shutil.which("claude"))
  CF_ACCOUNT_ID, CF_API_TOKEN — optional; if set, prototype hits live
                 Cloudflare for web-research content. Otherwise it
                 logs the missing-creds warning and proceeds without.
"""
from __future__ import annotations

import hashlib
import json
import os
import re
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

BRIEF_SKILL = REPO_ROOT / ".agents" / "shared" / "skills" / "inference" / "generate-research-brief.md"
SKILLS_DIR = BRIEF_SKILL.parent

SEA_NOISE_BIN = REPO_ROOT / ".tempor" / "spike" / "bin" / "sea-noise"
VENV_PYTHON = REPO_ROOT / ".venv" / "bin" / "python"

ENVELOPE_OUT = REPO_ROOT / "testdata" / "envelopes" / "enrich-topics"
EXPECTED_OUT = REPO_ROOT / "testdata" / "expected" / "enrich-topics" / "post-state"

TOPIC_MARKER_RE = re.compile(r"topic '([^']+)' has reached critical mass")


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


def discover_go_prompts() -> list[dict]:
    result = run(
        [
            str(SEA_NOISE_BIN),
            "parity-discover",
            "--skill", "enrich-topics",
            "--vault", str(FIXTURE_VAULT),
            "--skills-dir", str(SKILLS_DIR),
        ],
        capture_output=True,
        text=True,
    )
    return json.loads(result.stdout)


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
    env["SEA_NOISE_CAPTURE_SKILL"] = "generate-research-brief"
    env["SEA_NOISE_CAPTURE_SKILL_PATH"] = str(BRIEF_SKILL)
    env["SEA_NOISE_REAL_CLAUDE"] = real_claude
    env["CLAUDECODE"] = ""

    python = str(VENV_PYTHON) if VENV_PYTHON.exists() else "python3"
    run([python, str(vault / "scripts" / "enrich-topics.py")], env=env, cwd=vault)


def load_raw_envelopes(capture_dir: Path) -> list[dict]:
    return [json.loads(p.read_text()) for p in sorted(capture_dir.glob("*.json"))]


def topic_from_prompt(prompt: str) -> str | None:
    m = TOPIC_MARKER_RE.search(prompt)
    return m.group(1) if m else None


def write_envelope(go_prompt: str, response: str) -> str:
    envelope = {
        "skill_name": "generate-research-brief",
        "skill_sha": file_sha(BRIEF_SKILL),
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

    print("==> discovering Go-side prompts")
    go_candidates = discover_go_prompts()
    print(f"    {len(go_candidates)} candidate(s) from parity-discover")
    if not go_candidates:
        print("error: no Go candidates; topic-frequencies.md may lack a topic above threshold", file=sys.stderr)
        return 1

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

        # Index prototype envelopes by extracted topic name.
        proto_by_topic: dict[str, dict] = {}
        for env in raw_envs:
            t = topic_from_prompt(env["prompt"])
            if not t:
                print(f"  WARN: could not extract topic from envelope {env.get('prompt_hash')}", file=sys.stderr)
                continue
            proto_by_topic[t] = env

        # Pair + write per Go candidate.
        written = 0
        for cand in go_candidates:
            topic = cand["rel"]
            proto = proto_by_topic.get(topic)
            if proto is None:
                print(f"  WARN: no prototype envelope for topic {topic!r}", file=sys.stderr)
                continue
            name = write_envelope(cand["prompt"], proto["response"])
            print(f"  wrote envelope: {topic} → {name}")
            written += 1

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
