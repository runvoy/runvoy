# Testing recipes

# Run all tests
test:
    mkdir -p .cache/go-build
    GOCACHE="{{justfile_directory()}}/.cache/go-build" go test ./...

# Generate coverage profile
gen-coverage:
    mkdir -p .cache/go-build
    GOCACHE="{{justfile_directory()}}/.cache/go-build" go test -coverprofile=coverage.out ./...

# Run tests with coverage
test-coverage: gen-coverage
    go tool cover -html=coverage.out -o coverage.html
    go-test-coverage --config .testcoverage.yml

# Run webapp tests
[working-directory: 'cmd/webapp']
test-webapp: type-check-webapp lint-webapp
    npm run test

# Run all checks (lint + test)
check: lint test-coverage test-webapp
