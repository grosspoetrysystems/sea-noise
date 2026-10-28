import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "scripts"))

from check_install import file_findings, settings_findings, workflow_problems  # noqa: E402
from configure_repository import QUEUE_LABELS, RISK_LABELS, TAG_RULESET  # noqa: E402

ROOT = Path(__file__).resolve().parents[1]
SHA = "3d3c42e5aac5ba805825da76410c181273ba90b1"
DIGEST = "31b9aac6ab067bada9a0c1b001d30ebc500ea4487e00a93c18c59b9ef5fa66db"
REPO = "owner/project"


def workflow(trigger: str, steps: str) -> str:
    return f"on:\n  {trigger}:\njobs:\n  job:\n    runs-on: ubuntu-latest\n    steps:\n{steps}"


TRUSTED_CHECKOUT = (
    f"      - uses: actions/checkout@{SHA}\n"
    "        with:\n"
    "          ref: ${{ github.event.repository.default_branch }}\n"
)


class WorkflowInvariants(unittest.TestCase):
    def test_pinned_actions_and_trusted_checkout_pass(self):
        steps = TRUSTED_CHECKOUT + f"      - uses: docker://pragent/pr-agent@sha256:{DIGEST}\n"
        self.assertEqual(workflow_problems("w.yml", workflow("pull_request_target", steps)), [])

    def test_tag_pinned_action_fails(self):
        problems = workflow_problems("w.yml", workflow("pull_request", "      - uses: actions/checkout@v4\n"))
        self.assertEqual(len(problems), 1)
        self.assertIn("not pinned", problems[0])

    def test_unpinned_image_fails(self):
        steps = "      - uses: docker://pragent/pr-agent:latest\n"
        self.assertTrue(workflow_problems("w.yml", workflow("pull_request", steps)))

    def test_privileged_checkout_without_trusted_ref_fails(self):
        steps = f"      - uses: actions/checkout@{SHA}\n      - run: echo hi\n"
        problems = workflow_problems("w.yml", workflow("pull_request_target", steps))
        self.assertTrue(any("privileged checkout" in p for p in problems))

    def test_bare_checkout_passes_only_where_github_picks_the_default_branch(self):
        bare = f"      - uses: actions/checkout@{SHA}\n        with:\n          persist-credentials: false\n"
        self.assertEqual(workflow_problems("w.yml", workflow("workflow_run", bare)), [])
        self.assertEqual(workflow_problems("w.yml", workflow("issue_comment", bare)), [])
        other_ref = f"      - uses: actions/checkout@{SHA}\n        with:\n          ref: ${{{{ github.event.workflow_run.head_branch }}}}\n"
        self.assertTrue(workflow_problems("w.yml", workflow("workflow_run", other_ref)))

    def test_privileged_reference_to_pr_head_fails(self):
        steps = TRUSTED_CHECKOUT + "      - run: git fetch origin ${{ github.event.pull_request.head.sha }}\n"
        problems = workflow_problems("w.yml", workflow("issue_comment", steps))
        self.assertTrue(any("pull request head" in p for p in problems))

    def test_unprivileged_workflow_may_check_out_pr_code(self):
        steps = f"      - uses: actions/checkout@{SHA}\n"
        self.assertEqual(workflow_problems("w.yml", workflow("pull_request", steps)), [])

    def test_repository_workflows_pass(self):
        root = ROOT / ".github/workflows"
        problems = [p for f in root.glob("*.y*ml") for p in workflow_problems(f.name, f.read_text())]
        self.assertEqual(problems, [])


def configured(mode: str = "contributor") -> dict:
    """API responses for a repository the configurator set up in `mode`."""
    reviews = mode != "solo"
    variables = [{"name": "HELICOPTER_MODE", "value": mode}]
    if mode == "solo":
        variables.append({"name": "HELICOPTER_SOLO_MAINTAINER_ID", "value": "42"})
    enabled = {"status": "enabled"}
    return {
        f"repos/{REPO}/actions/variables?per_page=100": (200, {"variables": variables}),
        f"repos/{REPO}": (200, {
            "default_branch": "main",
            "web_commit_signoff_required": True,
            "security_and_analysis": {
                "secret_scanning": enabled, "secret_scanning_push_protection": enabled, "dependabot_security_updates": enabled,
            },
        }),
        f"repos/{REPO}/branches/main/protection": (200, {
            "required_status_checks": {"strict": True, "contexts": ["test-validator", "validate-metadata", "build"]},
            "enforce_admins": {"enabled": mode == "team"},
            "required_pull_request_reviews": {
                "required_approving_review_count": int(reviews), "dismiss_stale_reviews": reviews,
                "require_code_owner_reviews": reviews, "require_last_push_approval": reviews,
            },
            "allow_force_pushes": {"enabled": False},
            "allow_deletions": {"enabled": False},
            "required_conversation_resolution": {"enabled": True},
        }),
        f"repos/{REPO}/actions/permissions/workflow": (
            200, {"default_workflow_permissions": "read", "can_approve_pull_request_reviews": False}
        ),
        f"repos/{REPO}/actions/permissions/fork-pr-contributor-approval": (200, {"approval_policy": "first_time_contributors"}),
        f"repos/{REPO}/labels?per_page=100": (200, [{"name": name} for name in (*QUEUE_LABELS, *RISK_LABELS)]),
        f"repos/{REPO}/rulesets": (200, [{"id": 7, "name": TAG_RULESET}]),
        f"repos/{REPO}/rulesets/7": (200, {"enforcement": "active", "rules": [{"type": "update"}, {"type": "deletion"}]}),
        f"repos/{REPO}/vulnerability-alerts": (204, None),
        f"repos/{REPO}/private-vulnerability-reporting": (200, {"enabled": True}),
    }


def failing(responses: dict) -> list[str]:
    findings = settings_findings(REPO, lambda endpoint: responses.get(endpoint, (404, None)))
    return [finding.item for finding in findings if finding.status == "FAIL"]


class SettingsTests(unittest.TestCase):
    def test_a_configured_repository_passes_in_every_mode(self):
        for mode in ("solo", "contributor", "team"):
            self.assertEqual(failing(configured(mode)), [], mode)

    def test_drift_is_reported_item_by_item(self):
        responses = configured("contributor")
        protection = responses[f"repos/{REPO}/branches/main/protection"][1]
        protection["required_pull_request_reviews"]["require_last_push_approval"] = False
        protection["allow_force_pushes"]["enabled"] = True
        responses[f"repos/{REPO}/labels?per_page=100"] = (200, [{"name": "queue:awaiting-author"}])
        responses[f"repos/{REPO}"][1]["web_commit_signoff_required"] = False
        self.assertEqual(
            failing(responses),
            ["reviews for contributor mode", "no force pushes or branch deletion", "queue labels", "risk labels", "sign-off on web commits"],
        )

    def test_solo_protection_fails_a_contributor_mode_repository(self):
        responses = configured("solo")
        responses[f"repos/{REPO}/actions/variables?per_page=100"] = (200, {"variables": [{"name": "HELICOPTER_MODE", "value": "contributor"}]})
        self.assertIn("reviews for contributor mode", failing(responses))

    def test_missing_or_invalid_mode_fails(self):
        responses = configured()
        responses[f"repos/{REPO}/actions/variables?per_page=100"] = (200, {"variables": []})
        self.assertIn("mode variable", failing(responses))

    def test_unprotected_branch_fails(self):
        responses = configured()
        del responses[f"repos/{REPO}/branches/main/protection"]
        self.assertIn("branch protection", failing(responses))

    def test_unreadable_settings_need_an_administrator_rather_than_failing(self):
        responses = configured()
        responses[f"repos/{REPO}/actions/permissions/workflow"] = (403, None)
        findings = settings_findings(REPO, lambda endpoint: responses.get(endpoint, (404, None)))
        self.assertEqual([f.item for f in findings if f.status == "ADMIN"], ["workflow token"])
        self.assertNotIn("workflow token", [f.item for f in findings if f.status == "FAIL"])

    def test_web_commit_signoff_is_not_required_when_the_check_is_off(self):
        responses = configured()
        responses[f"repos/{REPO}/actions/variables?per_page=100"][1]["variables"].append(
            {"name": "HELICOPTER_SIGNOFF", "value": "off"}
        )
        responses[f"repos/{REPO}"][1]["web_commit_signoff_required"] = False
        self.assertEqual(failing(responses), [])


class FileTests(unittest.TestCase):
    def test_helicopter_itself_passes(self):
        findings = file_findings(ROOT, "grosspoetrysystems/helicopter")
        self.assertEqual([f for f in findings if f.status != "PASS"], [])

    def adopter(self, files: dict[str, str]) -> dict[str, str]:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            installed = {
                ".github/workflows/pr-contract.yml": workflow(
                    "pull_request_target", TRUSTED_CHECKOUT + "      - run: python3 trusted-base/scripts/validate_pr.py\n"
                ),
                ".github/helicopter/version.json": '{"version": "v0.2.0", "commit": "abc"}',
                ".github/helicopter/LICENSE": "MIT License\n",
            }
            for name, text in {**installed, **files}.items():
                (root / name).parent.mkdir(parents=True, exist_ok=True)
                (root / name).write_text(text)
            return {f.item: f.detail for f in file_findings(root, REPO) if f.status == "FAIL"}

    def test_a_renamed_admission_workflow_counts(self):
        self.assertEqual(self.adopter({}), {})

    def test_missing_admission_workflow_and_marker_fail(self):
        with tempfile.TemporaryDirectory() as directory:
            (Path(directory) / ".github/workflows").mkdir(parents=True)
            failed = [f.item for f in file_findings(Path(directory), REPO) if f.status == "FAIL"]
        self.assertEqual(failed, ["admission workflow", "Helicopter version marker", "Helicopter license notice"])

    def test_a_broken_marker_fails(self):
        self.assertIn("Helicopter version marker", self.adopter({".github/helicopter/version.json": "{"}))

    def test_helicopters_owners_and_dev_tooling_are_caught(self):
        failed = self.adopter({
            ".github/CODEOWNERS": "# Helicopter currently has one maintainer. Adopting projects must replace it.\n",
            ".changie.yaml": "kinds:\n  - label: Action required\n",
            "scripts/dev_checks.py": "",
            "README.md": '<img src="assets/helicopter-header.webp">\n',
        })
        self.assertEqual(set(failed), {"your own owners and contacts", "no Helicopter-only files"})
        self.assertIn(".github/CODEOWNERS", failed["your own owners and contacts"])
        for path in (".changie.yaml", "scripts/dev_checks.py", "README.md"):
            self.assertIn(path, failed["no Helicopter-only files"])

    def test_an_adopters_own_files_with_the_same_names_pass(self):
        self.assertEqual(self.adopter({
            ".github/CODEOWNERS": "# Poolboy currently has one maintainer.\n* @thekidnamedkd\n",
            "lefthook.yml": "pre-commit:\n  commands:\n    go-format:\n      run: go fmt\n",
            "README.md": "# Poolboy\n",
            ".github/workflows/release.yml": workflow("push", ""),
            ".changes/unreleased/Added-1.yaml": "kind: Added\n",
        }), {})


if __name__ == "__main__":
    unittest.main()
