# Linting and formatting recipes

# Lint all Go code
lint:
    mkdir -p .cache/go-build .cache/golangci-lint
    GOCACHE="{{justfile_directory()}}/.cache/go-build" \
    GOLANGCI_LINT_CACHE="{{justfile_directory()}}/.cache/golangci-lint" \
    golangci-lint run

# Lint and fix Go code
lint-fix:
    mkdir -p .cache/go-build .cache/golangci-lint
    GOCACHE="{{justfile_directory()}}/.cache/go-build" \
    GOLANGCI_LINT_CACHE="{{justfile_directory()}}/.cache/golangci-lint" \
    golangci-lint run --fix

# Format Go code
fmt:
    go fmt ./...
    goimports -w .

# Type check webapp TypeScript
[working-directory: 'cmd/webapp']
type-check-webapp:
    npm run check

# Lint/prettify webapp
[working-directory: 'cmd/webapp']
lint-webapp:
    npx prettier --check src/**/*.{ts,svelte}
    npx eslint src --ext .ts,.svelte
