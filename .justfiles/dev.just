# Development recipes

# Setup development environment
dev-setup: dev-setup-webapp
    go mod tidy
    go mod download
    go install golang.org/x/tools/cmd/goimports@latest
    go install github.com/vladopajic/go-test-coverage/v2@latest

# Setup webapp development environment
[working-directory: 'cmd/webapp']
dev-setup-webapp:
    npm install

# Run local development server
dev-run: build-local
    ./bin/local

# Run local development server with hot reloading
dev-server:
    reflex -r '\.(go|csv|env)$' -s -- go run -ldflags '{{build_flags}}' ./cmd/local

# Run local development webapp
[working-directory: 'cmd/webapp']
dev-webapp:
    npx vite

# Sync Lambda environment variables to local .env file for development
dev-sync:
    go run scripts/sync-env-vars/main.go
