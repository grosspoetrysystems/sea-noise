# Maintaining Sea Noise

Sea Noise adopts Helicopter's contribution controls. The repository files express policy; GitHub settings enforce it.

## Activate the repository

- [ ] Replace project-specific URLs, contacts, and CODEOWNERS entries when ownership changes.
- [ ] Confirm DCO is the intended legal mechanism and configure its sign-off check, or replace it with the target project's CLA/CA process.
- [ ] Decide which bot or GitHub App identities, if any, may interact with the repository. List automation that has collaborator access in the `HELICOPTER_AUTOMATION_LOGINS` repository variable so its comments do not count as maintainer responses.
- [ ] Review the two-PR limit and issue-first categories in `CONTRIBUTING.md`.
- [ ] Create or replace the `needs-triage` and `automation-review` labels used by the issue forms.
- [ ] Test the pull request template and both acceptance/refusal paths of the intake workflow.
- [ ] Optionally add a `HELICOPTER_REVIEW_KEY` secret (an OpenRouter key with a credit limit) to turn on the first review.
- [ ] Use `solo` mode for this repository, then run the repository configurator after its acceptance and refusal checks pass.
- [ ] Publish reachable private security and conduct contacts.

## GitHub settings

Run `python3 scripts/configure_repository.py MODE --repo OWNER/REPOSITORY` after the required checks have run successfully. The configurator protects the default branch, applies the selected mode, and creates the `queue:` labels. The queue fails rather than create a missing label.

All modes:

- require `test-validator` and `validate-metadata`;
- require resolved review conversations;
- block force pushes and branch deletion; and
- retain administrator bypass for emergency recovery.

`solo` requires zero approvals because no second reviewer exists. `contributor` and `team` require one approval and CODEOWNER review; `team` also requires approval after the latest push. See [maintainer operating modes](docs/maintainer-modes.md) for the trust and transition rules.

For public repositories, set the concurrent non-draft PR limit for users without write access. Start at two unless maintainer capacity supports more; add trusted contributors to the bypass list deliberately.

Enable the security features appropriate to the repository and plan: private vulnerability reporting, dependency graph/review, secret scanning and push protection, code scanning, and release attestations/SBOMs.

## Protect the policy plane

`.github/CODEOWNERS` covers Sea Noise's workflows, validator, and governance files. Add dependency manifests, lockfiles, authentication and security paths, release code, code generators, and agent instruction files to the ownership policy.

The intake and queue workflows use `pull_request_target`, `issue_comment`, and `schedule` because those events load workflows and scripts from the protected default branch and receive a write-capable token even for fork pull requests. Their safety depends on these invariants:

1. only event metadata, the GitHub API, and protected default-branch files are read; the diff is read as text, never applied;
2. the contributor head is never checked out, built, or executed;
3. the only secret is the optional model key, held by `first-review`, which runs PR-Agent pinned by image digest; and
4. write permission is job-scoped: `first-review` writes its review comment and labels, `queue` jobs write queue labels and notices and hold no secret. PR-Agent posts model text as written, so a prompt-injected diff can put mentions or links in its comment; the comment is advisory and moves nothing.

| Job | Token | Secret |
| --- | --- | --- |
| `validate-metadata` | `contents: read` | none |
| `first-review` | `contents: read`; `issues`, `pull-requests: write` | `HELICOPTER_REVIEW_KEY` |
| `queue` (both workflows) | `contents`, `checks: read`; `issues`, `pull-requests: write` | none |

Bump the PR-Agent image deliberately: resolve the new digest with `docker buildx imagetools inspect pragent/pr-agent:VERSION-github_action --format '{{.Manifest.Digest}}'` and review its release notes first.

Put builds and tests in a separate `pull_request` workflow on ephemeral GitHub-hosted runners. Pin every external action to a reviewed full commit SHA and set `persist-credentials: false` unless a job genuinely needs to push.

## Intake order

1. Check template, accountable human, origin, bot authorization, issue requirement, and PR budget.
2. Reject duplicates, out-of-scope work, and unreviewable size before expensive CI.
3. Route protected paths, dependencies, generated artifacts, and security-sensitive changes to owners.
4. Run independent tests and security checks without repository secrets.
5. Use automated or AI review only as advisory evidence.
6. Merge through the protected ruleset after human review.

Use one actionable response: one reason, one policy link, one repair path. Do not deploy several bots that repeat the same refusal.

## Review ramp

Each pull request earns the next, more expensive step by passing the cheaper one. A new push starts again from the bottom.

| Step | Cost | Output | Then |
| --- | --- | --- | --- |
| Admission (`validate-metadata`) | $0 | refusal codes, or pass | refused: contributor's turn |
| First review (`first-review`, optional) | a fraction of a cent with the default model | one PR-Agent comment: key issues, effort, tests, security, merge recommendation; `Review effort` and `possible security issue` labels | advisory: the contributor fixes what it found |
| Queue | $0 | queue label, nudge, announced pause | human review and merge |

Admission's refusal and PR-Agent's review are the only automated responses on a pull request; the queue adds a notice only when a maintainer is late.

**First review.** Runs only when the `HELICOPTER_REVIEW_KEY` secret exists, never on drafts, on opening and on every push, editing its one comment in place. `.pr_agent.toml` on the default branch holds the model (default `openrouter/deepseek/deepseek-v4.1-flash`), the cost rail (`custom_model_max_tokens`: larger pull requests are clipped to fit), the output sections, and the input rules in `extra_instructions`. Change the review by editing that file. Set a credit limit on the key with your provider. The review is advisory: it does not move the turn or block merging.

**Queue.** Every run recomputes state from the pull request timeline; labels are outputs. The clock runs only while it is a maintainer's turn and restarts whenever the turn passes to a maintainer. It is the contributor's turn after refused admission or a maintainer's **Request changes**, until the contributor pushes or replies. Any maintainer comment or review restarts the clock.

- After `HELICOPTER_QUEUE_NUDGE_DAYS` (default 7), one comment mentions the requested reviewers.
- After `HELICOPTER_QUEUE_PAUSE_DAYS` (default 21), the pull request is labelled `queue:no-capacity` and the contributor is told publicly. It stays open.
- Pull requests from maintainers (`OWNER`, `MEMBER`, `COLLABORATOR`) and drafts are not queued.

`contribution-queue.yml` reconciles every six hours and on pull request comments. Maintainer reviews on fork pull requests are picked up by the next sweep.

**Hoppers.** The first review's labels sort waiting work; no extra tooling is involved. Replace `OWNER` with the account or organization:

| Hopper | GitHub search |
| --- | --- |
| Quick wins | `org:OWNER is:pr is:open label:"queue:awaiting-reviewer" label:"Review effort 1/5","Review effort 2/5" -label:"Possible security concern"` |
| Security | `org:OWNER is:pr is:open label:"queue:awaiting-reviewer" label:"Possible security concern"` |
| Deep-review candidates | `org:OWNER is:pr is:open label:"queue:awaiting-reviewer" label:"Review effort 4/5","Review effort 5/5"` |
| Everything waiting, oldest first | `org:OWNER is:pr is:open label:"queue:awaiting-reviewer" sort:created-asc` |

Pull requests without a first review (no key configured) appear only in the last search.

## Risk and response

- **Routine:** Request a specific repair for missing fields, weak reproduction, or excessive scope.
- **Abuse:** Close duplicate, flooding, or unauthorized automation submissions, then apply PR or interaction limits.
- **Suspicious:** Quarantine dependency substitution, obfuscated files, reviewer prompt injection, or workflow escalation. Do not execute the contribution; involve security owners.
- **Incident:** Cancel runs, revoke identities, and freeze affected merges or releases after credential exposure, malicious workflow execution, a compromised bot or runner, or protected-branch compromise. Rotate possibly exposed credentials, preserve URLs, actor IDs, SHAs, run IDs, and logs, then rebuild from trusted state and use private disclosure.

Automation may triage or temporarily contain; a human must be able to reverse consequential moderation.

## Tune from evidence

Track maintainer minutes per accepted pull request, first-review latency, CI cost before triage, false refusal rate, major rewrite/revert rate, duplicate/spam rate, and repeat-contributor rate. Do not optimize for raw pull request count, generated lines, or bot comments.

Do not publish response-time promises until someone owns them. If you publish targets, call them service objectives rather than guarantees.
