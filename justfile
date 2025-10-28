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

# Create lambda bucket
create-lambda-bucket:
    aws cloudformation deploy \
        --stack-name runvoy-releases-bucket \
        --template-file infra/runvoy-bucket.yaml

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

smoke-test-backend:
    curl -X GET https://h4wgz3vui4wsri6bp65yzbynv40vqhqt.lambda-url.us-east-2.on.aws/api/v1/greet/$(date +%s)

# Run local development server with hot reloading
local-dev-server:
    reflex -r '\.go$' -s -- sh -c 'go run ./cmd/local'
