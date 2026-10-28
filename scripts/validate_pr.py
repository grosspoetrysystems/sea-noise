#!/usr/bin/env python3
"""Validate the metadata contract in a GitHub pull request event."""

from __future__ import annotations

import argparse
import json
import re
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any

REQUIRED_SECTIONS = ("Change", "Verification", "Risk")
PLACEHOLDERS = (
    "Closes #",
    "Commands and results:",
    "<commands and results; say what you couldn't run or test. Project CI verifies independently.>",
    "<commands and concise result; project CI verifies independently>",  # earlier wording, still in open pull requests
    "Check every area this pull request changes:",
    "Details and mitigations:",
)


@dataclass(frozen=True)
class Refusal:
    code: str
    message: str


def _metadata_exempt(event: dict[str, Any], mode: str, solo_maintainer_id: int) -> bool:
    pull_request = event.get("pull_request") or {}
    author = pull_request.get("user") or {}
    base_repository = (pull_request.get("base") or {}).get("repo") or {}
    head_repository = (pull_request.get("head") or {}).get("repo") or {}
    repository = event.get("repository") or {}
    owner = base_repository.get("owner") or {}
    sender = event.get("sender") or {}
    repository_id = repository.get("id")

    trusted_same_repository_author = (
        type(repository_id) is int
        and repository_id > 0
        and owner.get("type") in {"User", "Organization"}
        and author.get("type") == "User"
        and sender.get("type") == "User"
        and type(author.get("id")) is int
        and author.get("id") > 0
        and sender.get("id") == author.get("id")
        and base_repository.get("id") == repository_id
        and head_repository.get("id") == repository_id
    )
    if not trusted_same_repository_author:
        return False

    if mode == "team":
        return pull_request.get("author_association") in {"OWNER", "MEMBER", "COLLABORATOR"}

    if mode != "solo" or author.get("id") != solo_maintainer_id:
        return False

    return owner.get("type") == "Organization" or owner.get("id") == solo_maintainer_id


def _repository_bot(event: dict[str, Any]) -> bool:
    """A bot pushing a branch of this repository. GitHub allows that only with write access an administrator
    granted, so the grant is the authorization; there is no list to keep."""
    pull_request = event.get("pull_request") or {}
    author = pull_request.get("user") or {}
    sender = event.get("sender") or {}
    repository_id = (event.get("repository") or {}).get("id")
    return (
        author.get("type") == "Bot"
        and bool(author.get("login"))
        and sender.get("login") == author.get("login")
        and type(repository_id) is int
        and ((pull_request.get("base") or {}).get("repo") or {}).get("id") == repository_id
        and ((pull_request.get("head") or {}).get("repo") or {}).get("id") == repository_id
    )


def _section(body: str, name: str) -> str | None:
    """Return a section's text without comments and template placeholders, or None if absent."""
    match = re.search(rf"^## {re.escape(name)}[ \t]*$(?P<text>.*?)(?=^## |\Z)", body, re.MULTILINE | re.DOTALL)
    if not match:
        return None
    text = re.sub(r"<!--.*?-->", "", match.group("text"), flags=re.DOTALL)
    lines = [line.strip() for line in text.splitlines()]
    return "\n".join(line for line in lines if line and line not in PLACEHOLDERS and not line.startswith("```"))


def validate_body(body: str) -> list[Refusal]:
    refusals: list[Refusal] = []
    sections = {name: _section(body, name) for name in REQUIRED_SECTIONS}

    missing = [f"## {name}" for name, text in sections.items() if text is None]
    if missing:
        refusals.append(
            Refusal(
                "REFUSE_TEMPLATE_INCOMPLETE",
                "Missing required sections: " + ", ".join(missing) + ". Copy them from the pull request template.",
            )
        )

    if sections["Change"] == "":
        refusals.append(
            Refusal("REFUSE_CHANGE_EMPTY", "Under ## Change, describe the problem, the fix, and its visible effect.")
        )
    if sections["Verification"] == "":
        refusals.append(
            Refusal(
                "REFUSE_VERIFICATION_EMPTY",
                "Under ## Verification, list the commands or steps you ran and what they showed.",
            )
        )
    if sections["Risk"] is not None and not re.search(r"^- \[[xX]\] ", sections["Risk"], re.MULTILINE):
        refusals.append(
            Refusal(
                "REFUSE_RISK_UNCHECKED",
                "Under ## Risk, check every area this pull request changes, or check `None of the above`.",
            )
        )

    return refusals


def _commit_pages(commits: list[Any]) -> list[dict[str, Any]]:
    """Accept the commits endpoint's array, or `gh api --paginate --slurp` pages of it."""
    return [commit for item in commits for commit in (item if isinstance(item, list) else [item])]


def validate_signoff(commits: list[Any]) -> list[Refusal]:
    """Every non-merge commit carries a `Signed-off-by` with its author's email (DCO 1.1)."""
    unsigned = []
    for commit in _commit_pages(commits):
        if len(commit.get("parents") or []) > 1:
            continue  # merge commits, such as GitHub's "Update branch", carry no sign-off
        details = commit.get("commit") or {}
        email = ((details.get("author") or {}).get("email") or "").lower()
        signers = re.findall(r"^Signed-off-by: .*<([^>]+)>\s*$", details.get("message") or "", re.MULTILINE | re.IGNORECASE)
        if not email or email not in {signer.lower() for signer in signers}:
            unsigned.append(str(commit.get("sha") or "?")[:7])
    if not unsigned:
        return []
    return [
        Refusal(
            "REFUSE_SIGNOFF_MISSING",
            f"Commits without a `Signed-off-by` matching their author's email: {', '.join(unsigned)}. "
            "Run `git rebase --signoff` on your branch and force-push.",
        )
    ]


def validate_open_prs(open_prs: int, limit: int) -> list[Refusal]:
    if limit <= 0 or open_prs <= limit:
        return []
    return [
        Refusal(
            "REFUSE_OPEN_PR_LIMIT",
            f"You have {open_prs} open non-draft pull requests here; the limit is {limit}. "
            "Convert one to a draft or close it, then edit this description to re-run the check.",
        )
    ]


def validate_shape(pull_request: dict[str, Any], default_branch: str, max_lines: int, max_files: int) -> list[Refusal]:
    """Outside contributions target the default branch and stay small enough to review. A limit of 0 is off."""
    refusals = []
    base = (pull_request.get("base") or {}).get("ref")
    if default_branch and base and base != default_branch:
        refusals.append(
            Refusal(
                "REFUSE_BASE_BRANCH",
                f"This pull request targets `{base}`; contributions go to `{default_branch}`. "
                "Change the base branch with **Edit** next to the title.",
            )
        )
    lines = (pull_request.get("additions") or 0) + (pull_request.get("deletions") or 0)
    files = pull_request.get("changed_files") or 0
    over = []
    if max_lines and lines > max_lines:
        over.append(f"{lines} changed lines (limit {max_lines})")
    if max_files and files > max_files:
        over.append(f"{files} changed files (limit {max_files})")
    if over:
        refusals.append(
            Refusal(
                "REFUSE_TOO_LARGE",
                f"This pull request has {' and '.join(over)}. Split it into smaller pull requests, "
                "or ask a maintainer in an issue first.",
            )
        )
    return refusals


def _limit(value: str, default: int) -> int:
    return int(value) if re.fullmatch(r"[0-9]+", value) else default


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("event", type=Path, help="GitHub pull request event JSON")
    parser.add_argument(
        "--mode",
        default="contributor",
        help="repository mode: solo, contributor, or team",
    )
    parser.add_argument(
        "--solo-maintainer-id",
        default="",
        help="immutable numeric GitHub user ID authorized in solo mode",
    )
    parser.add_argument(
        "--commits",
        type=Path,
        help="JSON from the pull request commits endpoint; checks DCO sign-off when given",
    )
    parser.add_argument(
        "--open-prs",
        type=int,
        default=0,
        help="the author's open non-draft pull requests in this repository, this one included",
    )
    parser.add_argument(
        "--open-pr-limit",
        default="2",
        help="most open non-draft pull requests an outside contributor may have; 0 disables",
    )
    parser.add_argument("--max-changed-lines", default="5000", help="outside contributors' size limit; 0 disables")
    parser.add_argument("--max-changed-files", default="300", help="outside contributors' file limit; 0 disables")
    args = parser.parse_args()

    try:
        event = json.loads(args.event.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        print(f"REFUSE_EVENT_INVALID: {error}")
        return 2

    if (event.get("pull_request") or {}).get("draft"):
        print("Draft: admission runs when the pull request is marked ready for review.")
        return 0

    if re.fullmatch(r"[1-9][0-9]*", args.solo_maintainer_id):
        solo_maintainer_id = int(args.solo_maintainer_id)
    else:
        solo_maintainer_id = 0

    mode = args.mode if args.mode in {"solo", "contributor", "team"} else "contributor"
    if _metadata_exempt(event, mode, solo_maintainer_id):
        print(f"{mode.capitalize()}-mode metadata exemption accepted for the trusted maintainer.")
        return 0

    if _repository_bot(event):
        print("Bot on a branch of this repository: authorized by its write access. Human review is still required.")
        return 0

    pull_request = event.get("pull_request") or {}
    if (pull_request.get("user") or {}).get("type") == "Bot":
        print(
            "REFUSE_BOT_UNAUTHORIZED: bots open pull requests only from a branch of this repository, which needs "
            "write access an administrator grants. Agents submit through a person's account; that person signs off."
        )
        return 1

    refusals = validate_body(pull_request.get("body") or "")
    if args.commits:
        try:
            commits = json.loads(args.commits.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError) as error:
            print(f"REFUSE_EVENT_INVALID: commits: {error}")
            return 2
        refusals += validate_signoff(commits)
    if pull_request.get("author_association") not in {"OWNER", "MEMBER", "COLLABORATOR"}:
        refusals += validate_open_prs(args.open_prs, _limit(args.open_pr_limit, 2))
        default_branch = (event.get("repository") or {}).get("default_branch") or ""
        refusals += validate_shape(
            pull_request, default_branch, _limit(args.max_changed_lines, 5000), _limit(args.max_changed_files, 300)
        )
    if refusals:
        print("Contribution intake refused:")
        for refusal in refusals:
            print(f"{refusal.code}: {refusal.message}")
        return 1

    print("Contribution metadata accepted. Human review is still required.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
