# Contributing to Sea Noise

Keep changes small, reviewable, and reproducible.

## Before implementation

Open or reference an issue before starting a feature, public API change, dependency change, security change, or release/workflow change. Focused bug fixes and documentation corrections may proceed without issue-first review.

The following are enforced by checks, not by declarations:

| Rule | Enforced by | Refusal code |
| --- | --- | --- |
| The description has **Change**, **Verification**, and **Risk** sections, each filled in | `validate-metadata` (required check) | `REFUSE_TEMPLATE_INCOMPLETE`, `REFUSE_CHANGE_EMPTY`, `REFUSE_VERIFICATION_EMPTY`, `REFUSE_RISK_UNCHECKED` |
| Every commit carries `Signed-off-by` with its author's email ([DCO](#dco)); merge commits are skipped | `validate-metadata` | `REFUSE_SIGNOFF_MISSING` |
| A bot opens pull requests only from a branch of this repository, which needs write access an administrator grants | `validate-metadata` | `REFUSE_BOT_UNAUTHORIZED` |
| Contributors without write access have at most two open non-draft pull requests (drafts don't count) | `validate-metadata` | `REFUSE_OPEN_PR_LIMIT` |
| Contributors without write access target the default branch | `validate-metadata` | `REFUSE_BASE_BRANCH` |
| Contributors without write access change at most 5,000 lines and 300 files per pull request | `validate-metadata` | `REFUSE_TOO_LARGE` |
| Merging needs the required checks to pass and the reviews your mode requires; nobody can force-push or delete the default branch | Branch protection set by `configure_repository.py` | — |
| Changes to workflows, security, dependencies, release machinery, policy, or agent instructions need the owners in `.github/CODEOWNERS` (contributor and team modes) | Branch protection (code-owner review) | — |
| Helicopter reruns its own checks; your report of what you ran is context, not evidence | Project CI | — |

## Pull requests

Each pull request should:

- solve one coherent problem;
- explain the user-visible effect;
- report commands and observed results;
- call out dependency, security, workflow, and generated-data impact; and
- disclose material AI or automation assistance.

Reviewers may ask for unrelated, generated, or unreviewable changes to be split out.

## Local checks

Requirements and checks are documented in [`README.md`](README.md):

```sh
./scripts/ci.sh
```

Use [`testdata/CAPTURING-FIXTURES.md`](testdata/CAPTURING-FIXTURES.md) before changing parity fixtures. Never use a live vault as committed test data.

## DCO

Sea Noise uses the [Developer Certificate of Origin 1.1](https://developercertificate.org/). Sign off every commit:

```text
Signed-off-by: Your Name <your.email@example.com>
```

The sign-off is made by the accountable human or organization submitting the change.

## Security and conduct

Report vulnerabilities privately as described in `SECURITY.md`. Follow `CODE_OF_CONDUCT.md` in all project spaces.
