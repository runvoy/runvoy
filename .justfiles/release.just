# Release recipes

# Release binaries using goreleaser + deploy webapp
release: tag-current-version
    AWS_REGION=us-east-1 REGIONS_COMMA_SEPARATED="{{regions_comma}}" \
        goreleaser release --clean
    just deploy-production-webapp
    just trigger-docs-build

# Test goreleaser configuration (snapshot build, no release)
# Creates a snapshot build without creating a GitHub release
release-snapshot:
    REGIONS_COMMA_SEPARATED="{{regions_comma}}" \
        goreleaser release --snapshot --clean

# Tag HEAD with current version
tag-current-version:
    git tag {{version}}
