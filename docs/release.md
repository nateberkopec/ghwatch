# Release Process

This document describes how to create a new release of `gogh`.

## Prerequisites

- Write access to the repository
- Clean working directory on `main` branch
- All tests passing

## Release Steps

### 1. Ensure main is ready

```bash
git checkout main
git pull origin main
go test ./...
```

All tests should pass before proceeding.

### 2. Choose a version number

Follow [Semantic Versioning](https://semver.org/):

- **MAJOR** version for incompatible API changes
- **MINOR** version for new functionality in a backwards compatible manner
- **PATCH** version for backwards compatible bug fixes

Examples: `v1.0.0`, `v1.1.0`, `v1.1.1`

### 3. Create and push a tag

```bash
# Create an annotated tag
git tag -a v1.0.0 -m "Release v1.0.0"

# Push the tag to GitHub
git push origin v1.0.0
```

### 4. Monitor the release workflow

The GitHub Actions workflow will automatically:

1. Run all tests
2. Build binaries for multiple platforms (Linux, macOS, Windows)
3. Create a GitHub release with:
   - Changelog generated from commits since the last tag
   - Pre-built binaries for all supported platforms
   - SHA256 checksums

Visit the Actions tab to monitor progress: https://github.com/nateberkopec/ghwatch/actions

### 5. Verify the release

Once the workflow completes:

1. Visit https://github.com/nateberkopec/ghwatch/releases
2. Verify the new release appears with all binaries
3. Check the changelog is accurate
4. Test downloading and running a binary:

```bash
# Example for macOS ARM64
curl -L https://github.com/nateberkopec/ghwatch/releases/download/v1.0.0/gogh_1.0.0_darwin_arm64.tar.gz | tar xz
./gogh --help
```

## Supported Platforms

GoReleaser builds binaries for:

- **macOS**: amd64 (Intel), arm64 (Apple Silicon)
- **Linux**: amd64, arm64
- **Windows**: amd64, arm64

All binaries are statically linked (CGO_ENABLED=0) for maximum portability.

## Troubleshooting

### Release workflow fails

Check the Actions tab for detailed error logs. Common issues:

- Tests failing: Fix the tests and create a new tag
- GoReleaser configuration error: Update `.goreleaser.yml` and create a new tag
- Permission issues: Ensure the `GITHUB_TOKEN` has `contents: write` permission

### Need to delete a tag

If you need to redo a release:

```bash
# Delete locally
git tag -d v1.0.0

# Delete remotely
git push origin :refs/tags/v1.0.0

# Delete the GitHub release manually from the web UI
```

Then recreate the tag and push again.

### Pre-release versions

To create a pre-release (e.g., `v1.0.0-beta.1`), follow the same process. GoReleaser automatically detects pre-release versions and marks them accordingly on GitHub.

## Changelog Management

The changelog is automatically generated from commit messages. To ensure quality changelogs:

- Write clear, descriptive commit messages
- Use conventional commit prefixes when appropriate
- Commits starting with `docs:`, `test:`, `ci:`, or `chore:` are excluded from the changelog
- Merge commits are excluded

## Version Information in Binaries

GoReleaser injects version information at build time using ldflags:

- `main.version`: The git tag (e.g., `v1.0.0`)
- `main.commit`: The git commit SHA
- `main.date`: The build timestamp

To use these in your code:

```go
package main

var (
    version = "dev"
    commit  = "none"
    date    = "unknown"
)

func main() {
    fmt.Printf("gogh %s (commit: %s, built: %s)\n", version, commit, date)
    // ...
}
```
