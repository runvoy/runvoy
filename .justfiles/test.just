# Testing recipes

# Run all tests
test:
    go test ./...

# Generate coverage profile
gen-coverage:
    go test -coverprofile=coverage.out ./...

# Run tests with coverage
test-coverage: gen-coverage
    go tool cover -html=coverage.out -o coverage.html
    go-test-coverage --config .testcoverage.yml
