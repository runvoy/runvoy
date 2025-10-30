# Environment variables that can be overridden:
# RUNVOY_RELEASES_BUCKET - S3 bucket for releases (default: runvoy-releases)
# RUNVOY_ADMIN_API_KEY - Admin API key for testing (required for smoke tests)
# RUNVOY_LAMBDA_URL - Lambda function URL for backend testing
bucket := env_var_or_default('RUNVOY_RELEASES_BUCKET', 'runvoy-releases')

# Build all binaries
build: build-cli build-local build-orchestrator build-event-processor

# Deploy all binaries
deploy: deploy-orchestrator deploy-event-processor

# Build CLI client
[working-directory: 'cmd/runvoy']
build-cli:
    go build \
        -ldflags "-X=runvoy/internal/constants.version=$(cat ../../VERSION | tr -d '\n')-$(date +%Y%m%d)-$(git rev-parse --short HEAD)" \
        -o ../../bin/runvoy

# Build orchestrator backend service (Lambda function)
[working-directory: 'cmd/backend/aws/orchestrator']
build-orchestrator:
    GOARCH=arm64 GOOS=linux go build \
        -ldflags "-X=runvoy/internal/constants.version=$(cat ../../../../VERSION | tr -d '\n')-$(date +%Y%m%d)-$(git rev-parse --short HEAD)" \
        -o ../../../../dist/bootstrap

[working-directory: 'dist']
build-orchestrator-zip: build-orchestrator
    rm -f bootstrap.zip
    zip bootstrap.zip bootstrap

[working-directory: 'dist']
deploy-orchestrator: build-orchestrator-zip
    aws s3 cp bootstrap.zip s3://{{bucket}}/bootstrap.zip
    aws lambda update-function-code \
        --function-name runvoy-orchestrator \
        --s3-bucket {{bucket}} \
        --s3-key bootstrap.zip > /dev/null
    aws lambda wait function-updated --function-name runvoy-orchestrator

# Build event processor backend service (Lambda function)
[working-directory: 'cmd/backend/aws/event_processor']
build-event-processor:
    GOARCH=arm64 GOOS=linux go build \
        -ldflags "-X=runvoy/internal/constants.version=$(cat ../../../../VERSION | tr -d '\n')-$(date +%Y%m%d)-$(git rev-parse --short HEAD)" \
        -o ../../../../dist/bootstrap

[working-directory: 'dist']
build-event-processor-zip:
    rm -f event-processor.zip
    zip event-processor.zip bootstrap

[working-directory: 'dist']
deploy-event-processor: build-event-processor-zip
    aws s3 cp event-processor.zip s3://{{bucket}}/event-processor.zip
    aws lambda update-function-code \
        --function-name runvoy-event-processor \
        --s3-bucket {{bucket}} \
        --s3-key event-processor.zip > /dev/null
    aws lambda wait function-updated --function-name runvoy-orchestrator

# Build local development server
[working-directory: 'cmd/local']
build-local:
    go build \
        -ldflags "-X=runvoy/internal/constants.version=$(cat ../../VERSION | tr -d '\n')-$(date +%Y%m%d)-$(git rev-parse --short HEAD)" \
        -o ../../bin/local

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
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    go install golang.org/x/tools/cmd/goimports@latest
    pip install pre-commit

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

update-backend-infra:
    aws cloudformation deploy \
        --stack-name runvoy-backend \
        --template-file deployments/cloudformation-backend.yaml \
        --capabilities CAPABILITY_NAMED_IAM

# Run local development server with hot reloading
local-dev-server:
    reflex -r '\.go$' -s -- sh \
        -c 'AWS_REGION=us-east-2 \
            AWS_PROFILE=api-l3x-in \
            RUNVOY_LOG_LEVEL=DEBUG \
            RUNVOY_API_KEYS_TABLE=runvoy-api-keys \
        go run \
            -ldflags "-X=runvoy/internal/constants.version=$(cat VERSION | tr -d '\n')-$(date +%Y%m%d)-$(git rev-parse --short HEAD)" \
            ./cmd/local'

# Smoke test local user creation (requires RUNVOY_ADMIN_API_KEY env var)
smoke-test-local-create-user:
    #!/usr/bin/env bash
    if [ -z "${RUNVOY_ADMIN_API_KEY}" ]; then \
        echo "Error: RUNVOY_ADMIN_API_KEY environment variable is required"; \
        exit 1; \
    fi
    curl -sS -X POST "http://localhost:56212/api/v1/users/create" \
        -H "X-API-Key: ${RUNVOY_ADMIN_API_KEY}" \
        -H "Content-Type: application/json" \
        -d '{"email":"bob@example.com"}' | jq .

smoke-test-local-revoke-user:
    #!/usr/bin/env bash
    if [ -z "${RUNVOY_ADMIN_API_KEY}" ]; then \
        echo "Error: RUNVOY_ADMIN_API_KEY environment variable is required"; \
        exit 1; \
    fi
    curl -sS -X POST "http://localhost:56212/api/v1/users/revoke" \
        -H "X-API-Key: ${RUNVOY_ADMIN_API_KEY}" \
        -H "Content-Type: application/json" \
        -d '{"email":"bob@example.com"}' | jq .

smoke-test-backend-health:
    curl -sS \
        -H "X-API-Key: ${RUNVOY_ADMIN_API_KEY}" \
        -X GET "${RUNVOY_LAMBDA_URL}/api/v1/health" | jq .

smoke-test-backend-users-create:
    curl -sS \
        -H "X-API-Key: ${RUNVOY_ADMIN_API_KEY}" \
        -X POST "${RUNVOY_LAMBDA_URL}/api/v1/users/create" \
        -H "Content-Type: application/json" \
        -d '{"email":"bob@example.com"}' | jq .

smoke-test-backend-run-command:
    curl -sS \
        -H "X-API-Key: ${RUNVOY_ADMIN_API_KEY}" \
        -X POST "${RUNVOY_LAMBDA_URL}/api/v1/run" \
        -H "Content-Type: application/json" \
        -d "{\"command\":\"echo Hello, World! $(date +'%Y-%m-%d-%H-%M')\"}" | jq .
