# Makefile for runvoy

.PHONY: help build build-lambda build-cli build-local test test-local run-local clean

# Default target
help:
	@echo "Available targets:"
	@echo "  build          - Build all binaries"
	@echo "  build-lambda   - Build Lambda function"
	@echo "  build-cli      - Build CLI client"
	@echo "  build-local    - Build local development server"
	@echo "  test           - Run all tests"
	@echo "  test-local     - Run local integration tests"
	@echo "  run-local      - Run local development server"
	@echo "  clean          - Clean build artifacts"

# Build all binaries
build: build-lambda build-cli build-local

# Build Lambda function
build-lambda:
	@echo "Building Lambda function..."
	@mkdir -p bin
	@cd cmd/lambda && go build -o ../../bin/lambda main.go

# Build CLI client
build-cli:
	@echo "Building CLI client..."
	@mkdir -p bin
	@cd cmd/runvoy && go build -o ../../bin/runvoy main.go

# Build local development server
build-local:
	@echo "Building local development server..."
	@mkdir -p bin
	@cd local && go build -o ../bin/local main.go

# Run all tests
test:
	@echo "Running tests..."
	@go test ./...

# Run local integration tests
test-local: build-local
	@echo "Running local integration tests..."
	@./scripts/test-local.sh

# Run local development server
run-local: build-local
	@echo "Starting local development server..."
	@./bin/local

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@go clean

# Development helpers
dev-setup:
	@echo "Setting up development environment..."
	@go mod tidy
	@go mod download

# Docker helpers
docker-build:
	@echo "Building Docker images..."
	@docker build -t runvoy-lambda -f Dockerfile.lambda .
	@docker build -t runvoy-local -f Dockerfile.local .

# Deployment helpers
deploy-lambda:
	@echo "Deploying Lambda function..."
	@./scripts/deploy.sh