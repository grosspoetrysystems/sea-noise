#!/usr/bin/env python3
"""Capture LLM envelopes for the profile-entities parity test.

Forks the W2 capture-llm-envelopes.py for the two-phase profile-entities
workflow. The prototype's auto-mode fires:
  1. ONE filter-entity-candidates LLM call with all ranked candidates.
  2. UP TO MAX_PROFILES_PER_RUN generate-entity-profile LLM calls — one
     per entity the filter response classifies as keep, in ranked order.

Capture flow:
  1. Build sea-noise binary.
  2. Ask parity-discover --skill profile-entities --mode filter for the
     Go-side filter prompt.
  3. Copy fixture vault to tmp; drop prototype script alongside (so its
     `VAULT = __file__.parent.parent` resolves to the temp vault).
  4. Run prototype with a PATH-shim ahead of `claude`. The shim writes
     one envelope per LLM call to a temp capture dir; envelope skill_name
     and skill_sha get fixed up post-pairing (the shim can't know which
     skill each call serves).
  5. Classify each shim envelope by prompt marker (filter vs. profile).
  6. Pair the filter envelope with the Go filter prompt (1:1), re-key
     under Go prompt hash, write to testdata/envelopes/profile-entities/.
  7. Parse filter response → extract keep names + types.
  8. Invoke parity-discover --mode profile with the keep set → Go-side
     profile prompts (one per kept entity, ranked order).
  9. Pair each prototype profile envelope with its Go counterpart by
     entity name extracted from the prompt; re-key + write.
 10. Snapshot the post-Python vault to testdata/expected/profile-entities/post-state/.

Re-run any time the fixture vault, the prototype, or either skill
template changes. Burns real claude -p tokens.

Environment:
  REAL_CLAUDE   path to real claude binary (default: shutil.which("claude"))
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
PROTOTYPE_SCRIPT = REPO_ROOT / "references" / "scripts" / "profile-entities.py"
SHIM = REPO_ROOT / "testdata" / "scripts" / "claude-shim.py"

FILTER_SKILL = REPO_ROOT / ".agents" / "shared" / "skills" / "inference" / "filter-entity-candidates.md"
PROFILE_SKILL = REPO_ROOT / ".agents" / "shared" / "skills" / "inference" / "generate-entity-profile.md"
SKILLS_DIR = FILTER_SKILL.parent

SEA_NOISE_BIN = REPO_ROOT / ".tempor" / "spike" / "bin" / "sea-noise"
VENV_PYTHON = REPO_ROOT / ".venv" / "bin" / "python"

ENVELOPE_OUT = REPO_ROOT / "testdata" / "envelopes" / "profile-entities"
EXPECTED_OUT = REPO_ROOT / "testdata" / "expected" / "profile-entities" / "post-state"

FILTER_MARKER = "You are filtering entity candidates"
PROFILE_MARKER_RE = re.compile(r"Generate a profile note for '([^']+)'")


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


def discover_filter_prompt() -> str:
    result = run(
        [
            str(SEA_NOISE_BIN),
            "parity-discover",
            "--skill", "profile-entities",
            "--mode", "filter",
            "--vault", str(FIXTURE_VAULT),
            "--skills-dir", str(SKILLS_DIR),
        ],
        capture_output=True,
        text=True,
    )
    arr = json.loads(result.stdout)
    if len(arr) != 1 or arr[0].get("kind") != "filter":
        raise RuntimeError(f"expected 1 filter prompt, got {arr!r}")
    return arr[0]["prompt"]


def discover_profile_prompts(keep_names: list[str], keep_types: dict[str, str]) -> list[dict]:
    keep_csv = ",".join(keep_names)
    types_csv = ",".join(f"{n}:{keep_types.get(n, 'concept')}" for n in keep_names)
    result = run(
        [
            str(SEA_NOISE_BIN),
            "parity-discover",
            "--skill", "profile-entities",
            "--mode", "profile",
            "--vault", str(FIXTURE_VAULT),
            "--skills-dir", str(SKILLS_DIR),
            "--keep-names", keep_csv,
            "--keep-types", types_csv,
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
    shutil.copy(PROTOTYPE_SCRIPT, scripts_dir / "profile-entities.py")
    return vault


def run_prototype(vault: Path, shim_bin_dir: Path, capture_dir: Path, real_claude: str) -> None:
    env = os.environ.copy()
    env["PATH"] = f"{shim_bin_dir}:{env['PATH']}"
    env["SEA_NOISE_CAPTURE_DIR"] = str(capture_dir)
    # Shim wants a skill_name + skill_path at write time; we fix up
    # post-pairing. Use the filter skill file as a stand-in path; the
    # written skill_sha will be overwritten when we re-emit.
    env["SEA_NOISE_CAPTURE_SKILL"] = "profile-entities-pending"
    env["SEA_NOISE_CAPTURE_SKILL_PATH"] = str(FILTER_SKILL)
    env["SEA_NOISE_REAL_CLAUDE"] = real_claude
    env["CLAUDECODE"] = ""

    python = str(VENV_PYTHON) if VENV_PYTHON.exists() else "python3"
    run([python, str(vault / "scripts" / "profile-entities.py")], env=env, cwd=vault)


def load_raw_envelopes(capture_dir: Path) -> list[dict]:
    envs = []
    for path in sorted(capture_dir.glob("*.json")):
        envs.append(json.loads(path.read_text()))
    return envs


def classify_envelope(env: dict) -> tuple[str, str | None]:
    """Return (kind, entity_name). kind ∈ {"filter", "profile"}."""
    prompt = env["prompt"]
    if FILTER_MARKER in prompt:
        return ("filter", None)
    m = PROFILE_MARKER_RE.search(prompt)
    if m:
        return ("profile", m.group(1))
    return ("unknown", None)


def parse_keep_set(filter_response: str) -> tuple[list[str], dict[str, str]]:
    """Extract keep names + types from the filter response JSON array.

    Mirrors references/scripts/profile-entities.py:filter_candidates_with_claude
    parsing — find the first [...] block in the response, JSON-decode it,
    pick items with keep=true.
    """
    m = re.search(r"\[.*\]", filter_response, re.DOTALL)
    if not m:
        raise RuntimeError("no JSON array in filter response")
    items = json.loads(m.group(0))
    names: list[str] = []
    types: dict[str, str] = {}
    for item in items:
        if item.get("keep"):
            name = item["name"]
            names.append(name)
            types[name] = item.get("type", "concept")
    return names, types


def write_envelope(rel_skill_path: Path, skill_name: str, go_prompt: str, response: str) -> str:
    envelope = {
        "skill_name": skill_name,
        "skill_sha": file_sha(rel_skill_path),
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

    print("==> discovering Go-side filter prompt")
    go_filter_prompt = discover_filter_prompt()

    ENVELOPE_OUT.mkdir(parents=True, exist_ok=True)
    # Wipe old envelopes so stale ones don't linger.
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

        # Classify.
        filter_envs = []
        profile_envs: dict[str, dict] = {}
        for env in raw_envs:
            kind, entity = classify_envelope(env)
            if kind == "filter":
                filter_envs.append(env)
            elif kind == "profile" and entity:
                profile_envs[entity] = env
            else:
                print(f"  WARN: could not classify envelope {env.get('prompt_hash')}", file=sys.stderr)

        if len(filter_envs) != 1:
            print(f"error: expected 1 filter envelope, got {len(filter_envs)}", file=sys.stderr)
            return 1

        # Pair + re-key filter.
        filter_env = filter_envs[0]
        filter_name = write_envelope(
            FILTER_SKILL, "filter-entity-candidates", go_filter_prompt, filter_env["response"]
        )
        print(f"  wrote envelope: filter → {filter_name}")

        # Parse keep set from filter response.
        keep_names, keep_types = parse_keep_set(filter_env["response"])
        print(f"  filter response kept {len(keep_names)} entities: {keep_names}")

        # Discover Go-side profile prompts for the kept entities.
        print("==> discovering Go-side profile prompts")
        go_profile_prompts = discover_profile_prompts(keep_names, keep_types)
        print(f"    {len(go_profile_prompts)} Go profile prompt(s) (after MaxPerRun cap)")

        # Pair + re-key each profile.
        written = 1  # filter already written
        for go in go_profile_prompts:
            entity = go["rel"]
            proto = profile_envs.get(entity)
            if proto is None:
                print(f"  WARN: no prototype envelope for entity {entity!r}", file=sys.stderr)
                continue
            name = write_envelope(
                PROFILE_SKILL, "generate-entity-profile", go["prompt"], proto["response"]
            )
            print(f"  wrote envelope: profile {entity} → {name}")
            written += 1

        expected_total = 1 + len(go_profile_prompts)
        if written != expected_total:
            print(
                f"error: wrote {written} of {expected_total} expected envelopes",
                file=sys.stderr,
            )
            return 1

        print("==> snapshotting post-state vault")
        snapshot_post_state(vault)

    print(f"==> done. {written} envelope(s) at {ENVELOPE_OUT}")
    print(f"==> post-state snapshot at {EXPECTED_OUT}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
