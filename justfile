# Settings
set dotenv-load

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
build: build-cli build-local build-backend build-frontend

# Build backend binaries (Lambda functions)
build-backend: build-orchestrator build-event-processor build-websocket-manager build-websocket-log-forwarder

# Build frontend binary
build-frontend: build-webapp

# Deploy all binaries
deploy: deploy-backend deploy-frontend

# Deploy backend binaries
deploy-backend: deploy-orchestrator deploy-event-processor deploy-websocket

# Deploy websocket binaries
deploy-websocket: deploy-websocket-manager deploy-websocket-log-forwarder

# Deploy webapp
deploy-frontend: deploy-webapp

# Build CLI client
[working-directory: 'cmd/cli']
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

# Build WebSocket connection manager backend service (Lambda function)
[working-directory: 'cmd/backend/aws/websocket/manager']
build-websocket-manager:
    GOARCH=arm64 GOOS=linux go build \
        -ldflags {{build_flags}} \
        -o ../../../../../dist/bootstrap

# Build WebSocket connection manager zip file
[working-directory: 'dist']
build-websocket-manager-zip: build-websocket-manager
    rm -f websocket-manager.zip
    zip websocket-manager.zip bootstrap

# Deploy WebSocket connection manager lambda function
[working-directory: 'dist']
deploy-websocket-manager: build-websocket-manager-zip
    aws s3 cp websocket-manager.zip s3://{{bucket}}/websocket-manager.zip
    aws lambda update-function-code \
        --function-name runvoy-websocket-manager \
        --s3-bucket {{bucket}} \
        --s3-key websocket-manager.zip > /dev/null
    aws lambda wait function-updated --function-name runvoy-websocket-manager

# Build WebSocket log forwarder backend service (Lambda function)
[working-directory: 'cmd/backend/aws/websocket/log_forwarder']
build-websocket-log-forwarder:
    GOARCH=arm64 GOOS=linux go build \
        -ldflags {{build_flags}} \
        -o ../../../../../dist/bootstrap

# Build WebSocket log forwarder zip file
[working-directory: 'dist']
build-websocket-log-forwarder-zip: build-websocket-log-forwarder
    rm -f websocket-log-forwarder.zip
    zip websocket-log-forwarder.zip bootstrap

# Deploy WebSocket log forwarder lambda function
[working-directory: 'dist']
deploy-websocket-log-forwarder: build-websocket-log-forwarder-zip
    aws s3 cp websocket-log-forwarder.zip s3://{{bucket}}/websocket-log-forwarder.zip
    aws lambda update-function-code \
        --function-name runvoy-websocket-log-forwarder \
        --s3-bucket {{bucket}} \
        --s3-key websocket-log-forwarder.zip > /dev/null
    aws lambda wait function-updated --function-name runvoy-websocket-log-forwarder

# Deploy webapp to S3
[working-directory: 'cmd/webapp']
deploy-webapp: build-webapp
    netlify deploy --no-build --dir dist --prod

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
    go tool cover -func=coverage.out | grep total | awk '{print $3}'

# Clean build artifacts
clean:
    rm -rf bin/ coverage.out coverage.html
    go clean

# Development setup
dev-setup: dev-setup-webapp
    go mod tidy
    go mod download
    go install golang.org/x/tools/cmd/goimports@latest

[working-directory: 'cmd/webapp']
dev-setup-webapp:
    npm install

# Run CI pipeline, to be executed by GitHub Actions
ci-test: dev-setup test

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
check: lint test lint-webapp

# Install pre-commit hook
install-hook:
    printf "just update-readme-help check\n" > .git/hooks/pre-commit
    chmod +x .git/hooks/pre-commit

# pre-commit hook commands to run when the pre-commit hook is triggered
pre-commit: check test-coverage
    git add coverage.html coverage.out README.md

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
    @echo ""
    @echo "==================================================================="
    @echo "Backend infrastructure initialized successfully!"
    @echo "==================================================================="
    @echo ""
    @echo "Next step: Register the default Docker image"
    @echo ""
    @echo "To register the default image, use the CLI:"
    @echo "  runvoy images register public.ecr.aws/docker/library/ubuntu:22.04 --set-default"
    @echo ""
    @echo "You can also list all registered images:"
    @echo "  runvoy images list"
    @echo ""

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

# Update README.md with latest CLI help output
# This ensures the README stays in sync with CLI commands
update-readme-help: build-cli
    go run scripts/update-readme-help/main.go ./bin/runvoy
    git add README.md

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

# Run local development webapp
[working-directory: 'cmd/webapp']
local-dev-webapp:
    npx vite

# Build webapp
[working-directory: 'cmd/webapp']
build-webapp: lint-webapp
    npx vite build

# Lint/prettify webapp
[working-directory: 'cmd/webapp']
lint-webapp:
    npx prettier --check src/**/*.{js,svelte}
    npx eslint src --ext .js,.svelte
