bucket := 'runvoy-releases'

# Build all binaries
build: build-cli build-lambda build-local

# Build CLI client
[working-directory: 'cmd/runvoy']
build-cli:
    go build -o ../../bin/runvoy

# Build Lambda function
[working-directory: 'cmd/lambda']
build-lambda:
    GOARCH=arm64 GOOS=linux go build -o ../../bin/lambda

# Build local development server
[working-directory: 'local']
build-local:
    go build -o ../bin/local

# Run local development server
run-local: build-local
    ./bin/local

# Run tests
test:
    go test ./...

# Run local integration tests
test-local: build-local
    ./scripts/test-local.sh

# Clean build artifacts
clean:
    rm -rf bin/
    go clean

# Development setup
dev-setup:
    go mod tidy
    go mod download

# Infrastructure commands
create-lambda-bucket:
    aws cloudformation deploy \
        --stack-name runvoy-releases-bucket \
        --template-file infra/runvoy-bucket.yaml

[working-directory: 'cmd/lambda']
update-lambda:
    rm -f function.zip bootstrap
    GOARCH=arm64 GOOS=linux go build -o bootstrap
    zip function.zip bootstrap
    aws s3 cp function.zip s3://{{bucket}}/bootstrap.zip
    aws lambda update-function-code --function-name runvoy-orchestrator --zip-file fileb://function.zip > /dev/null
    aws lambda wait function-updated --function-name runvoy-orchestrator

init:
    aws cloudformation deploy \
        --stack-name runvoy-backend \
        --template-file infra/cloudformation-backend.yaml \
        --parameter-overrides LambdaCodeBucket={{bucket}} JWTSecret=$(openssl rand -hex 32) \
        --capabilities CAPABILITY_NAMED_IAM

destroy:
    aws cloudformation delete-stack --stack-name runvoy-backend
    aws cloudformation wait stack-delete-complete --stack-name runvoy-backend
