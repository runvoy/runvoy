bucket := 'runvoy-releases'

# Build all binaries
build: build-cli build-backend build-local

# Build CLI client
[working-directory: 'cmd/runvoy']
build-cli:
    go build -o ../../bin/runvoy

# Build backend service (Lambda function)
[working-directory: 'cmd/backend/aws']
build-backend:
    GOARCH=arm64 GOOS=linux go build -o ../../../dist/bootstrap

# Build local development server
[working-directory: 'cmd/local']
build-local:
    go build -o ../../bin/local

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
        --template-file infra/runvoy-bucket.yaml

update-backend-infra:
    aws cloudformation deploy \
        --stack-name runvoy-backend \
        --template-file infra/cloudformation-backend.yaml \
        --capabilities CAPABILITY_NAMED_IAM

# Update backend service (Lambda function)
[working-directory: 'dist']
update-backend: build-backend
    zip bootstrap.zip bootstrap
    aws s3 cp bootstrap.zip s3://{{bucket}}/bootstrap.zip
    aws lambda update-function-code \
        --function-name runvoy-orchestrator \
        --s3-bucket runvoy-releases \
        --s3-key bootstrap.zip > /dev/null
    aws lambda wait function-updated --function-name runvoy-orchestrator

smoke-test-backend-health:
    curl -X GET https://h4wgz3vui4wsri6bp65yzbynv40vqhqt.lambda-url.us-east-2.on.aws/api/v1/greet/$(date +%s)

# Run local development server with hot reloading
local-dev-server:
    reflex -r '\.go$' -s -- sh -c 'AWS_REGION=us-east-2 AWS_PROFILE=api-l3x-in RUNVOY_LOG_LEVEL=DEBUG RUNVOY_API_KEYS_TABLE=runvoy-api-keys go run ./cmd/local'

smoke-test-local-user:
    curl -sS -X POST "http://localhost:56212/api/v1/users" \
        -H "Content-Type: application/json" \
        -d '{"email":"alice@example.com"}' | jq .