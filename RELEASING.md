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

## Rollback

Do not move or overwrite a published tag. If a release is defective, publish a new patch version and explain the correction in `CHANGELOG.md`.
