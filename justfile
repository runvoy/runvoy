# Settings
set dotenv-required

# Variables
bucket := env('RUNVOY_RELEASES_BUCKET', 'runvoy-releases')
stack_name := env('RUNVOY_CLOUDFORMATION_BACKEND_STACK', 'runvoy-backend')
admin_email := env('RUNVOY_ADMIN_EMAIL', 'admin@example.com')
version := trim(read('VERSION'))
git_short_hash := trim(`git rev-parse --short HEAD`)
build_date := datetime_utc('%Y%m%d')
build_flags_x := '-X=runvoy/internal/constants.version='
build_flags := build_flags_x + version + '-' + build_date + '-' + git_short_hash

# Aliases
alias r := runvoy

## Commands
# Build the CLI binary and run it with the given arguments
[default]
runvoy *ARGS: build-cli
    ./bin/runvoy --verbose {{ARGS}}

# Build all binaries
build: build-cli build-local build-orchestrator build-event-processor

# Deploy all binaries
deploy: deploy-backend deploy-webviewer

# Deploy backend binaries
deploy-backend: deploy-orchestrator deploy-event-processor

# Build CLI client
[working-directory: 'cmd/runvoy']
build-cli:
    go build \
        -ldflags {{build_flags}} \
        -o ../../bin/runvoy

# Build orchestrator backend service (Lambda function)
[working-directory: 'cmd/backend/aws/orchestrator']
build-orchestrator:
    GOARCH=arm64 GOOS=linux go build \
        -ldflags {{build_flags}} \
        -o ../../../../dist/bootstrap

# Build event processor backend service (Lambda function)
[working-directory: 'cmd/backend/aws/event_processor']
build-event-processor:
    GOARCH=arm64 GOOS=linux go build \
        -ldflags {{build_flags}} \
        -o ../../../../dist/bootstrap

# Build local development server
[working-directory: 'cmd/local']
build-local:
    go build \
        -ldflags {{build_flags}} \
        -o ../../bin/local

# Build orchestrator zip file
[working-directory: 'dist']
build-orchestrator-zip: build-orchestrator
    rm -f bootstrap.zip
    zip bootstrap.zip bootstrap

# Deploy orchestrator lambda function
[working-directory: 'dist']
deploy-orchestrator: build-orchestrator-zip
    aws s3 cp bootstrap.zip s3://{{bucket}}/bootstrap.zip
    aws lambda update-function-code \
        --function-name runvoy-orchestrator \
        --s3-bucket {{bucket}} \
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
    aws s3 cp event-processor.zip s3://{{bucket}}/event-processor.zip
    aws lambda update-function-code \
        --function-name runvoy-event-processor \
        --s3-bucket {{bucket}} \
        --s3-key event-processor.zip > /dev/null
    aws lambda wait function-updated --function-name runvoy-event-processor

# Deploy webviewer HTML to S3
deploy-webviewer:
    aws s3 cp cmd/webviewer/index.html \
        s3://{{bucket}}/webviewer.html \
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
        --stack-name {{stack_name}} \
        --template-file deployments/cloudformation-backend.yaml \
        --capabilities CAPABILITY_NAMED_IAM
    just create-config-file {{stack_name}}
    just seed-admin-user {{admin_email}} {{stack_name}}

# Destroy backend infrastructure via cloudformation
destroy-backend-infra:
    aws cloudformation delete-stack \
        --stack-name {{stack_name}}
    aws cloudformation wait stack-delete-complete \
        --stack-name {{stack_name}}

# Initialize backend infrastructure and seed admin user
init: create-backend-infra deploy

# Destroy lambda bucket
destroy-lambda-bucket:
    aws s3 rm s3://{{bucket}} --recursive
    aws s3 rb s3://{{bucket}}

# Create/update config file with API endpoint from CloudFormation stack
create-config-file stack_name:
    go run scripts/create-config-file/main.go {{stack_name}}

# Seed initial admin user into DynamoDB (idempotent)
seed-admin-user email stack_name:
    go run scripts/seed-admin-user/main.go {{email}} {{stack_name}}

# Run local development server with hot reloading
local-dev-server:
    reflex -r '\.go$' -s -- go run -ldflags {{build_flags}} ./cmd/local

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
smoke-test-backend-users-create email:
    curl -sS \
        -H "X-API-Key: ${RUNVOY_ADMIN_API_KEY}" \
        -X POST "${RUNVOY_LAMBDA_URL}/api/v1/users/create" \
        -H "Content-Type: application/json" \
        -d '{"email":"{{email}}"}' | jq .

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

# Update README.md with latest CLI help output
# This ensures the README stays in sync with CLI commands
update-readme-help: build-cli
    go run scripts/update-readme-help/main.go ./bin/runvoy

# Sync Lambda environment variables to local .env file for development
local-dev-sync:
    go run scripts/sync-env-vars/main.go

# Upgrade all dependencies
upgrade-dependencies:
    go get -u all

# TODO run agg into a github action and store it as asset so to avoid
# having to commit the gif to the repository
record-demo:
    asciinema rec --overwrite runvoy-demo.cast
    agg --theme monokai runvoy-demo.cast runvoy-demo.gif
