# Contributing to Sea Noise

Keep changes small, reviewable, and reproducible.

## Before implementation

Open or reference an issue before starting a feature, public API change, dependency change, security change, or release/workflow change. Focused bug fixes and documentation corrections may proceed without issue-first review.

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
