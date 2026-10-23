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

REQUIRED_SECTIONS = (
    "## Human accountability",
    "## Verification",
    "## Risk",
    "## AI / automation",
)
ORIGINS = (
    "Human-authored",
    "AI-assisted",
    "Agent-authored",
    "Authorized automation",
)
AGENT_ORIGINS = {"Agent-authored", "Authorized automation"}
ATTESTATIONS = (
    "I reviewed the complete diff.",
    "I understand and can explain the change.",
    "I verified the evidence below.",
    "I have the right to submit this contribution.",
)
CAPABILITY_KEYS = {
    "repository_write": bool,
    "command_execution": bool,
    "network": str,
    "secrets": str,
    "approve": bool,
    "merge": bool,
}
USERNAME = r"[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?"


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


def _checked(body: str, label: str) -> bool:
    return re.search(rf"^- \[[xX]\] {re.escape(label)}\s*$", body, re.MULTILINE) is not None


def _backtick_field(body: str, label: str) -> str | None:
    match = re.search(rf"^{re.escape(label)}:\s*`([^`]+)`\s*$", body, re.IGNORECASE | re.MULTILINE)
    return match.group(1).strip() if match else None


def _capabilities(body: str) -> tuple[dict[str, Any] | None, Refusal | None]:
    match = re.search(
        r"^Capabilities:\s*\n+```json\s*\n(?P<value>\{.*?\})\s*\n```",
        body,
        re.IGNORECASE | re.MULTILINE | re.DOTALL,
    )
    if not match:
        return None, Refusal(
            "REFUSE_CAPABILITIES_INVALID",
            "Agent-authored work requires the JSON capability block from the pull request template.",
        )

    try:
        value = json.loads(match.group("value"))
    except json.JSONDecodeError as error:
        return None, Refusal(
            "REFUSE_CAPABILITIES_INVALID",
            f"Capability JSON is invalid: {error.msg}.",
        )

    if not isinstance(value, dict):
        return None, Refusal("REFUSE_CAPABILITIES_INVALID", "Capabilities must be a JSON object.")

    invalid = [key for key, kind in CAPABILITY_KEYS.items() if type(value.get(key)) is not kind]
    unknown = sorted(set(value) - set(CAPABILITY_KEYS))
    if invalid or unknown:
        details = []
        if invalid:
            details.append(f"missing or wrong type: {', '.join(invalid)}")
        if unknown:
            details.append(f"unknown: {', '.join(unknown)}")
        return None, Refusal(
            "REFUSE_CAPABILITIES_INVALID",
            "Use exactly the capability fields from the template (" + "; ".join(details) + ").",
        )

    if value["network"] not in {"none", "restricted", "unrestricted"}:
        return None, Refusal(
            "REFUSE_CAPABILITIES_INVALID",
            "Capability `network` must be `none`, `restricted`, or `unrestricted`.",
        )

    return value, None


def validate_body(body: str) -> list[Refusal]:
    refusals: list[Refusal] = []

    missing_sections = [section for section in REQUIRED_SECTIONS if section not in body]
    if missing_sections:
        refusals.append(
            Refusal(
                "REFUSE_TEMPLATE_INCOMPLETE",
                "Missing required sections: " + ", ".join(missing_sections) + ".",
            )
        )

    human = re.search(
        rf"^Accountable contributor:\s*@(?P<user>{USERNAME})\s*$",
        body,
        re.IGNORECASE | re.MULTILINE,
    )
    if not human or human.group("user").upper() in {"YOUR_GITHUB_USERNAME", "USERNAME"}:
        refusals.append(
            Refusal(
                "REFUSE_HUMAN_ACCOUNTABILITY_MISSING",
                "Declare `Accountable contributor: @github-user` with a real username.",
            )
        )

    unchecked = [label for label in ATTESTATIONS if not _checked(body, label)]
    if unchecked:
        refusals.append(
            Refusal(
                "REFUSE_ACCOUNTABILITY_UNCONFIRMED",
                "Confirm every human-accountability checkbox.",
            )
        )

    selected_origins = [origin for origin in ORIGINS if _checked(body, origin)]
    if len(selected_origins) != 1:
        refusals.append(
            Refusal(
                "REFUSE_AGENT_ORIGIN_AMBIGUOUS",
                "Select exactly one AI / automation origin.",
            )
        )
        return refusals

    if selected_origins[0] not in AGENT_ORIGINS:
        return refusals

    identity = _backtick_field(body, "Agent identity")
    if not identity or identity.lower() == "none":
        refusals.append(
            Refusal(
                "REFUSE_AGENT_IDENTITY_UNKNOWN",
                "Agent-authored work requires a non-empty agent or bot identity.",
            )
        )

    run_id = _backtick_field(body, "Run ID")
    base_sha = _backtick_field(body, "Base SHA")
    if not run_id or run_id.lower() == "none":
        refusals.append(Refusal("REFUSE_PROVENANCE_INCOMPLETE", "Provide a non-empty immutable Run ID."))
    if not base_sha or re.fullmatch(r"[0-9a-fA-F]{40}", base_sha) is None:
        refusals.append(
            Refusal("REFUSE_PROVENANCE_INCOMPLETE", "Provide the full 40-character base commit SHA.")
        )

    capabilities, capability_refusal = _capabilities(body)
    if capability_refusal:
        refusals.append(capability_refusal)
    elif capabilities:
        if capabilities["secrets"].lower() != "none":
            refusals.append(
                Refusal(
                    "REFUSE_SECRET_ACCESS",
                    "Untrusted contribution runs may not use repository or deployment secrets.",
                )
            )
        if capabilities["approve"] or capabilities["merge"]:
            refusals.append(
                Refusal(
                    "REFUSE_AGENT_SELF_APPROVAL",
                    "An authoring agent may not approve or merge its own contribution.",
                )
            )

    return refusals


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
    args = parser.parse_args()

    try:
        event = json.loads(args.event.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        print(f"REFUSE_EVENT_INVALID: {error}")
        return 2

    if re.fullmatch(r"[1-9][0-9]*", args.solo_maintainer_id):
        solo_maintainer_id = int(args.solo_maintainer_id)
    else:
        solo_maintainer_id = 0

    mode = args.mode if args.mode in {"solo", "contributor", "team"} else "contributor"
    if _metadata_exempt(event, mode, solo_maintainer_id):
        print(f"{mode.capitalize()}-mode metadata exemption accepted for the trusted maintainer.")
        return 0

    body = (event.get("pull_request") or {}).get("body") or ""
    refusals = validate_body(body)
    if refusals:
        print("Contribution intake refused:")
        for refusal in refusals:
            print(f"{refusal.code}: {refusal.message}")
        print("\nUpdate the pull request description from the repository template.")
        return 1

    print("Contribution metadata accepted. Human review is still required.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
