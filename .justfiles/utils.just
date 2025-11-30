# Utility recipes

# Clean build artifacts
clean:
    rm -rf bin/ dist/ site/ coverage.out coverage.html
    go clean
    git clean -f .

# Install pre-commit hook
install-hook:
    printf "just update-readme-help check\n" > .git/hooks/pre-commit
    chmod +x .git/hooks/pre-commit

# Update README.md with latest CLI help output
# This ensures the README stays in sync with CLI commands
update-readme-help: build-cli
    go run scripts/update-readme-help/main.go ./bin/runvoy
    just generate-cli-docs
    git add README.md docs/CLI.md

# Generate CLI documentation as a single markdown file
generate-cli-docs out_file="docs/CLI.md":
    go run scripts/generate-cli-docs/main.go -out {{out_file}}

# Update CHANGELOG.md with commits since last release
update-changelog:
    go run scripts/update-changelog/main.go

# Trigger docs site build on GitHub Actions
trigger-docs-build:
    gh workflow run build-docs.yml --repo runvoy/runvoy.github.io

# Upgrade all dependencies
upgrade-dependencies:
    go get -u all

# TODO run agg into a github action and store it as asset so to avoid
# having to commit the gif to the repository
record-demo:
    asciinema rec --overwrite runvoy-demo.cast
    agg --theme monokai runvoy-demo.cast runvoy-demo.gif

# Curl helper
curl-get path:
    curl -sS \
        -X GET "http://localhost:${RUNVOY_DEV_SERVER_PORT}/api/v1{{path}}" \
        -H "X-API-Key: ${RUNVOY_ADMIN_API_KEY}"

# Curl helper
curl-post path body:
    curl -sS \
        -X POST "http://localhost:${RUNVOY_DEV_SERVER_PORT}/api/v1{{path}}" \
        -H "X-API-Key: ${RUNVOY_ADMIN_API_KEY}" \
        -d '{{body}}'

# Helper to truncate a DynamoDB table
truncate-dynamodb-table table_name:
    go run scripts/truncate-dynamodb-table/main.go {{table_name}}

# Empty and delete buckets
delete-buckets *ARGS:
    go run scripts/delete-s3-buckets/*.go {{ARGS}}

# Fetch and print API endpoint URL from CloudFormation stack
get-endpoint stack_name:
    @aws cloudformation describe-stacks \
        --stack-name {{stack_name}} \
        --query "Stacks[0].Outputs[?OutputKey=='APIEndpoint'].OutputValue" \
        --output text

# Cleanup old branches
cleanup-branches:
    git fetch --prune
    git branch | grep -v '^\* main' | xargs git branch -D

# Print build version
print-build-version:
    @printf "{{build_version}}\n"
