# Release recipes

# Release binaries using goreleaser + deploy webapp
release: tag-current-version
    goreleaser release --clean
    just deploy-webapp

# Test goreleaser configuration (snapshot build, no release)
# Creates a snapshot build without creating a GitHub release
release-snapshot:
    goreleaser release --snapshot --clean

# Tag HEAD with current version
tag-current-version:
    git tag {{version}}
