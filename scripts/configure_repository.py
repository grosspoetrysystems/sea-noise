#!/usr/bin/env python3
"""Configure Sea Noise's repository mode and default-branch protection."""

from __future__ import annotations

import argparse
import json
import subprocess
from typing import Any

MODES = ("solo", "contributor", "team")
REQUIRED_CHECKS = ("ci", "test-validator", "validate-metadata", "DCO")
REPOSITORY_LABELS = {
    "queue:awaiting-reviewer": ("fbca04", "Waiting on a maintainer; the queue clock is running."),
    "queue:awaiting-author": ("c5def5", "Waiting on the contributor."),
    "queue:no-capacity": ("d4c5f9", "Paused: no maintainer capacity yet. Stays open and keeps its place."),
    "needs-triage": ("ededed", "Needs maintainer triage."),
    "automation-review": ("5319e7", "Requires maintainer review before automation may interact."),
}


def protection(mode: str) -> dict[str, Any]:
    reviews = mode != "solo"
    return {
        "required_status_checks": {
            "strict": True,
            "contexts": list(REQUIRED_CHECKS),
        },
        "enforce_admins": False,
        "required_pull_request_reviews": {
            "dismiss_stale_reviews": reviews,
            "require_code_owner_reviews": reviews,
            "required_approving_review_count": int(reviews),
            "require_last_push_approval": mode == "team",
        },
        "restrictions": None,
        "required_linear_history": False,
        "allow_force_pushes": False,
        "allow_deletions": False,
        "block_creations": False,
        "required_conversation_resolution": True,
        "lock_branch": False,
        "allow_fork_syncing": True,
    }


def gh(*arguments: str, input_text: str | None = None) -> str:
    result = subprocess.run(
        ["gh", *arguments],
        check=False,
        capture_output=True,
        input=input_text,
        text=True,
    )
    if result.returncode:
        raise SystemExit(result.stderr.strip() or result.stdout.strip())
    return result.stdout.strip()


def set_variable(repository: str, name: str, value: str) -> None:
    gh("variable", "set", name, "--body", value, "--repo", repository)


def configure(repository: str, mode: str) -> None:
    variables = {
        item["name"]
        for item in json.loads(gh("variable", "list", "--json", "name", "--repo", repository))
    }

    if "HELICOPTER_SOLO_MAINTAINER_MODE" in variables:
        gh(
            "variable",
            "delete",
            "HELICOPTER_SOLO_MAINTAINER_MODE",
            "--repo",
            repository,
        )

    # Disable exemptions before tightening non-solo modes.
    if mode != "solo":
        set_variable(repository, "HELICOPTER_MODE", "contributor")

    default_branch = gh(
        "repo",
        "view",
        repository,
        "--json",
        "defaultBranchRef",
        "--jq",
        ".defaultBranchRef.name",
    )
    gh(
        "api",
        "--method",
        "PUT",
        f"repos/{repository}/branches/{default_branch}/protection",
        "--input",
        "-",
        input_text=json.dumps(protection(mode)),
    )

    if mode == "solo":
        maintainer_id = gh("api", "user", "--jq", ".id")
        set_variable(repository, "HELICOPTER_SOLO_MAINTAINER_ID", maintainer_id)
    elif "HELICOPTER_SOLO_MAINTAINER_ID" in variables:
        gh(
            "variable",
            "delete",
            "HELICOPTER_SOLO_MAINTAINER_ID",
            "--repo",
            repository,
        )

    set_variable(repository, "HELICOPTER_MODE", mode)
    for name, (color, description) in REPOSITORY_LABELS.items():
        gh("label", "create", name, "--color", color, "--description", description, "--force", "--repo", repository)
    print(f"Configured {repository} for Sea Noise {mode} mode on {default_branch}.")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=MODES)
    parser.add_argument("--repo", help="OWNER/REPOSITORY; defaults to the current repository")
    args = parser.parse_args()

    repository = args.repo or gh(
        "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner"
    )
    configure(repository, args.mode)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
