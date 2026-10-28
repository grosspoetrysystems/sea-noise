#!/usr/bin/env python3

import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))

from risk_labels import (  # noqa: E402
    FILE_CAP,
    changed_paths,
    comparison,
    derive,
    generated_patterns,
    set_labels,
    summary,
    ticked_boxes,
)


class FakeGitHub:
    def __init__(self, responses=None):
        self.responses = responses or {}
        self.calls = []

    def request(self, method, path, body=None):
        self.calls.append((method, path, body))
        return self.responses.get((method, path))


class PathMappingTests(unittest.TestCase):
    def test_rules_map_dependency_and_automation_paths(self):
        labels = derive(
            [
                "app/package.json",
                "src/go.sum",
                ".github/workflows/test.yml",
                "nested/CODEOWNERS",
                "README.md",
            ],
            [],
        )
        self.assertEqual(labels["risk:dependencies"], ["app/package.json", "src/go.sum"])
        self.assertEqual(labels["risk:automation"], [".github/workflows/test.yml", "nested/CODEOWNERS"])
        self.assertNotIn("README.md", sum(labels.values(), []))

    def test_lockfiles_match_by_name_not_by_substring(self):
        locks = ["yarn.lock", "web/pnpm-lock.yaml", "package-lock.json", "Cargo.lock", "bun.lockb"]
        lookalikes = ["src/clock.ts", "deadlock_test.go", "blocklist.json", "ui/unlock.tsx"]
        labels = derive(locks + lookalikes, [])
        self.assertEqual(sorted(labels["risk:dependencies"]), sorted(locks))

    def test_gitattributes_generated_and_vendored_patterns(self):
        patterns = generated_patterns(
            """# generated files
*.min.js linguist-generated
vendor/** linguist-vendored
*.txt linguist-generated=false
"""
        )
        labels = derive(["web/app.min.js", "vendor/lib/file.go", "notes/readme.txt"], patterns)
        self.assertEqual(labels, {"risk:generated": ["vendor/lib/file.go", "web/app.min.js"]})


class LabelSyncTests(unittest.TestCase):
    def test_exact_risk_set_removes_stale_and_leaves_other_labels(self):
        github = FakeGitHub()
        current = [
            {"name": "risk:dependencies"},
            {"name": "risk:stale"},
            {"name": "queue:awaiting-reviewer"},
            {"name": "maintainer-important"},
        ]
        set_labels(github, 7, {"risk:automation"}, current)
        self.assertEqual(
            github.calls,
            [
                ("DELETE", "issues/7/labels/risk%3Adependencies", None),
                ("DELETE", "issues/7/labels/risk%3Astale", None),
                ("POST", "issues/7/labels", {"labels": ["risk:automation"]}),
            ],
        )

    def test_unknown_file_data_does_not_remove_existing_risk_labels(self):
        github = FakeGitHub()
        set_labels(
            github,
            7,
            set(),
            [{"name": "risk:dependencies"}, {"name": "queue:awaiting-author"}],
            complete=False,
        )
        self.assertEqual(github.calls, [])


class UnknownDataTests(unittest.TestCase):
    def test_truncated_changed_file_data_is_unknown(self):
        github = FakeGitHub()
        files = [{"filename": f"file-{number}.txt"} for number in range(FILE_CAP)]
        github.responses[("GET", "pulls/9/files?per_page=100")] = files
        self.assertEqual(changed_paths(github, 9, {"changed_files": FILE_CAP})[1], True)

    def test_missing_changed_file_data_is_unknown(self):
        github = FakeGitHub({("GET", "pulls/9/files?per_page=100"): None})
        self.assertEqual(changed_paths(github, 9, {"changed_files": 1}), ([], True))


class SummaryTests(unittest.TestCase):
    BODY = """## Risk

- [x] Dependencies or lockfiles
- [ ] Authentication, authorization, secrets, or other security-sensitive code
- [ ] GitHub Actions, build, release, policy, or agent instructions
- [ ] Generated, vendored, minified, or binary content
- [ ] None of the above

Details and mitigations:
"""

    def test_ticked_boxes_use_validate_pr_section_parser(self):
        self.assertEqual(ticked_boxes(self.BODY), ["Dependencies or lockfiles"])

    def test_summary_records_box_comparison(self):
        report = summary({"risk:dependencies": ["package.json"]}, self.BODY, False)
        self.assertIn("Paths and the author's Risk boxes agree.", report)
        self.assertIn("`risk:dependencies`", report)
        self.assertIn("`package.json`", report)

    def test_summary_calls_out_a_difference_and_unknown_data(self):
        report = summary({"risk:automation": [".github/workflows/ci.yml"]}, self.BODY, True)
        self.assertIn("Differ", report)
        self.assertIn("Unknown file data", report)
        self.assertIn("absence of a label is not a finding", report)


if __name__ == "__main__":
    unittest.main()
