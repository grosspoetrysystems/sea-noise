# Maintainer operating modes

Sea Noise starts in solo-maintainer mode. This document records the minimum branch-protection posture without coupling the repository to a private automation system.

## Solo mode

Use solo mode while one accountable maintainer owns the repository:

- require successful `ci` and `DCO` checks;
- allow zero required approvals;
- block force pushes and branch deletion;
- require resolved review conversations when pull requests are used; and
- keep administrator bypass available only for recovery.

Do not treat solo mode as permission to skip local verification. Every release still follows [`RELEASING.md`](../RELEASING.md).

## Adding maintainers

When the project gains a second maintainer, switch to a team policy deliberately:

- require at least one approval from a different maintainer;
- require CODEOWNERS review for workflows, release machinery, security files, and dependency changes;
- require approval after the latest push; and
- document the maintainer roster and ownership boundaries.

## Pull requests

Every pull request should identify one coherent change, include verification commands and results, and disclose material AI or automation assistance. CI runs on the pull-request revision with read-only permissions and no repository secrets.
