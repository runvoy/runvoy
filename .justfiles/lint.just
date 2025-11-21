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

# Type check webapp TypeScript
[working-directory: 'cmd/webapp']
type-check-webapp:
    npm run check

# Lint/prettify webapp
[working-directory: 'cmd/webapp']
lint-webapp:
    npx prettier --check src/**/*.{ts,svelte}
    npx eslint src --ext .ts,.svelte
