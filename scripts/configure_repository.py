#!/usr/bin/env python3
"""Configure Helicopter's repository mode, branch protection, and security baseline."""

from __future__ import annotations

from __future__ import annotations

import argparse
import json
import subprocess
from typing import Any

MODES = ("solo", "contributor", "team")
REQUIRED_CHECKS = ("test-validator", "validate-metadata")
TAG_RULESET = "helicopter-release-tags"
QUEUE_LABELS = {
    "queue:awaiting-reviewer": ("fbca04", "Waiting on a maintainer; the queue clock is running."),
    "queue:awaiting-author": ("c5def5", "Waiting on the contributor."),
    "queue:no-capacity": ("d4c5f9", "Paused: no maintainer capacity yet. Stays open and keeps its place."),
    "queue:possibly-superseded": ("e4e669", "Another pull request for the same issue merged while this one waited."),
}
RISK_LABELS = {
    "risk:dependencies": ("b60205", "Advisory: changed dependency manifests or lockfiles."),
    "risk:automation": ("fbca04", "Advisory: changed automation, build, release, policy, or agent instructions."),
    "risk:generated": ("d4c5f9", "Advisory: changed generated or vendored content marked in .gitattributes."),
}

def protection(mode: str, extra_checks: tuple[str, ...] = ()) -> dict[str, Any]:
    reviews = mode != "solo"
    return {
        "required_status_checks": {
            "strict": True,
            "contexts": [*REQUIRED_CHECKS, *extra_checks],
        },
        # Team mode has several maintainers, so administrators follow the same review rule.
        # Solo and contributor modes may have one maintainer, who needs admin merge for their own work.
        "enforce_admins": mode == "team",
        "required_pull_request_reviews": {
            "dismiss_stale_reviews": reviews,
            "require_code_owner_reviews": reviews,
            "required_approving_review_count": int(reviews),
            # Whoever pushed last can't also approve: an agent's push after approval needs fresh review.
            "require_last_push_approval": reviews,
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


def tag_ruleset() -> dict[str, Any]:
    """Release tags can't be moved or deleted, so a tag's release manifest stays immutable."""
    return {
        "name": TAG_RULESET,
        "target": "tag",
        "enforcement": "active",
        "conditions": {"ref_name": {"include": ["refs/tags/v*"], "exclude": []}},
        "rules": [{"type": "update"}, {"type": "deletion"}],
    }


def baseline(repository: str) -> list[tuple[str, str, dict[str, Any] | None]]:
    """Repository settings every mode shares, as (method, endpoint, payload) API calls."""
    repo = f"repos/{repository}"
    return [
        # Workflows get a read-only token by default and can never approve pull requests.
        ("PUT", f"{repo}/actions/permissions/workflow", {
            "default_workflow_permissions": "read",
            "can_approve_pull_request_reviews": False,
        }),
        ("PUT", f"{repo}/actions/permissions/fork-pr-contributor-approval", {"approval_policy": "first_time_contributors"}),
        ("PUT", f"{repo}/vulnerability-alerts", None),
        ("PUT", f"{repo}/private-vulnerability-reporting", None),
        # Web commits (including "Apply suggestion") carry a sign-off, so admission's DCO check accepts them.
        ("PATCH", repo, {"web_commit_signoff_required": True, "security_and_analysis": {
            "secret_scanning": {"status": "enabled"},
            "secret_scanning_push_protection": {"status": "enabled"},
            "dependabot_security_updates": {"status": "enabled"},
        }}),
    ]


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


def api(method: str, endpoint: str, payload: dict[str, Any] | None, dry_run: bool) -> None:
    if dry_run:
        print(f"{method} {endpoint}" + (f" {json.dumps(payload)}" if payload else ""))
        return
    gh("api", "--method", method, endpoint, *(["--input", "-"] if payload else []), input_text=json.dumps(payload) if payload else None)


def configure(repository: str, mode: str, dry_run: bool = False, extra_checks: tuple[str, ...] = ()) -> None:
    def run(*arguments: str) -> None:
        if dry_run:
            print("gh " + " ".join(arguments))
        else:
            gh(*arguments)

    variables = {
        item["name"]
        for item in json.loads(gh("variable", "list", "--json", "name", "--repo", repository))
    }

    if "HELICOPTER_SOLO_MAINTAINER_MODE" in variables:
        run("variable", "delete", "HELICOPTER_SOLO_MAINTAINER_MODE", "--repo", repository)

    # Disable exemptions before tightening non-solo modes.
    if mode != "solo":
        run("variable", "set", "HELICOPTER_MODE", "--body", "contributor", "--repo", repository)

    default_branch = gh("repo", "view", repository, "--json", "defaultBranchRef", "--jq", ".defaultBranchRef.name")
    # Keep checks the repository already requires (its own CI, DCO): PUT replaces the whole list.
    try:
        current = json.loads(gh("api", f"repos/{repository}/branches/{default_branch}/protection/required_status_checks/contexts"))
    except SystemExit:
        current = []  # not protected yet
    extra_checks = tuple(dict.fromkeys([*extra_checks, *(c for c in current if c not in REQUIRED_CHECKS)]))
    api("PUT", f"repos/{repository}/branches/{default_branch}/protection", protection(mode, extra_checks), dry_run)

    if mode == "solo":
        run("variable", "set", "HELICOPTER_SOLO_MAINTAINER_ID", "--body", gh("api", "user", "--jq", ".id"), "--repo", repository)
    elif "HELICOPTER_SOLO_MAINTAINER_ID" in variables:
        run("variable", "delete", "HELICOPTER_SOLO_MAINTAINER_ID", "--repo", repository)

    run("variable", "set", "HELICOPTER_MODE", "--body", mode, "--repo", repository)
    for labels in (QUEUE_LABELS, RISK_LABELS):
        for name, (color, description) in labels.items():
            run("label", "create", name, "--color", color, "--description", description, "--force", "--repo", repository)

    manual = []
    for method, endpoint, payload in baseline(repository):
        try:
            api(method, endpoint, payload, dry_run)
        except SystemExit as error:
            # Some features depend on the plan (secret scanning on private repositories, for one).
            manual.append(f"{method} {endpoint}: {error}")

    rulesets = json.loads(gh("api", f"repos/{repository}/rulesets"))
    existing = next((ruleset["id"] for ruleset in rulesets if ruleset["name"] == TAG_RULESET), None)
    if existing:
        api("PUT", f"repos/{repository}/rulesets/{existing}", tag_ruleset(), dry_run)
    else:
        api("POST", f"repos/{repository}/rulesets", tag_ruleset(), dry_run)

    print(f"{'Would configure' if dry_run else 'Configured'} {repository} for Helicopter {mode} mode on {default_branch}.")
    for step in manual:
        print(f"Manual step (GitHub refused, likely the plan): {step}")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=MODES)
    parser.add_argument("--repo", help="OWNER/REPOSITORY; defaults to the current repository")
    parser.add_argument("--dry-run", action="store_true", help="print every change instead of making it")
    parser.add_argument(
        "--extra-check",
        action="append",
        default=[],
        help="another required status check, repeatable; only checks your pull request workflows always report",
    )
    args = parser.parse_args()

    repository = args.repo or gh(
        "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner"
    )
    configure(repository, args.mode, args.dry_run, tuple(args.extra_check))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
