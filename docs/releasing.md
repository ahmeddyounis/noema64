# Releasing Noema64

Noema64 publishes draft GitHub Releases through `.github/workflows/release.yml`.

The current release workflow builds unsigned desktop artifacts for:

- macOS: `noema64-<tag>-macos-unsigned.zip`
- Windows: `noema64-<tag>-windows-unsigned.zip`
- Checksums: `SHA256SUMS.txt`

The release is created as a draft so maintainers can inspect assets and notes before publishing.

## Trigger A Release

Create and push a version tag:

```sh
git tag v1.0.0
git push origin v1.0.0
```

Or run the `Release` workflow manually in GitHub Actions and enter a tag such as `v1.0.0`.

## What The Workflow Does

1. Checks out the repository.
2. Installs Go and Node.js.
3. Installs the Wails CLI version used by this repo.
4. Runs Go tests for non-GUI packages and frontend smoke tests.
5. Builds the Wails GUI on native macOS and Windows runners.
6. Packages the app outputs as unsigned zip archives.
7. Generates SHA-256 checksums.
8. Creates a draft GitHub Release and attaches the artifacts.

## Unsigned Artifact Caveats

These artifacts are not code signed or notarized.

- macOS users may see Gatekeeper warnings.
- Windows users may see SmartScreen warnings.

Production signing can be added later with Apple Developer ID signing, macOS notarization, and Windows Authenticode or Trusted Signing.
