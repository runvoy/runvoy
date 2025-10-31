set dotenv-required
version := shell('cat VERSION | tr -d "\n"')
git_short_hash := shell('git rev-parse --short HEAD')
build_date := shell('date +%Y%m%d')
build_flags := "-X=runvoy/internal/constants.version="

# Aliases
alias r := runvoy

# Build the CLI binary and run it with the given arguments
[default]
runvoy *ARGS: build-cli
    ./bin/runvoy {{ARGS}}

# Build all binaries
build: build-cli build-local build-orchestrator build-event-processor

# Deploy all binaries
deploy: deploy-orchestrator deploy-event-processor deploy-webviewer

# Build CLI client
[working-directory: 'cmd/runvoy']
build-cli:
    go build \
        -ldflags "{{build_flags}}{{version}}-{{build_date}}-{{git_short_hash}}" \
        -o ../../bin/runvoy

# Build orchestrator backend service (Lambda function)
[working-directory: 'cmd/backend/aws/orchestrator']
build-orchestrator:
    GOARCH=arm64 GOOS=linux go build \
        -ldflags "{{build_flags}}{{version}}-{{build_date}}-{{git_short_hash}}" \
        -o ../../../../dist/bootstrap

# Build event processor backend service (Lambda function)
[working-directory: 'cmd/backend/aws/event_processor']
build-event-processor:
    GOARCH=arm64 GOOS=linux go build \
        -ldflags "{{build_flags}}{{version}}-{{build_date}}-{{git_short_hash}}" \
        -o ../../../../dist/bootstrap

# Build local development server
[working-directory: 'cmd/local']
build-local:
    go build \
        -ldflags "{{build_flags}}{{version}}-{{build_date}}-{{git_short_hash}}" \
        -o ../../bin/local

# Build orchestrator zip file
[working-directory: 'dist']
build-orchestrator-zip: build-orchestrator
    rm -f bootstrap.zip
    zip bootstrap.zip bootstrap

# Deploy orchestrator lambda function
[working-directory: 'dist']
deploy-orchestrator: build-orchestrator-zip
    aws s3 cp bootstrap.zip s3://${RUNVOY_RELEASES_BUCKET}/bootstrap.zip
    aws lambda update-function-code \
        --function-name runvoy-orchestrator \
        --s3-bucket ${RUNVOY_RELEASES_BUCKET} \
        --s3-key bootstrap.zip > /dev/null
    aws lambda wait function-updated --function-name runvoy-orchestrator

# Build event processor zip file
[working-directory: 'dist']
build-event-processor-zip: build-event-processor
    rm -f event-processor.zip
    zip event-processor.zip bootstrap

# Deploy event processor lambda function
[working-directory: 'dist']
deploy-event-processor: build-event-processor-zip
    aws s3 cp event-processor.zip s3://${RUNVOY_RELEASES_BUCKET}/event-processor.zip
    aws lambda update-function-code \
        --function-name runvoy-event-processor \
        --s3-bucket ${RUNVOY_RELEASES_BUCKET} \
        --s3-key event-processor.zip > /dev/null
    aws lambda wait function-updated --function-name runvoy-event-processor

# Deploy webviewer HTML to S3
deploy-webviewer:
    aws s3 cp cmd/webviewer/index.html \
        s3://${RUNVOY_RELEASES_BUCKET}/webviewer.html \
        --content-type text/html

# Run local development server
run-local: build-local
    ./bin/local

# Run all tests
test:
    go test ./...

# Run tests with coverage
test-coverage:
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
    rm -rf bin/
    go clean

# Development setup
dev-setup:
    go mod tidy
    go mod download
    go install golang.org/x/tools/cmd/goimports@latest
    pip install pre-commit # TODO: add to requirements.txt?

# Lint all Go code
lint:
    golangci-lint run

# Lint and fix Go code
lint-fix:
    golangci-lint run --fix

# Format Go code
fmt:
    go fmt ./...
    goimports -w .

# Run all checks (lint + test)
check: lint test

# Install pre-commit hooks
install-hooks:
    pre-commit install

# Run pre-commit on all files
pre-commit-all:
    pre-commit run --all-files

# Create lambda bucket
create-lambda-bucket:
    aws cloudformation deploy \
        --stack-name runvoy-releases-bucket \
        --template-file deployments/runvoy-bucket.yaml

# Create/update backend infrastructure via cloudformation
create-backend-infra:
    aws cloudformation deploy \
        --stack-name ${RUNVOY_CLOUDFORMATION_BACKEND_STACK} \
        --template-file deployments/cloudformation-backend.yaml \
        --capabilities CAPABILITY_NAMED_IAM
    just seed-admin-user

# Seed initial admin user into DynamoDB (idempotent)
# Requires:
#   - RUNVOY_ADMIN_EMAIL: admin email to associate with the API key
#   - ~/.runvoy/config.yaml containing api_key
seed-admin-user:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -z "${RUNVOY_ADMIN_EMAIL:-}" ]; then \
        echo "Error: RUNVOY_ADMIN_EMAIL environment variable is required"; \
        exit 1; \
    fi
    CONFIG_FILE="$HOME/.runvoy/config.yaml"
    if [ ! -f "$CONFIG_FILE" ]; then \
        echo "Error: config file not found at $CONFIG_FILE"; \
        exit 1; \
    fi
    API_KEY=$(grep -E "^api_key:" "$CONFIG_FILE" | head -n1 | awk -F ':' '{print $2}' | tr -d ' "\t')
    if [ -z "$API_KEY" ]; then \
        echo "Error: api_key not found in $CONFIG_FILE"; \
        exit 1; \
    fi
    API_KEY_HASH=$(printf "%s" "$API_KEY" | openssl dgst -sha256 -binary | base64)
    CREATED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    TABLE_NAME=$(aws cloudformation describe-stacks --stack-name runvoy-backend --query "Stacks[0].Outputs[?OutputKey=='APIKeysTableName'].OutputValue" --output text)
    if [ -z "$TABLE_NAME" ] || [ "$TABLE_NAME" = "None" ]; then \
        echo "Error: failed to resolve API keys table name from CloudFormation outputs"; \
        exit 1; \
    fi
    echo "Seeding admin user $RUNVOY_ADMIN_EMAIL into table $TABLE_NAME (idempotent)..."
    set +e
    aws dynamodb put-item \
        --table-name "$TABLE_NAME" \
        --item "{\"api_key_hash\":{\"S\":\"$API_KEY_HASH\"},\"user_email\":{\"S\":\"$RUNVOY_ADMIN_EMAIL\"},\"created_at\":{\"S\":\"$CREATED_AT\"},\"revoked\":{\"BOOL\":false}}" \
        --condition-expression "attribute_not_exists(api_key_hash)" >/dev/null 2>&1
    STATUS=$?
    set -e
    if [ $STATUS -eq 0 ]; then \
        echo "Admin user created."; \
    else \
        echo "Admin user already exists or condition failed; continuing."; \
    fi

# Run local development server with hot reloading
local-dev-server:
    reflex -r '\.go$' -s -- sh \
        -c 'AWS_REGION=us-east-2 \
            AWS_PROFILE=api-l3x-in \
            RUNVOY_LOG_LEVEL=DEBUG \
            RUNVOY_API_KEYS_TABLE=runvoy-api-keys \
        go run \
            -ldflags "{{build_flags}}{{version}}-{{build_date}}-{{git_short_hash}}" \
            ./cmd/local'

# Smoke test local user creation
smoke-test-local-create-user email:
    curl -sS \
        -X POST "http://localhost:${RUNVOY_DEV_SERVER_PORT}/api/v1/users/create" \
        -H "X-API-Key: ${RUNVOY_ADMIN_API_KEY}" \
        -H "Content-Type: application/json" \
        -d '{"email":"{{email}}"}' | jq .

# Smoke test local user revocation
smoke-test-local-revoke-user email:
    curl -sS \
        -X POST "http://localhost:${RUNVOY_DEV_SERVER_PORT}/api/v1/users/revoke" \
        -H "X-API-Key: ${RUNVOY_ADMIN_API_KEY}" \
        -H "Content-Type: application/json" \
        -d '{"email":"{{email}}"}' | jq .

# Smoke test local execution logs
smoke-test-local-get-logs execution_id:
    curl -sS \
        -X GET "http://localhost:${RUNVOY_DEV_SERVER_PORT}/api/v1/executions/{{execution_id}}/logs" \
        -H "X-API-Key: ${RUNVOY_ADMIN_API_KEY}" | jq .

# Smoke test backend health
smoke-test-backend-health:
    curl -sS \
        -H "X-API-Key: ${RUNVOY_ADMIN_API_KEY}" \
        -X GET "${RUNVOY_LAMBDA_URL}/api/v1/health" | jq .

# Smoke test backend user creation
smoke-test-backend-users-create:
    curl -sS \
        -H "X-API-Key: ${RUNVOY_ADMIN_API_KEY}" \
        -X POST "${RUNVOY_LAMBDA_URL}/api/v1/users/create" \
        -H "Content-Type: application/json" \
        -d '{"email":"bob@example.com"}' | jq .

# Smoke test backend command execution
smoke-test-backend-run-command command:
    curl -sS \
        -H "X-API-Key: ${RUNVOY_ADMIN_API_KEY}" \
        -X POST "${RUNVOY_LAMBDA_URL}/api/v1/run" \
        -H "Content-Type: application/json" \
        -d "{\"command\":\"{{command}}\"}" | jq .

# Smoke test local execution killing
smoke-test-local-kill-execution execution_id:
    curl -sS \
        -X POST "http://localhost:${RUNVOY_DEV_SERVER_PORT}/api/v1/executions/{{execution_id}}/kill" \
        -H "X-API-Key: ${RUNVOY_ADMIN_API_KEY}" | jq .

# TODO run agg into a github action and store it as asset so to avoid
# having to commit the gif to the repository
record-demo:
    asciinema rec --overwrite runvoy-demo.cast
    agg --theme monokai runvoy-demo.cast runvoy-demo.gif
