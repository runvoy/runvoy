# Linting and formatting recipes

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

# Lint/prettify webapp
[working-directory: 'cmd/webapp']
lint-webapp:
    npx prettier --check src/**/*.{js,svelte}
    npx eslint src --ext .js,.svelte
