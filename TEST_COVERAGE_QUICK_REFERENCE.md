# Runvoy Test Coverage - Quick Reference Guide

## Current Status
- **Overall Coverage:** 49.8% (2,422/4,859 statements)
- **Threshold:** 45% (being met)
- **Target:** 70%+

## Critical Gaps (Highest Priority)

### 1. Event Processor - Status Determination (0%)
**File:** `internal/providers/aws/events/backend.go`

**Missing Tests:**
- `determineStatusAndExitCode()` function and related logic
- All stop codes: UserInitiated, EssentialContainerExited, TaskFailedToStart
- Edge cases: missing exit codes, multiple containers, nil fields

**Impact:** Core execution status tracking

**Effort:** 15-20 hours

**Test Pattern:**
```go
// Use existing mockExecutionRepo and mockWebSocketHandler
// Table-driven tests for all status/exit code combinations
// Already has test utilities and patterns in place
```

---

### 2. User Management (0%)
**File:** `internal/app/users.go`

**Missing Tests:**
- `CreateUser()` - User creation with API key generation
- `ClaimAPIKey()` - Claiming pending API keys
- `validateCreateUserRequest()` - Input validation
- `generateOrUseAPIKey()` - API key generation logic
- `createPendingClaim()` - Pending claim workflow

**Impact:** Authentication, user management

**Effort:** 15-20 hours

**Test Pattern:**
```go
// Reuse mockUserRepository from internal/app/mocks_test.go
// Use testutil.UserBuilder for test data
// Error cases: invalid email, user exists, database errors
```

---

### 3. Secrets Management (0%)
**File:** `internal/app/secrets.go` and `internal/server/handlers_secrets.go`

**Missing Tests:**
- `CreateSecret()` - Parameter Store integration
- `GetSecret()` - Retrieval and decryption
- `ListSecrets()` - Listing with metadata
- `UpdateSecret()` - Updates with rotation
- `DeleteSecret()` - Cleanup
- All secrets HTTP handlers

**Impact:** Secure credential handling

**Effort:** 10-15 hours

**Test Pattern:**
```go
// Mock AWS Parameter Store client
// Mock KMS for encryption/decryption
// Test metadata + value store separation
```

---

## High Priority Gaps

### 4. HTTP Handlers - Error Paths (59.6%)
**File:** `internal/server/handlers.go`

**Functions Below 70%:**
- `handleRunCommand()` - 57.9%
- `handleGetExecutionLogs()` - 60.0%
- `handleGetExecutionStatus()` - 60.0%
- `handleKillExecution()` - 66.7%
- `handleCreateUser()` - 26.3%
- `handleClaimAPIKey()` - 0%
- `handleRegisterImage()` - 60.0%
- `handleListImages()` - 45.5%
- `handleRemoveImage()` - 59.1%
- `handleRevokeUser()` - 57.1%

**Missing Tests:** Error scenarios, validation failures, edge cases

**Effort:** 15-20 hours

**Test Pattern:**
```go
// Use httptest.NewRequest/NewRecorder
// Mock service layer with error returns
// Test all error response types
// Verify status codes and error messages
```

---

### 5. DynamoDB Repositories (49.2%)
**File:** `internal/providers/aws/database/dynamodb/*.go`

**Problem Areas:**
- `executions.go` - Execution CRUD (~60%)
- `images.go` - Image metadata (~67%)
- Complex query operations
- Error handling paths

**Effort:** 10-15 hours

**Test Pattern:**
```go
// Mock DynamoDB client
// Test conversion functions (toExecutionItem, toAPIExecution)
// Test query building and parsing
// Error cases: ConditionalCheckFailed, ValidationException
```

---

### 6. AWS App Integration (39.0%)
**File:** `internal/providers/aws/app/`

**Critical Functions at 0%:**
- `registerNewImage()` - Task definition creation
- `RegisterImage()` - Workflow
- `registerTaskDefinitionWithRoles()` - ECS API integration
- `handleExistingImage()` - Update scenarios

**Partially Covered:**
- `RemoveImage()` - 51.8%
- `GetTaskDefinitionARNForImage()` - 85.7%

**Effort:** 15-20 hours

**Test Pattern:**
```go
// Mock ECS client
// Test task definition naming and sanitization
// Test image metadata persistence
// Test CPU/memory validation
```

---

## Testing Framework in Use

### Stack
- **Go testing** - Standard library (all tests)
- **testify** - Assertions only (v1.11.1)
- **Manual mocks** - No mockgen or external frameworks

### Key Patterns

1. **Manual Mocking**
   - Simple function fields in structs
   - No auto-verification
   - Example: `type mockUserRepository struct { createUserFunc func(...) error }`

2. **Builder Pattern**
   - `testutil.NewUserBuilder()`
   - `testutil.NewExecutionBuilder()`
   - Fluent API for test data

3. **Table-Driven Tests**
   - Parameterized test cases
   - Multiple scenarios per test function
   - Clear test case names

4. **Test Utilities**
   - `testutil.TestContext()` - Context with timeout
   - `testutil.TestLogger()` - Error-level logger
   - `testutil.SilentLogger()` - No output
   - `testutil.AssertAppErrorCode()` - Error assertions

---

## Recommended Test Implementation Order

### Phase 1: Critical (40-50 hours)
1. **Event Processor Status** (15-20h)
   - Highest impact
   - Isolated logic
   - Clear test patterns exist

2. **User Management** (15-20h)
   - Core system
   - Well-established patterns
   - Impacts authentication

3. **Secrets** (10-15h)
   - Critical security
   - Reuses user patterns
   - Clear requirements

### Phase 2: Important (30-40 hours)
4. **HTTP Handlers** (15-20h)
   - Coverage gaps across multiple handlers
   - Error path focus

5. **DynamoDB Repos** (10-15h)
   - Improve execution/image operations
   - Query testing

### Phase 3: Polish (20-30 hours)
6. **AWS App Integration** (15-20h)
   - Complex workflows
   - ECS integration

7. **WebSocket** (10-15h)
   - Connection lifecycle
   - Message routing

---

## Quick Start: Writing New Tests

### 1. Mock Setup Pattern
```go
mockRepo := &mockExecutionRepository{
    getExecutionFunc: func(ctx context.Context, id string) (*api.Execution, error) {
        if id == "error" {
            return nil, fmt.Errorf("not found")
        }
        return &api.Execution{ExecutionID: id}, nil
    },
}
```

### 2. Test Data Pattern
```go
execution := testutil.NewExecutionBuilder().
    WithExecutionID("exec-123").
    WithStatus("RUNNING").
    Completed().
    Build()

user := testutil.NewUserBuilder().
    WithEmail("user@example.com").
    Build()
```

### 3. Test Function Pattern
```go
func TestFunctionName_Scenario(t *testing.T) {
    // Arrange
    mockRepo := &mockRepository{...}
    svc := service.New(mockRepo)
    
    // Act
    result, err := svc.DoSomething(testutil.TestContext(), arg)
    
    // Assert
    assert.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

### 4. Error Test Pattern
```go
func TestFunctionName_ErrorCase(t *testing.T) {
    mockRepo := &mockRepository{
        methodFunc: func(...) error {
            return apperrors.ErrDatabaseError
        },
    }
    
    _, err := svc.DoSomething(testutil.TestContext(), arg)
    
    assert.Error(t, err)
    assert.True(t, testutil.AssertAppErrorCode(t, err, "DATABASE_ERROR"))
}
```

---

## Test Execution Commands

```bash
# Run all tests
just test

# Generate coverage
just gen-coverage

# View HTML coverage
open coverage.html

# Check coverage meets threshold
just test-coverage

# Run specific test
go test -run TestName ./path/to/package

# Run with verbose output
go test -v ./...
```

---

## File Locations Reference

| Component | Files | Coverage |
|-----------|-------|----------|
| Event Processor | `internal/providers/aws/events/backend.go` | 75% (status logic: 0%) |
| User Management | `internal/app/users.go` | 0% |
| Secrets | `internal/app/secrets.go` | 0% |
| Secrets Handlers | `internal/server/handlers_secrets.go` | 0% |
| HTTP Handlers | `internal/server/handlers.go` | 59.6% |
| DynamoDB Repos | `internal/providers/aws/database/dynamodb/*.go` | 49.2% |
| AWS App | `internal/providers/aws/app/*.go` | 39.0% |
| WebSocket | `internal/providers/aws/websocket/*.go` | 68.2% |
| CLI Commands | `cmd/cli/cmd/*_test.go` | 70%+ |
| Test Utils | `internal/testutil/*.go` | Available |

---

## Success Metrics

### Short-term (Phase 1)
- Event processor: 0% → 100%
- User management: 0% → 100%
- Secrets: 0% → 90%+
- **Overall: 49.8% → ~58%**

### Medium-term (Phase 2)
- HTTP handlers: 59.6% → 85%+
- DynamoDB: 49.2% → 80%+
- **Overall: ~58% → ~65%**

### Long-term (Phase 3)
- AWS app: 39.0% → 75%+
- WebSocket: 68.2% → 85%+
- **Overall: ~65% → 75%+**

---

## Tips & Best Practices

1. **Use builders instead of constructing structs manually**
   - More readable
   - Consistent test data
   - Easy to extend

2. **Write error tests for every error case**
   - Invalid inputs
   - Database failures
   - Missing resources
   - Validation errors

3. **Follow existing patterns**
   - Manual mocks (no mockgen)
   - Table-driven tests
   - testutil helpers

4. **Test names are documentation**
   - Be specific: `TestCreateUser_InvalidEmail` not `TestCreateUser`
   - Include scenario: `TestHandleECSTaskCompletion_Success`

5. **Mock only what you need**
   - Don't mock external dependencies you control
   - Focus on isolating the unit under test

6. **Run tests locally before pushing**
   - `just test` runs full suite
   - `go test -run TestName` for specific tests
   - `just test-coverage` verifies threshold met

