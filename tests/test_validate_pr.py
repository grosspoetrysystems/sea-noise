#!/usr/bin/env python3

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))

from configure_repository import protection
from validate_pr import validate_body

SCRIPT = ROOT / "scripts" / "validate_pr.py"
TEMPLATE = ROOT / ".github" / "pull_request_template.md"


VALID_HUMAN = """## Change

Small fix.

## Human accountability

Accountable contributor: @alice

- [x] I reviewed the complete diff.
- [x] I understand and can explain the change.
- [x] I verified the evidence below.
- [x] I have the right to submit this contribution.

## Verification

Ran the check.

## Risk

- [x] None of the above

## AI / automation

- [x] Human-authored
- [ ] AI-assisted
- [ ] Agent-authored
- [ ] Authorized automation

Agent identity: `none`
Run ID: `none`
Base SHA: `none`

Capabilities:

```json
{
  "repository_write": false,
  "command_execution": false,
  "network": "none",
  "secrets": "none",
  "approve": false,
  "merge": false
}
```
"""


def run_cli(event: dict[str, object], *arguments: str) -> subprocess.CompletedProcess[str]:
    with tempfile.TemporaryDirectory() as directory:
        event_path = Path(directory) / "event.json"
        event_path.write_text(json.dumps(event), encoding="utf-8")
        return subprocess.run(
            [sys.executable, str(SCRIPT), str(event_path), *arguments],
            check=False,
            capture_output=True,
            text=True,
        )


def owner_event(
    *,
    author_id: int = 1,
    author_type: str = "User",
    owner_type: str = "User",
    owner_id: int = 1,
    sender_id: int = 1,
    sender_type: str = "User",
    event_repository_id: int = 100,
    head_repository_id: int = 100,
    author_association: str = "OWNER",
) -> dict[str, object]:
    return {
        "repository": {"id": event_repository_id},
        "sender": {"id": sender_id, "type": sender_type},
        "pull_request": {
            "author_association": author_association,
            "body": "",
            "user": {"id": author_id, "type": author_type},
            "base": {"repo": {"id": 100, "owner": {"id": owner_id, "type": owner_type}}},
            "head": {"repo": {"id": head_repository_id}},
        },
    }


def codes(body: str) -> set[str]:
    return {refusal.code for refusal in validate_body(body)}


class ValidateBodyTests(unittest.TestCase):
    def test_valid_human_submission_passes(self) -> None:
        self.assertEqual(codes(VALID_HUMAN), set())

    def test_accountability_must_be_real_and_confirmed(self) -> None:
        body = VALID_HUMAN.replace("@alice", "@YOUR_GITHUB_USERNAME").replace(
            "[x] I reviewed the complete diff.", "[ ] I reviewed the complete diff."
        )
        self.assertEqual(
            codes(body),
            {"REFUSE_HUMAN_ACCOUNTABILITY_MISSING", "REFUSE_ACCOUNTABILITY_UNCONFIRMED"},
        )

    def test_exactly_one_origin_is_required(self) -> None:
        body = VALID_HUMAN.replace("[ ] AI-assisted", "[x] AI-assisted")
        self.assertIn("REFUSE_AGENT_ORIGIN_AMBIGUOUS", codes(body))

    def test_agent_submission_requires_identity_and_provenance(self) -> None:
        body = VALID_HUMAN.replace("[x] Human-authored", "[ ] Human-authored").replace(
            "[ ] Agent-authored", "[x] Agent-authored"
        )
        self.assertEqual(
            codes(body),
            {"REFUSE_AGENT_IDENTITY_UNKNOWN", "REFUSE_PROVENANCE_INCOMPLETE"},
        )

    def test_valid_agent_submission_passes(self) -> None:
        body = (
            VALID_HUMAN.replace("[x] Human-authored", "[ ] Human-authored")
            .replace("[ ] Agent-authored", "[x] Agent-authored")
            .replace("Agent identity: `none`", "Agent identity: `builder[bot]`")
            .replace("Run ID: `none`", "Run ID: `run-123`")
            .replace("Base SHA: `none`", "Base SHA: `" + "a" * 40 + "`")
        )
        self.assertEqual(codes(body), set())

    def test_agent_cannot_claim_secrets_or_merge(self) -> None:
        body = (
            VALID_HUMAN.replace("[x] Human-authored", "[ ] Human-authored")
            .replace("[ ] Agent-authored", "[x] Agent-authored")
            .replace("Agent identity: `none`", "Agent identity: `builder[bot]`")
            .replace("Run ID: `none`", "Run ID: `run-123`")
            .replace("Base SHA: `none`", "Base SHA: `" + "a" * 40 + "`")
            .replace('"secrets": "none"', '"secrets": "repository"')
            .replace('"merge": false', '"merge": true')
        )
        self.assertEqual(codes(body), {"REFUSE_SECRET_ACCESS", "REFUSE_AGENT_SELF_APPROVAL"})

    def test_agent_capabilities_must_match_template(self) -> None:
        body = (
            VALID_HUMAN.replace("[x] Human-authored", "[ ] Human-authored")
            .replace("[ ] Agent-authored", "[x] Agent-authored")
            .replace("Agent identity: `none`", "Agent identity: `builder[bot]`")
            .replace("Run ID: `none`", "Run ID: `run-123`")
            .replace("Base SHA: `none`", "Base SHA: `" + "a" * 40 + "`")
            .replace('"network": "none"', '"network": true')
        )
        self.assertIn("REFUSE_CAPABILITIES_INVALID", codes(body))

    def test_shipped_template_matches_validator_contract(self) -> None:
        body = (
            TEMPLATE.read_text(encoding="utf-8")
            .replace("@YOUR_GITHUB_USERNAME", "@alice")
            .replace("- [ ] I ", "- [x] I ")
            .replace("- [ ] Human-authored", "- [x] Human-authored")
        )
        self.assertEqual(codes(body), set())

    def test_cli_accepts_a_valid_event(self) -> None:
        result = run_cli({"pull_request": {"body": VALID_HUMAN}})

        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("Human review is still required", result.stdout)

    def test_solo_mode_requires_configured_maintainer(self) -> None:
        owner = owner_event()
        enabled = ("--mode", "solo", "--solo-maintainer-id", "1")
        cases = {
            "personal owner": (owner, 0),
            "organization maintainer": (
                owner_event(owner_id=200, owner_type="Organization"),
                0,
            ),
            "wrong account": (owner_event(author_id=2), 1),
            "bot": (owner_event(author_type="Bot"), 1),
            "wrong repository": (owner_event(event_repository_id=101), 1),
            "different sender": (owner_event(sender_id=2), 1),
            "bot sender": (owner_event(sender_type="Bot"), 1),
            "fork": (owner_event(head_repository_id=101), 1),
        }
        for name, (event, expected_code) in cases.items():
            with self.subTest(name=name):
                result = run_cli(event, *enabled)
                self.assertEqual(result.returncode, expected_code, result.stdout + result.stderr)

        for invalid_id in ("", "0", "-1", "+1", "01", " 1 ", "owner"):
            with self.subTest(invalid_id=invalid_id):
                rejected = run_cli(owner, "--mode", "solo", "--solo-maintainer-id", invalid_id)
                self.assertEqual(rejected.returncode, 1, rejected.stdout + rejected.stderr)

    def test_contributor_mode_exempts_nobody(self) -> None:
        for mode in ("contributor", "invalid"):
            with self.subTest(mode=mode):
                result = run_cli(owner_event(), "--mode", mode, "--solo-maintainer-id", "1")
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)

    def test_team_mode_exempts_trusted_same_repository_authors(self) -> None:
        for association in ("OWNER", "MEMBER", "COLLABORATOR"):
            with self.subTest(association=association):
                result = run_cli(
                    owner_event(
                        author_association=association,
                        owner_id=200,
                        owner_type="Organization",
                    ),
                    "--mode",
                    "team",
                )
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

        refused = (
            owner_event(author_association="NONE", owner_id=200, owner_type="Organization"),
            owner_event(author_association="MEMBER", head_repository_id=101),
            owner_event(author_association="MEMBER", sender_id=2),
            owner_event(author_association="MEMBER", author_type="Bot"),
        )
        for event in refused:
            result = run_cli(event, "--mode", "team")
            self.assertEqual(result.returncode, 1, result.stdout + result.stderr)


class ConfigureRepositoryTests(unittest.TestCase):
    def test_mode_protection_presets(self) -> None:
        expected_reviews = {
            "solo": (0, False, False, False),
            "contributor": (1, True, True, False),
            "team": (1, True, True, True),
        }
        for mode, reviews in expected_reviews.items():
            with self.subTest(mode=mode):
                settings = protection(mode)
                review_settings = settings["required_pull_request_reviews"]
                self.assertEqual(
                    (
                        review_settings["required_approving_review_count"],
                        review_settings["require_code_owner_reviews"],
                        review_settings["dismiss_stale_reviews"],
                        review_settings["require_last_push_approval"],
                    ),
                    reviews,
                )
                self.assertEqual(
                    settings["required_status_checks"],
                    {"strict": True, "contexts": ["test-validator", "validate-metadata"]},
                )
                self.assertFalse(settings["enforce_admins"])
                self.assertFalse(settings["allow_force_pushes"])
                self.assertFalse(settings["allow_deletions"])
                self.assertTrue(settings["required_conversation_resolution"])


if __name__ == "__main__":
    unittest.main(verbosity=2)
