# Release recipes

# Release binaries using goreleaser + deploy webapp
release: tag-current-version
    REGIONS_COMMA_SEPARATED="{{regions_comma}}" goreleaser release --clean
    just deploy-production-webapp

# Test goreleaser configuration (snapshot build, no release)
# Creates a snapshot build without creating a GitHub release
release-snapshot:
    REGIONS_COMMA_SEPARATED="{{regions_comma}}" goreleaser release --snapshot --clean

# Tag HEAD with current version
tag-current-version:
    git tag {{version}}
