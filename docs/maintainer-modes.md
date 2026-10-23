# Maintainer operating modes

Sea Noise uses Helicopter's trusted-project and external-contribution separation. Choose one repository mode:

| Mode | Use when | Metadata exemption | Protected-branch reviews |
| --- | --- | --- | --- |
| `solo` | One human owns or maintains the project | The configured immutable user ID, on a same-repository branch | Zero required approvals |
| `contributor` | One or more maintainers primarily receive external contributions | None; every pull request follows the contribution contract | One approval and CODEOWNER review |
| `team` | An internal organization team builds in the open and also accepts external contributions | Same-repository authors GitHub identifies as `OWNER`, `MEMBER`, or `COLLABORATOR` | One approval, CODEOWNER review, and approval after the latest push |

Unset or invalid mode configuration fails closed to `contributor`.

## Configure or change mode

Authenticate an administrator, ensure the default branch exists, and run:

```sh
gh auth status
python3 scripts/configure_repository.py MODE --repo OWNER/REPOSITORY
```

Replace `MODE` with `solo`, `contributor`, or `team`. The configurator:

1. protects the default branch;
2. requires up-to-date `test-validator` and `validate-metadata` checks;
3. blocks force pushes and branch deletion;
4. requires resolved conversations;
5. applies the review policy for the selected mode;
6. sets the repository-scoped `HELICOPTER_MODE` variable;
7. sets `HELICOPTER_SOLO_MAINTAINER_ID` to the authenticated user's immutable ID only in `solo` mode, and deletes it in other modes.

Run the configurator again to change modes. Do not set these mode variables at organization scope: GitHub merges organization variables into the same `vars` context, which could affect every inheriting repository.

Required checks must have run successfully before making them required. During first installation, open valid and invalid draft pull requests before running the configurator.

## Trust decisions

### Solo

The exemption applies only when:

1. `HELICOPTER_SOLO_MAINTAINER_ID` is a positive decimal GitHub user ID;
2. the pull-request author and event sender are that user;
3. the author, sender, and repository owner are human users for a personal repository, or the repository belongs to an organization;
4. the event repository and pull-request base and head repositories have the same immutable repository ID.

Other users, bots, and forks follow the external contribution contract.

### Contributor

Nobody bypasses contribution metadata. This is the conservative mode for a maintainer-led open source project. External contributors cannot merge their own work; protected paths require CODEOWNER review.

Repository administrators retain emergency bypass because branch protection is not enforced for administrators. Use that authority deliberately.

### Team

GitHub's server-supplied `author_association` identifies trusted repository relationships. Sea Noise accepts the internal path only when:

1. the author association is `OWNER`, `MEMBER`, or `COLLABORATOR`;
2. the author and event sender are the same human user;
3. the pull request uses a branch in the governed repository, not a fork.

The same-repository requirement matters: organization membership alone does not grant the exemption. An internal coworker using a fork follows the external path. A collaborator trusted with repository write access is treated as internal even if they are not an employee.

External contributors, bots, and fork pull requests always follow the full contribution contract.

## Shared protection policy

All modes:

- require the exact `test-validator` and `validate-metadata` check contexts;
- require branches to be up to date;
- require resolved review conversations;
- block force pushes and branch deletion;
- leave administrator enforcement disabled for emergency recovery;
- keep production deploy and release triggers explicit.

`solo` deliberately requires no approval because no second reviewer exists. `contributor` requires one approval but does not require approval after the latest push, allowing a maintainer to complete review after contributor updates. `team` requires approval after the latest push so one coworker cannot both supply the final update and final approval.

Inspect the effective classic branch protection:

```sh
gh api repos/OWNER/REPOSITORY/branches/BRANCH/protection \
  --jq '{
    strict: .required_status_checks.strict,
    checks: .required_status_checks.contexts,
    admin_enforced: .enforce_admins.enabled,
    approvals: .required_pull_request_reviews.required_approving_review_count,
    codeowners: .required_pull_request_reviews.require_code_owner_reviews,
    last_push: .required_pull_request_reviews.require_last_push_approval,
    conversations: .required_conversation_resolution.enabled,
    force_pushes: .allow_force_pushes.enabled,
    deletions: .allow_deletions.enabled
  }'
```

Repository rulesets and organization rulesets may add stricter controls:

```sh
gh api repos/OWNER/REPOSITORY/rulesets
gh api repos/OWNER/REPOSITORY/rules/branches/BRANCH
```

## Trusted intake

The intake workflow uses `pull_request_target` and loads its validator, review rules (`.pr_agent.toml`), and queue script from `github.event.repository.default_branch`. No job checks out or executes pull-request code; per-job permissions and the one optional secret are listed in [Protect the policy plane](../MAINTAINERS.md#protect-the-policy-plane). The mode variable is read at run time, but workflow changes must reach the protected default branch before the new behavior exists.

The exemption returns success before parsing the pull-request body. It does not auto-check attestations or invent provenance. Project builds and tests remain in an unprivileged `pull_request` workflow.

## Direct pushes and releases

A direct administrator push is appropriate only when an administrator has reviewed a small change locally and accepts that required checks run after it reaches the default branch. This is emergency or solo-maintainer authority, not proof that checks ran before landing.

Do not attach production publication to every default-branch push merely because administrators can bypass protection:

```text
push or merge to main  -> CI only
explicit site deploy   -> production site/landing publication
version tag or release -> CLI/package publication
```

## Stacked pull requests

GitHub can identify dependent pull requests as a stack. The normal GraphQL merge mutation may require the asynchronous REST merge endpoint:

```text
PUT /repos/OWNER/REPOSITORY/pulls/NUMBER/merge-async
GET /repos/OWNER/REPOSITORY/pulls/NUMBER/merge-async/UUID
```

A `202` only enqueues the merge. Poll until the result is `merged` or `failed`. The async endpoint has no administrator-override parameter, so required checks still apply.

## References

- [About protected branches](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches)
- [About CODEOWNERS](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners)
- [REST API endpoints for branch protection](https://docs.github.com/en/rest/branches/branch-protection)
- [REST API endpoints for repository variables](https://docs.github.com/en/rest/actions/variables)
