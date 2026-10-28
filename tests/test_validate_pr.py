#!/usr/bin/env python3

import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))

from configure_repository import baseline, protection, tag_ruleset
from validate_pr import _metadata_exempt, validate_body, validate_signoff

SCRIPT = ROOT / "scripts" / "validate_pr.py"
TEMPLATE = ROOT / ".github" / "pull_request_template.md"


VALID = """## Change

Small fix.

## Verification

Ran the check.

## Risk

- [x] None of the above
"""


def run_cli(event: dict[str, object], *arguments: str) -> subprocess.CompletedProcess[str]:
    with tempfile.TemporaryDirectory() as directory:
        event_path = Path(directory) / "event.json"
        event_path.write_text(json.dumps(event), encoding="utf-8")
        env = os.environ.copy()
        env.pop("GITHUB_TOKEN", None)
        env.pop("GITHUB_REPOSITORY", None)
        return subprocess.run(
            [sys.executable, str(SCRIPT), str(event_path), *arguments],
            check=False,
            capture_output=True,
            text=True,
            env=env,
        )


def owner_event(
    *,
    author_id: int = 1,
    author_type: str = "User",
    author_login: str = "alice",
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
            "user": {"id": author_id, "login": author_login, "type": author_type},
            "base": {"repo": {"id": 100, "owner": {"id": owner_id, "type": owner_type}}},
            "head": {"repo": {"id": head_repository_id}},
        },
    }


def codes(body: str) -> set[str]:
    return {refusal.code for refusal in validate_body(body)}


class ValidateBodyTests(unittest.TestCase):
    def test_minimal_submission_passes(self) -> None:
        self.assertEqual(codes(VALID), set())

    def test_each_missing_section_is_named(self) -> None:
        for section in ("Change", "Verification", "Risk"):
            with self.subTest(section=section):
                body = VALID.replace(f"## {section}", "## Other")
                refusals = validate_body(body)
                self.assertIn("REFUSE_TEMPLATE_INCOMPLETE", {r.code for r in refusals})
                self.assertIn(f"## {section}", " ".join(r.message for r in refusals))

    def test_unfilled_template_is_refused_with_specific_fixes(self) -> None:
        self.assertEqual(
            codes(TEMPLATE.read_text(encoding="utf-8")),
            {"REFUSE_CHANGE_EMPTY", "REFUSE_VERIFICATION_EMPTY", "REFUSE_RISK_UNCHECKED"},
        )

    def test_filled_template_passes(self) -> None:
        body = (
            TEMPLATE.read_text(encoding="utf-8")
            .replace("Closes #", "Fixes the queue label.\n\nCloses #12")
            .replace("<commands and results; say what you couldn't run or test. Project CI verifies independently.>", "unittest: 40 passed")
            .replace("- [ ] None of the above", "- [x] None of the above")
        )
        self.assertEqual(codes(body), set())

    def test_removed_ceremony_is_not_required(self) -> None:
        for field in ("Accountable contributor", "I reviewed the complete diff", "Human-authored", "Capabilities", "Run ID"):
            with self.subTest(field=field):
                self.assertNotIn(field, TEMPLATE.read_text(encoding="utf-8"))
        self.assertEqual(codes(VALID), set())

    def test_cli_accepts_a_valid_event(self) -> None:
        result = run_cli({"pull_request": {"body": VALID}})

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

    def test_team_mode_uses_permission_lookup_not_author_association(self) -> None:
        lookup = lambda login: login == "alice"
        for association in ("OWNER", "MEMBER", "COLLABORATOR", "CONTRIBUTOR"):
            with self.subTest(association=association):
                event = owner_event(
                    author_association=association,
                    owner_id=200,
                    owner_type="Organization",
                )
                self.assertTrue(_metadata_exempt(event, "team", 0, lookup))

        outsider = owner_event(
            author_association="CONTRIBUTOR",
            owner_id=200,
            owner_type="Organization",
            author_login="outsider",
        )
        self.assertFalse(_metadata_exempt(outsider, "team", 0, lookup))

        refused = (
            owner_event(author_association="MEMBER", head_repository_id=101),
            owner_event(author_association="MEMBER", sender_id=2),
            owner_event(author_association="MEMBER", author_type="Bot"),
        )
        for event in refused:
            self.assertFalse(_metadata_exempt(event, "team", 0, lookup))


class RepositoryBotTests(unittest.TestCase):
    def bot_event(self, login: str = "dependabot[bot]", head_repository_id: int = 100) -> dict[str, object]:
        event = owner_event(author_type="Bot", sender_type="Bot", head_repository_id=head_repository_id)
        event["pull_request"]["user"]["login"] = login
        event["sender"]["login"] = login
        return event

    def test_any_bot_on_a_repository_branch_is_admitted_without_a_list(self) -> None:
        for login in ("dependabot[bot]", "renovate[bot]"):
            result = run_cli(self.bot_event(login=login))
            self.assertEqual(result.returncode, 0, result.stdout)
            self.assertIn("authorized by its write access", result.stdout)

    def test_bot_on_a_fork_or_acting_for_another_sender_is_refused(self) -> None:
        fork = self.bot_event(head_repository_id=101)
        relayed = self.bot_event()
        relayed["sender"]["login"] = "someone-else[bot]"
        for event in (fork, relayed):
            event["pull_request"]["body"] = VALID
            result = run_cli(event)
            self.assertEqual(result.returncode, 1, result.stdout)
            self.assertIn("REFUSE_BOT_UNAUTHORIZED", result.stdout)

    def test_a_person_named_like_a_bot_is_not_exempt(self) -> None:
        event = owner_event(author_association="NONE")
        event["pull_request"]["user"]["login"] = event["sender"]["login"] = "dependabot[bot]"
        self.assertEqual(run_cli(event).returncode, 1)


def commit(sha: str, email: str, message: str, parents: int = 1) -> dict[str, object]:
    return {"sha": sha, "parents": [{}] * parents, "commit": {"author": {"email": email}, "message": message}}


class SignoffTests(unittest.TestCase):
    def test_each_commit_needs_its_authors_signoff(self) -> None:
        signed = commit("aaaaaaa1", "Ann@Example.com", "fix\n\nSigned-off-by: Ann <ann@example.com>\n")
        other = commit("bbbbbbb2", "bob@example.com", "fix\n\nSigned-off-by: Ann <ann@example.com>")
        prose = commit("ccccccc3", "cat@example.com", "says Signed-off-by: Cat <cat@example.com> inline")
        merge = commit("ddddddd4", "web@github.com", "Merge branch main", parents=2)
        self.assertEqual(validate_signoff([signed, merge]), [])
        refusal, = validate_signoff([[signed, other], [prose]])  # --slurp pages
        self.assertEqual(refusal.code, "REFUSE_SIGNOFF_MISSING")
        self.assertIn("bbbbbbb, ccccccc", refusal.message)
        self.assertNotIn("aaaaaaa", refusal.message)

    def test_cli_checks_signoff_only_when_commits_are_given(self) -> None:
        event = owner_event(author_association="NONE", author_id=2, sender_id=2)
        event["pull_request"]["body"] = VALID
        self.assertEqual(run_cli(event).returncode, 0)
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "commits.json"
            path.write_text(json.dumps([[commit("eeeeeee5", "x@example.com", "unsigned")]]), encoding="utf-8")
            result = run_cli(event, "--commits", str(path))
        self.assertEqual(result.returncode, 1)
        self.assertIn("REFUSE_SIGNOFF_MISSING", result.stdout)


class OpenPullRequestLimitTests(unittest.TestCase):
    def run_with(self, association: str, open_prs: int, limit: str = "2") -> subprocess.CompletedProcess[str]:
        event = owner_event(author_association=association, author_id=2, sender_id=2)
        event["pull_request"]["body"] = VALID
        return run_cli(event, "--open-prs", str(open_prs), "--open-pr-limit", limit)

    def test_outside_contributors_are_held_to_the_limit(self) -> None:
        self.assertEqual(self.run_with("NONE", 2).returncode, 0)
        result = self.run_with("FIRST_TIME_CONTRIBUTOR", 3)
        self.assertEqual(result.returncode, 1)
        self.assertIn("REFUSE_OPEN_PR_LIMIT", result.stdout)

    def test_collaborator_association_does_not_bypass_without_permission_lookup(self) -> None:
        result = self.run_with("COLLABORATOR", 9)
        self.assertEqual(result.returncode, 1)
        self.assertIn("REFUSE_OPEN_PR_LIMIT", result.stdout)
        self.assertEqual(self.run_with("NONE", 9, limit="0").returncode, 0)


class ShapeTests(unittest.TestCase):
    def run_with(self, association: str = "NONE", *arguments: str, **pull_request: object) -> subprocess.CompletedProcess[str]:
        event = owner_event(author_association=association, author_id=2, sender_id=2)
        event["repository"]["default_branch"] = "main"
        event["pull_request"]["base"]["ref"] = pull_request.pop("base_ref", "main")
        event["pull_request"].update({"body": VALID, **pull_request})
        return run_cli(event, *arguments)

    def test_drafts_skip_admission(self) -> None:
        event = owner_event(author_association="NONE", author_id=2, sender_id=2)
        event["pull_request"]["draft"] = True
        self.assertEqual(run_cli(event).returncode, 0)  # empty body: checked once it's ready for review

    def test_outside_contributions_target_the_default_branch(self) -> None:
        self.assertEqual(self.run_with().returncode, 0)
        result = self.run_with(base_ref="release/1.x")
        self.assertEqual(result.returncode, 1)
        self.assertIn("REFUSE_BASE_BRANCH", result.stdout)

    def test_size_limits_count_lines_and_files_and_zero_turns_them_off(self) -> None:
        self.assertEqual(self.run_with(additions=4000, deletions=1000, changed_files=300).returncode, 0)
        for size in ({"additions": 4000, "deletions": 1001}, {"changed_files": 301}):
            result = self.run_with(**size)
            self.assertEqual(result.returncode, 1, size)
            self.assertIn("REFUSE_TOO_LARGE", result.stdout)
        self.assertEqual(self.run_with("NONE", "--max-changed-lines", "0", additions=9999).returncode, 0)
        result = self.run_with("NONE", additions=9999)
        self.assertEqual(result.returncode, 1)
        self.assertIn("REFUSE_TOO_LARGE", result.stdout)


class ConfigureRepositoryTests(unittest.TestCase):
    def test_workflows_get_read_only_tokens_and_cannot_approve(self) -> None:
        calls = {endpoint: payload for _, endpoint, payload in baseline("o/r")}
        self.assertEqual(
            calls["repos/o/r/actions/permissions/workflow"],
            {"default_workflow_permissions": "read", "can_approve_pull_request_reviews": False},
        )
        self.assertEqual(
            calls["repos/o/r/actions/permissions/fork-pr-contributor-approval"],
            {"approval_policy": "first_time_contributors"},
        )

    def test_extra_checks_are_required_alongside_the_defaults(self) -> None:
        contexts = protection("solo", ("checks",))["required_status_checks"]["contexts"]
        self.assertEqual(contexts, ["test-validator", "validate-metadata", "checks"])

    def test_release_tags_cannot_move_or_be_deleted(self) -> None:
        ruleset = tag_ruleset()
        self.assertEqual((ruleset["target"], ruleset["enforcement"]), ("tag", "active"))
        self.assertEqual(ruleset["conditions"]["ref_name"]["include"], ["refs/tags/v*"])
        self.assertEqual({rule["type"] for rule in ruleset["rules"]}, {"update", "deletion"})

    def test_mode_protection_presets(self) -> None:
        expected_reviews = {
            "solo": (0, False, False, False),
            "contributor": (1, True, True, True),
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
                self.assertEqual(settings["enforce_admins"], mode == "team")
                self.assertFalse(settings["allow_force_pushes"])
                self.assertFalse(settings["allow_deletions"])
                self.assertTrue(settings["required_conversation_resolution"])


if __name__ == "__main__":
    unittest.main(verbosity=2)
