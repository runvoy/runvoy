# Testing Strategy and Improvement Plan

## Executive Summary

Current test coverage: **11.1%**
- 4 test files covering 3 out of 15 internal packages
- Basic testing patterns with no mocking framework
- No integration tests
- Critical business logic untested

This document outlines a comprehensive strategy to achieve >80% test coverage with a maintainable, extensible testing infrastructure.

---

## Current State Analysis

### What We Have

**Testing Framework:**
- Standard Go `testing` package
- Basic table-driven tests in some areas
- CI integration with race detector and coverage reporting
- Codecov integration

**Tested Components:**
- ✅ `internal/output` - Output formatting (32 tests + benchmarks)
- ✅ `internal/server` - Middleware and basic handlers (14 tests)
- ✅ `internal/client` - Client initialization (1 test)

**Test Statistics:**
```
Total Packages:        15
Tested Packages:       3 (20%)
Untested Packages:     12 (80%)
Overall Coverage:      11.1%
Total Test Functions:  47
```

### Critical Gaps

**Untested Core Business Logic:**
1. **Database Layer** (`internal/database/dynamodb/`)
   - User management (CreateUser, GetUser, UpdateLastUsed, RevokeUser)
   - Execution tracking (CreateExecution, GetExecution, UpdateExecution, ListExecutions)
   - DynamoDB operations with error handling

2. **Authentication** (`internal/auth/`)
   - API key generation
   - API key hashing
   - Authentication middleware

3. **Event Processing** (`internal/events/`)
   - ECS task completion handling
   - Event routing

4. **AWS Integration** (`internal/app/aws/`)
   - Task runner orchestration
   - CloudWatch logs retrieval
   - ECS task management

5. **API Handlers** (`internal/server/`)
   - Run command endpoint
   - Status endpoint
   - Logs endpoint
   - Kill endpoint
   - List executions endpoint

6. **CLI Commands** (`cmd/cli/cmd/`)
   - All command implementations
   - Configuration management
   - Output formatting integration

---

## Problems with Current Approach

### 1. **No Mocking Strategy**
- Direct AWS SDK calls make testing difficult
- No interfaces for external dependencies
- Tests require real DynamoDB/AWS resources or are skipped

### 2. **Lack of Test Organization**
- No test helpers or utilities
- Duplicated test setup code
- No consistent patterns across packages

### 3. **Missing Test Types**
- No integration tests
- No end-to-end tests
- No contract tests for API endpoints
- Limited error path testing

### 4. **Poor Testability**
- Many functions have side effects
- Hard-coded dependencies
- Difficult to isolate units

---

## Proposed Testing Strategy

### Three-Layer Testing Approach

```
┌─────────────────────────────────────────────────────┐
│  1. Unit Tests (Fast, Isolated)                    │
│     - Pure functions                                │
│     - Business logic with mocked dependencies       │
│     - Target: 90% coverage                          │
└─────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────┐
│  2. Integration Tests (Moderate Speed)              │
│     - Database operations with DynamoDB Local       │
│     - API handlers with test server                 │
│     - Target: Key workflows covered                 │
└─────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────┐
│  3. E2E Tests (Slow, Comprehensive)                 │
│     - Full CLI command execution                    │
│     - Real infrastructure (dev environment)         │
│     - Target: Critical user journeys               │
└─────────────────────────────────────────────────────┘
```

---

## Recommended Test Infrastructure

### 1. Testing Dependencies

Add to `go.mod`:

```go
require (
    github.com/stretchr/testify v1.10.0
    github.com/aws/aws-sdk-go-v2/service/dynamodb v1.52.3
    // For mocking AWS services
    github.com/golang/mock v1.6.0  // or github.com/uber-go/mock
)
```

**Why testify?**
- Already in dependencies (indirect)
- Provides `assert` and `require` for cleaner assertions
- Includes `mock` package for interface mocking
- Industry standard in Go community

### 2. Directory Structure

```
runvoy/
├── internal/
│   ├── database/
│   │   ├── dynamodb/
│   │   │   ├── users.go
│   │   │   ├── users_test.go          # Unit tests with mocks
│   │   │   ├── executions.go
│   │   │   └── executions_test.go
│   │   ├── repository.go               # Interface definitions
│   │   └── mocks/                      # Generated mocks
│   │       ├── mock_user_repository.go
│   │       └── mock_execution_repository.go
│   ├── testutil/                       # Shared test utilities
│   │   ├── assert.go                   # Custom assertions
│   │   ├── fixtures.go                 # Test data builders
│   │   ├── dynamodb.go                 # DynamoDB test helpers
│   │   └── context.go                  # Context helpers
│   └── ...
├── test/
│   ├── integration/                    # Integration tests
│   │   ├── database_test.go
│   │   ├── api_test.go
│   │   └── README.md
│   └── e2e/                           # End-to-end tests
│       ├── cli_test.go
│       └── README.md
└── scripts/
    └── run-dynamodb-local.sh          # Local DynamoDB for testing
```

### 3. Mock Generation Setup

Create `Makefile` or add to `justfile`:

```makefile
# Generate mocks for all interfaces
generate-mocks:
    mockgen -source=internal/database/repository.go \
        -destination=internal/database/mocks/mock_repository.go \
        -package=mocks
    mockgen -source=internal/app/runner.go \
        -destination=internal/app/mocks/mock_runner.go \
        -package=mocks
```

---

## Implementation Patterns

### Pattern 1: Repository Testing with Mocks

**Before (Untestable):**
```go
// users.go
func (r *UserRepository) CreateUser(ctx context.Context, user *api.User) error {
    _, err := r.client.PutItem(ctx, &dynamodb.PutItemInput{...})
    return err
}
```

**After (Testable with Interface):**
```go
// repository.go - Define interface
type DynamoDBClient interface {
    PutItem(ctx context.Context, params *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error)
    GetItem(ctx context.Context, params *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error)
    Query(ctx context.Context, params *dynamodb.QueryInput) (*dynamodb.QueryOutput, error)
    UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput) (*dynamodb.UpdateItemOutput, error)
}

// users.go
type UserRepository struct {
    client    DynamoDBClient  // Use interface
    tableName string
    logger    *slog.Logger
}
```

**Test with Mock:**
```go
// users_test.go
func TestUserRepository_CreateUser(t *testing.T) {
    tests := []struct {
        name      string
        user      *api.User
        mockSetup func(*mocks.MockDynamoDBClient)
        wantErr   bool
        errType   error
    }{
        {
            name: "successfully creates user",
            user: &api.User{
                Email:     "test@example.com",
                CreatedAt: time.Now(),
            },
            mockSetup: func(m *mocks.MockDynamoDBClient) {
                m.EXPECT().
                    PutItem(gomock.Any(), gomock.Any()).
                    Return(&dynamodb.PutItemOutput{}, nil)
            },
            wantErr: false,
        },
        {
            name: "returns conflict when user exists",
            user: &api.User{Email: "existing@example.com"},
            mockSetup: func(m *mocks.MockDynamoDBClient) {
                m.EXPECT().
                    PutItem(gomock.Any(), gomock.Any()).
                    Return(nil, &types.ConditionalCheckFailedException{})
            },
            wantErr: true,
            errType: apperrors.ErrConflict("", nil),
        },
        {
            name: "handles database errors",
            user: &api.User{Email: "test@example.com"},
            mockSetup: func(m *mocks.MockDynamoDBClient) {
                m.EXPECT().
                    PutItem(gomock.Any(), gomock.Any()).
                    Return(nil, errors.New("database connection failed"))
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctrl := gomock.NewController(t)
            defer ctrl.Finish()

            mockClient := mocks.NewMockDynamoDBClient(ctrl)
            tt.mockSetup(mockClient)

            repo := NewUserRepository(mockClient, "test-table", slog.Default())

            err := repo.CreateUser(context.Background(), tt.user, "hash")

            if tt.wantErr {
                assert.Error(t, err)
                if tt.errType != nil {
                    assert.ErrorIs(t, err, tt.errType)
                }
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Pattern 2: HTTP Handler Testing

```go
// handlers_run_test.go
func TestRouter_HandleRun(t *testing.T) {
    tests := []struct {
        name           string
        requestBody    string
        mockSetup      func(*mocks.MockService)
        expectedStatus int
        expectedBody   map[string]interface{}
    }{
        {
            name: "successfully starts execution",
            requestBody: `{"command": "echo hello"}`,
            mockSetup: func(m *mocks.MockService) {
                m.EXPECT().
                    Run(gomock.Any(), gomock.Any()).
                    Return(&api.Execution{
                        ID:      "exec-123",
                        Status:  "pending",
                        Command: "echo hello",
                    }, nil)
            },
            expectedStatus: http.StatusOK,
            expectedBody: map[string]interface{}{
                "execution_id": "exec-123",
                "status":       "pending",
            },
        },
        {
            name: "returns 400 for invalid command",
            requestBody: `{"command": ""}`,
            expectedStatus: http.StatusBadRequest,
        },
        {
            name: "handles unauthorized user",
            requestBody: `{"command": "echo test"}`,
            mockSetup: func(m *mocks.MockService) {
                m.EXPECT().
                    Run(gomock.Any(), gomock.Any()).
                    Return(nil, apperrors.ErrUnauthorized("", nil))
            },
            expectedStatus: http.StatusUnauthorized,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctrl := gomock.NewController(t)
            defer ctrl.Finish()

            mockSvc := mocks.NewMockService(ctrl)
            if tt.mockSetup != nil {
                tt.mockSetup(mockSvc)
            }

            router := NewRouter(mockSvc)

            req := httptest.NewRequest(http.MethodPost, "/api/v1/run",
                strings.NewReader(tt.requestBody))
            req.Header.Set("Content-Type", "application/json")

            w := httptest.NewRecorder()
            router.ServeHTTP(w, req)

            assert.Equal(t, tt.expectedStatus, w.Code)

            if tt.expectedBody != nil {
                var response map[string]interface{}
                err := json.Unmarshal(w.Body.Bytes(), &response)
                assert.NoError(t, err)

                for key, expected := range tt.expectedBody {
                    assert.Equal(t, expected, response[key])
                }
            }
        })
    }
}
```

### Pattern 3: Test Fixtures and Builders

```go
// internal/testutil/fixtures.go
package testutil

import (
    "time"
    "runvoy/internal/api"
)

// UserBuilder provides a fluent interface for building test users
type UserBuilder struct {
    user *api.User
}

func NewUserBuilder() *UserBuilder {
    return &UserBuilder{
        user: &api.User{
            Email:     "test@example.com",
            CreatedAt: time.Now(),
            Revoked:   false,
        },
    }
}

func (b *UserBuilder) WithEmail(email string) *UserBuilder {
    b.user.Email = email
    return b
}

func (b *UserBuilder) Revoked() *UserBuilder {
    b.user.Revoked = true
    return b
}

func (b *UserBuilder) Build() *api.User {
    return b.user
}

// Usage in tests:
// user := testutil.NewUserBuilder().WithEmail("admin@example.com").Build()
// revokedUser := testutil.NewUserBuilder().Revoked().Build()
```

### Pattern 4: Integration Tests with DynamoDB Local

```go
// test/integration/database_test.go
//go:build integration

package integration

import (
    "context"
    "testing"

    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func setupDynamoDBLocal(t *testing.T) *dynamodb.Client {
    cfg, err := config.LoadDefaultConfig(context.Background(),
        config.WithEndpointResolver(aws.EndpointResolverFunc(
            func(service, region string) (aws.Endpoint, error) {
                return aws.Endpoint{
                    URL: "http://localhost:8000",
                }, nil
            },
        )),
    )
    require.NoError(t, err)

    return dynamodb.NewFromConfig(cfg)
}

func TestUserRepository_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    client := setupDynamoDBLocal(t)
    repo := dynamodb.NewUserRepository(client, "test-users", slog.Default())

    t.Run("create and retrieve user", func(t *testing.T) {
        user := testutil.NewUserBuilder().
            WithEmail("integration@example.com").
            Build()

        err := repo.CreateUser(context.Background(), user, "test-hash")
        require.NoError(t, err)

        retrieved, err := repo.GetUserByEmail(context.Background(), user.Email)
        require.NoError(t, err)
        assert.Equal(t, user.Email, retrieved.Email)
    })
}
```

---

## Test Organization Guidelines

### 1. File Naming Conventions

```
✅ GOOD:
users.go          → users_test.go
handlers.go       → handlers_test.go
middleware.go     → middleware_test.go

❌ BAD:
users.go          → test_users.go
users.go          → user_tests.go
```

### 2. Test Function Naming

```go
// Format: Test<FunctionName>_<Scenario>
func TestUserRepository_CreateUser_Success(t *testing.T)
func TestUserRepository_CreateUser_ConflictError(t *testing.T)
func TestUserRepository_CreateUser_ValidationError(t *testing.T)

// For table-driven tests:
func TestUserRepository_CreateUser(t *testing.T) {
    tests := []struct {
        name string  // Describes the scenario
        // ...
    }{
        {name: "successfully creates user"},
        {name: "returns error when email is invalid"},
    }
}
```

### 3. Test Structure (AAA Pattern)

```go
func TestExample(t *testing.T) {
    // ARRANGE - Set up test data and dependencies
    user := testutil.NewUserBuilder().Build()
    mockRepo := mocks.NewMockRepository()

    // ACT - Execute the function under test
    result, err := service.DoSomething(user)

    // ASSERT - Verify the results
    assert.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

### 4. Build Tags for Test Types

```go
// Unit tests (default, no build tag)
// users_test.go

// Integration tests
//go:build integration
// users_integration_test.go

// E2E tests
//go:build e2e
// cli_e2e_test.go
```

**Running different test types:**
```bash
# Unit tests only (fast)
go test ./...

# With integration tests
go test -tags=integration ./...

# E2E tests
go test -tags=e2e ./test/e2e/...

# All tests
go test -tags=integration,e2e ./...
```

---

## Implementation Roadmap

### Phase 1: Foundation (Week 1)
**Goal: Establish testing infrastructure**

- [ ] Add testify to dependencies
- [ ] Set up mockgen tooling
- [ ] Create `internal/testutil` package with helpers
- [ ] Define repository interfaces
- [ ] Generate initial mocks
- [ ] Update justfile with test commands
- [ ] Document testing patterns in TESTING.md

**Success Criteria:**
- Mocking infrastructure in place
- Test utilities available
- Coverage: 15%

### Phase 2: Core Business Logic (Week 2-3)
**Goal: Test critical paths**

- [ ] Database layer tests (users + executions)
- [ ] Authentication tests
- [ ] Error handling tests
- [ ] API handler tests (run, status, logs)

**Success Criteria:**
- All database operations tested
- Auth logic covered
- Coverage: 40%

### Phase 3: Service Layer (Week 4)
**Goal: Test application logic**

- [ ] App service tests
- [ ] AWS integration tests with mocks
- [ ] Event processor tests
- [ ] Config loading tests

**Success Criteria:**
- Service layer well-tested
- Error paths covered
- Coverage: 60%

### Phase 4: Integration Tests (Week 5)
**Goal: Test component interactions**

- [ ] Set up DynamoDB Local
- [ ] Database integration tests
- [ ] API integration tests
- [ ] Lambda handler integration tests

**Success Criteria:**
- Integration tests passing
- Key workflows validated
- Coverage: 70%

### Phase 5: CLI & E2E (Week 6)
**Goal: Test user-facing features**

- [ ] CLI command tests
- [ ] E2E test suite
- [ ] Error message validation
- [ ] Output format tests

**Success Criteria:**
- CLI commands tested
- E2E tests for critical flows
- Coverage: 80%+

---

## Updated CI Configuration

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [main, develop]
  pull_request:

jobs:
  test:
    name: Test
    runs-on: ubuntu-latest

    services:
      dynamodb-local:
        image: amazon/dynamodb-local
        ports:
          - 8000:8000

    strategy:
      matrix:
        go-version: ['1.23.x', '1.24.x']

    steps:
      - uses: actions/checkout@v5

      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version: ${{ matrix.go-version }}
          cache: true

      - name: Download dependencies
        run: go mod download

      - name: Generate mocks
        run: make generate-mocks

      - name: Run unit tests
        run: go test -v -race -coverprofile=coverage-unit.out ./...

      - name: Run integration tests
        run: go test -v -race -tags=integration -coverprofile=coverage-integration.out ./...
        env:
          DYNAMODB_ENDPOINT: http://localhost:8000

      - name: Merge coverage reports
        run: |
          go install github.com/wadey/gocovmerge@latest
          gocovmerge coverage-*.out > coverage.out

      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          file: ./coverage.out

      - name: Check coverage threshold
        run: |
          COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          echo "Coverage: $COVERAGE%"
          if (( $(echo "$COVERAGE < 70" | bc -l) )); then
            echo "Coverage is below 70%"
            exit 1
          fi
```

---

## Justfile Updates

```makefile
# Test commands
test:
    go test ./...

test-unit:
    go test -v ./...

test-integration:
    @echo "Starting DynamoDB Local..."
    docker run -d -p 8000:8000 --name dynamodb-local amazon/dynamodb-local
    @echo "Running integration tests..."
    DYNAMODB_ENDPOINT=http://localhost:8000 go test -v -tags=integration ./...
    @echo "Stopping DynamoDB Local..."
    docker stop dynamodb-local && docker rm dynamodb-local

test-e2e:
    go test -v -tags=e2e ./test/e2e/...

test-all: test-unit test-integration test-e2e

test-coverage:
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    go tool cover -func=coverage.out | grep total

test-watch:
    reflex -r '\.go$' -s -- go test ./...

# Mock generation
generate-mocks:
    go generate ./...

# Test helpers
test-clean:
    find . -name '*_test.go' -type f -delete
    find . -path '*/mocks/*' -type f -delete
```

---

## Testing Best Practices

### Do's ✅

1. **Write tests first for new code** (TDD approach)
2. **Test behavior, not implementation**
3. **Use table-driven tests for multiple scenarios**
4. **Mock external dependencies** (databases, APIs, file systems)
5. **Test error paths thoroughly**
6. **Keep tests fast** - unit tests should run in milliseconds
7. **Use meaningful test names** that describe the scenario
8. **Follow AAA pattern** (Arrange, Act, Assert)
9. **Use test fixtures and builders** for complex setup
10. **Run tests in CI** before merging

### Don'ts ❌

1. **Don't test third-party libraries** (trust AWS SDK, etc.)
2. **Don't write flaky tests** that randomly fail
3. **Don't share state between tests**
4. **Don't mock everything** - test real logic
5. **Don't test private functions directly** - test through public API
6. **Don't ignore test failures**
7. **Don't skip writing tests** because "it's simple"
8. **Don't duplicate test setup** - use helpers
9. **Don't write tests that depend on execution order**
10. **Don't commit commented-out tests**

---

## Measuring Success

### Coverage Targets

```
Phase 1: 15% → 20%   (Infrastructure)
Phase 2: 20% → 40%   (Database & Auth)
Phase 3: 40% → 60%   (Services)
Phase 4: 60% → 70%   (Integration)
Phase 5: 70% → 80%+  (CLI & E2E)
```

### Quality Metrics

- **Test Speed**: Unit tests < 5 seconds total
- **Flakiness**: 0 flaky tests
- **Maintenance**: Tests updated with code changes
- **Documentation**: All complex test setups documented

### CI Requirements

- ✅ All tests pass before merge
- ✅ Coverage doesn't decrease
- ✅ No race conditions detected
- ✅ Integration tests pass with DynamoDB Local

---

## Next Steps

1. **Review this document** with the team
2. **Prioritize packages** to test first
3. **Start with Phase 1** infrastructure setup
4. **Set up pairing sessions** for knowledge sharing
5. **Create tickets** for each phase
6. **Schedule regular coverage reviews**

---

## Resources

- [Go Testing Best Practices](https://go.dev/doc/tutorial/add-a-test)
- [Testify Documentation](https://github.com/stretchr/testify)
- [gomock Guide](https://github.com/golang/mock)
- [Table Driven Tests](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
- [DynamoDB Local Setup](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/DynamoDBLocal.html)

---

**Document Version**: 1.0
**Last Updated**: 2025-11-01
**Author**: Claude (AI Assistant)
**Status**: Proposal for Review
