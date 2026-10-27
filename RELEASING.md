# Releasing Sea Noise

Releases are explicit, signed, and built from a clean `main` commit.

## Before tagging

1. Update `CHANGELOG.md` with the user-visible changes.
2. Set the CLI version in `cmd/sea-noise/main.go`.
3. Run the complete local gate:

   ```sh
   ./scripts/ci.sh
   ```

4. Review the exact diff and confirm no vault data, credentials, local paths, or generated scratch files are present.
5. Commit the release change with a DCO sign-off.

## Tag and publish

Use an annotated semantic-version tag:

```sh
git switch main
git pull --ff-only
git tag -a v0.1.0 -m "Sea Noise v0.1.0"
git push origin main
git push origin v0.1.0
```

The tag is the release identifier. Build artifacts may be attached to the corresponding GitHub release after the tag is visible on the remote.

## Reproducible local artifacts

Build a static binary for the current platform:

```sh
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o sea-noise ./cmd/sea-noise
shasum -a 256 sea-noise
```

Cross-compile only from a clean checkout and record the target tuple and checksum with the release artifacts. Do not publish a binary built from an uncommitted tree.

## Rollout and rollback

Before enabling a release against a real vault:

1. Build from the tagged commit with `CGO_ENABLED=0`.
2. Record the binary checksum and verify `sea-noise --version`.
3. Run the first-run sequence against a disposable vault.
4. Snapshot the real vault, configuration, and retrieval index.
5. Process one controlled inbox note and inspect the complete diff.
6. Enable the daemon only after the canary is clean; keep other schedulers disabled.

To upgrade, stop the daemon, save the current binary and index, replace the
binary, verify its version, rebuild or validate the index, and restart with the
same explicit flags. If vault mutations are wrong, stop the daemon, restore the
vault snapshot and previous binary, and inspect the processing log before
retrying. Do not restore by moving a published tag; publish a new patch release
for defective code.

Sea Noise is currently a Go-binary distribution. There is no npm release
artifact or Node runtime to upgrade.

## Rollback

Do not move or overwrite a published tag. If a release is defective, publish a new patch version and explain the correction in `CHANGELOG.md`.
