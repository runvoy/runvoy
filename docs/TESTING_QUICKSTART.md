# Testing Quick Start Guide

This guide helps you get started with the new testing infrastructure immediately.

## What We've Improved

We've moved from **11.1% coverage** with casual tests to a comprehensive testing strategy targeting **80%+ coverage**.

### New Structure

```
runvoy/
├── internal/
│   ├── testutil/           ← NEW: Shared test utilities
│   │   ├── fixtures.go     ← Test data builders
│   │   └── assert.go       ← Custom assertions
│   ├── database/
│   │   └── interfaces.go   ← NEW: Repository interfaces for mocking
│   └── auth/
│       └── apikey_test.go  ← NEW: Example comprehensive test
└── docs/
    ├── TESTING_STRATEGY.md  ← Full strategy document
    └── TESTING_EXAMPLES.md  ← Before/after examples
```

## Quick Start: Writing Your First Test

### 1. Install Dependencies (Already Done)

```bash
go mod tidy
```

Dependencies added:
- `github.com/stretchr/testify` - Assertions and mocking

### 2. Use Test Utilities

```go
import (
    "testing"
    "runvoy/internal/testutil"
    "github.com/stretchr/testify/assert"
)

func TestSomething(t *testing.T) {
    // Build test data easily
    user := testutil.NewUserBuilder().
        WithEmail("test@example.com").
        Build()

    // Use clear assertions
    assert.NotNil(t, user)
    assert.Equal(t, "test@example.com", user.Email)
}
```

### 3. Follow the Pattern

Every test should follow **AAA** (Arrange, Act, Assert):

```go
func TestUserRepository_CreateUser(t *testing.T) {
    // ARRANGE - Set up test data and dependencies
    user := testutil.NewUserBuilder().Build()
    mockClient := new(MockDynamoDBClient)
    repo := NewUserRepository(mockClient, "test-table", testutil.SilentLogger())

    // ACT - Execute the function
    err := repo.CreateUser(context.Background(), user, "hash")

    // ASSERT - Verify results
    assert.NoError(t, err)
}
```

## Common Testing Patterns

### Table-Driven Tests (Recommended)

```go
func TestFormatStatus(t *testing.T) {
    tests := []struct {
        name   string
        status string
        want   string
    }{
        {name: "pending", status: "pending", want: "⏳ pending"},
        {name: "running", status: "running", want: "▶️  running"},
        {name: "completed", status: "completed", want: "✅ completed"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := FormatStatus(tt.status)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### Testing with Mocks

```go
// 1. Define interface (in production code)
type DynamoDBClient interface {
    PutItem(ctx context.Context, params *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error)
}

// 2. Create mock (in test file)
type MockDynamoDBClient struct {
    mock.Mock
}

func (m *MockDynamoDBClient) PutItem(ctx context.Context, params *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error) {
    args := m.Called(ctx, params)
    return args.Get(0).(*dynamodb.PutItemOutput), args.Error(1)
}

// 3. Use in test
func TestWithMock(t *testing.T) {
    mockClient := new(MockDynamoDBClient)
    mockClient.On("PutItem", mock.Anything, mock.Anything).
        Return(&dynamodb.PutItemOutput{}, nil)

    // Test your code...

    mockClient.AssertExpectations(t)
}
```

### Test Fixtures

```go
// Instead of this:
user := &api.User{
    Email: "test@example.com",
    CreatedAt: time.Now(),
    Revoked: false,
}

// Use this:
user := testutil.NewUserBuilder().Build()

// Or customize:
user := testutil.NewUserBuilder().
    WithEmail("custom@example.com").
    Revoked().
    Build()
```

## Running Tests

```bash
# All tests
go test ./...

# Specific package
go test ./internal/auth/...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Verbose output
go test -v ./...

# Run specific test
go test -run TestGenerateAPIKey ./internal/auth/

# Short mode (skip slow tests)
go test -short ./...
```

## Using Justfile Commands

```bash
# Run all tests
just test

# Run with coverage report
just test-coverage

# Run integration tests
just test-integration

# Run all checks (lint + test)
just check
```

## Example: Testing a New Function

Let's say you want to add a function `ValidateEmail(email string) error`:

### 1. Write the Test First (TDD)

```go
// internal/validation/email_test.go
package validation

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestValidateEmail(t *testing.T) {
    tests := []struct {
        name    string
        email   string
        wantErr bool
    }{
        {name: "valid email", email: "user@example.com", wantErr: false},
        {name: "empty email", email: "", wantErr: true},
        {name: "no @", email: "userexample.com", wantErr: true},
        {name: "no domain", email: "user@", wantErr: true},
        {name: "no user", email: "@example.com", wantErr: true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateEmail(tt.email)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### 2. Run the Test (It Will Fail)

```bash
go test ./internal/validation/...
# FAIL: undefined: ValidateEmail
```

### 3. Implement the Function

```go
// internal/validation/email.go
package validation

import (
    "errors"
    "strings"
)

func ValidateEmail(email string) error {
    if email == "" {
        return errors.New("email cannot be empty")
    }
    if !strings.Contains(email, "@") {
        return errors.New("email must contain @")
    }
    parts := strings.Split(email, "@")
    if parts[0] == "" || parts[1] == "" {
        return errors.New("email must have user and domain")
    }
    return nil
}
```

### 4. Run the Test Again (It Should Pass)

```bash
go test ./internal/validation/...
# PASS
```

## Testing Checklist

When writing tests for a new feature:

- [ ] Test the happy path (success case)
- [ ] Test error cases (invalid input, failures)
- [ ] Test edge cases (empty strings, nil values, boundaries)
- [ ] Test with table-driven tests when you have >2 scenarios
- [ ] Use meaningful test names that describe the scenario
- [ ] Follow AAA pattern (Arrange, Act, Assert)
- [ ] Mock external dependencies (databases, APIs)
- [ ] Keep tests fast (<5 seconds total for package)
- [ ] Verify tests pass before committing

## Next Steps

1. **Read the full strategy**: `docs/TESTING_STRATEGY.md`
2. **See before/after examples**: `docs/TESTING_EXAMPLES.md`
3. **Pick a package to test**: Start with `internal/auth` or `internal/config`
4. **Write tests using patterns from this guide**
5. **Run tests and check coverage**
6. **Iterate and improve**

## Getting Help

- Review existing tests in `internal/auth/apikey_test.go`
- Check the examples in `docs/TESTING_EXAMPLES.md`
- Look at the test utilities in `internal/testutil/`
- Read Go testing best practices: https://go.dev/doc/tutorial/add-a-test

## Current Status

✅ Testing infrastructure set up
✅ Test utilities created (`internal/testutil`)
✅ Example test written (`internal/auth/apikey_test.go`)
✅ Repository interfaces defined (`internal/database/interfaces.go`)
✅ Documentation complete

**Current Coverage**: ~13% (up from 11.1%)
**Target Coverage**: 80%+

Ready to start testing! 🚀
