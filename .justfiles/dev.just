# Development recipes

# Setup development environment
dev-setup: dev-setup-webapp dev-pull-mkdocs-docker
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

# Pull MkDocs documentation from Docker Hub
dev-pull-mkdocs-docker:
    docker pull squidfunk/mkdocs-material

# Create a new MkDocs site in the current directory
dev-mkdocs-new:
    docker run --rm -it -v ${PWD}:/docs squidfunk/mkdocs-material new .

# Build the MkDocs site
dev-mkdocs-build:
    docker run --rm -it -v ${PWD}:/docs squidfunk/mkdocs-material build

# Serve the MkDocs site
dev-mkdocs-serve:
    docker run --rm -it -p 8000:8000 -v ${PWD}:/docs squidfunk/mkdocs-material