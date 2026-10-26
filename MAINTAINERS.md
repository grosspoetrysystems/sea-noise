# Maintaining Sea Noise

Sea Noise is maintained as a small public Go project. Keep the repository easy to build, inspect, and recover.

## Repository mode

The project uses a solo-maintainer workflow until a second maintainer is deliberately added. GitHub branch protection should require the CI and DCO checks, block force pushes and branch deletion, and retain administrator bypass only for recovery.

Do not add repository-wide bots, privileged pull-request workflows, or model-backed automation without a reviewed issue and a documented threat model.

## Required checks

Before merging or publishing:

```sh
./scripts/ci.sh
```

The CI workflow must remain unprivileged for pull requests. It must not check out or execute untrusted pull-request code in a job that has repository secrets or write permissions.

## Protected paths

Review changes to these areas carefully:

- `.github/workflows/`
- build and release scripts
- authentication, credential, or network clients
- dependency manifests
- inference skills and embedded skill mirrors
- fixture and expected-data generation
- contribution and security policy

## Public-data boundary

Never commit:

- a real vault or note export;
- local filesystem paths, private hostnames, or personal contact details;
- credentials, tokens, cookies, or command transcripts containing secrets;
- generated scratch state or private planning artifacts.

Use the synthetic fixture vault for tests. Private source material belongs in the attached Vernacular workspace, not in Git.

## Releases

Follow [`RELEASING.md`](RELEASING.md). Every release is built from a clean, reviewed commit, uses a version tag, and includes a DCO-signed commit history.
