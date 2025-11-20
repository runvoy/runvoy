# Development recipes

# Development setup
dev-setup: dev-setup-webapp
    go mod tidy
    go mod download
    go install golang.org/x/tools/cmd/goimports@latest
    go install github.com/vladopajic/go-test-coverage/v2@latest

[working-directory: 'cmd/webapp']
dev-setup-webapp:
    npm install

# Run CI pipeline, to be executed by GitHub Actions
ci-test: dev-setup test

# Run local development server
run-local: build-local
    ./bin/local

# Run local development server with hot reloading
local-dev-server:
    reflex -r '\.(go|csv|env)$' -s -- go run -ldflags '{{build_flags}}' ./cmd/local

# Run local development webapp
[working-directory: 'cmd/webapp']
local-dev-webapp:
    npx vite

# Sync Lambda environment variables to local .env file for development
local-dev-sync:
    go run scripts/sync-env-vars/main.go
