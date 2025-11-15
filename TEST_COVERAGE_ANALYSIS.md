# Runvoy Test Structure and Coverage Analysis

## Executive Summary

**Overall Coverage: 49.8%** (2,422/4,859 statements)

The runvoy codebase has a solid testing foundation with strong patterns established for CLI commands and some core services. However, several critical areas lack adequate test coverage, particularly the event processor status determination logic (0%), DynamoDB repository operations (49.2%), API request/response handling (59.6%), and AWS app integration (39.0%).

---

## 1. Test File Locations and Patterns

### Current Test Distribution

**Test files count:** 59 test files across the codebase

**Main test locations:**
```
cmd/cli/cmd/          - 13 test files (CLI command tests)
internal/app/         - 4 test files (business logic)
internal/server/      - 4 test files (HTTP handlers)
internal/api/         - 6 test files (API layer)
internal/database/    - 1 test file
internal/providers/   - 20+ test files (AWS implementation)
internal/auth/        - 1 test file
internal/config/      - 2 test files
internal/client/      - 2 test files
```

### Test Organization Pattern

The project follows Go conventions:
- Tests are colocated with source code (file_test.go)
- Test utilities are centralized in `internal/testutil/`
- Fixtures and builders available for consistent test setup
- Manual mocking pattern (no external mocking frameworks)

---

## 2. Main Coverage Gaps

### Critical Gaps (0% Coverage)

#### A. Event Processor Status Determination (0%)
**Location:** `internal/providers/aws/events/backend.go`

**Functions at 0%:**
- `determineStatusAndExitCode()` - Core status determination logic
- Partial coverage in `handleECSTaskEvent()` and related methods

**Why This Matters:**
- Responsible for determining final execution status (SUCCEEDED/FAILED/STOPPED)
- Critical for accurate execution tracking and user feedback
- Currently tested only through integration paths

**Current Code Complexity:**
- 48 lines of status determination logic
- Handles multiple stop codes and exit code scenarios
- Edge cases: missing exit codes, missing startedAt timestamps

**Existing Tests:** Limited to basic `TestParseTime` and `TestExtractExecutionIDFromTaskArn` (utility functions only)

---

#### B. User Management (0%)
**Location:** `internal/app/users.go`

**Functions at 0%:**
- `CreateUser()` (78 lines) - Creates new users with API keys
- `ClaimAPIKey()` (50+ lines) - Claims pending API keys
- `validateCreateUserRequest()` - Input validation
- `generateOrUseAPIKey()` - API key generation
- `createPendingClaim()` - Pending claim creation
- `ListUsers()` - Lists all users (only 0% in this package; handlers have some coverage)

**Why This Matters:**
- Core authentication and user management system
- Complex business logic with multiple dependencies
- No direct unit tests despite being heavily used

---

#### C. Secrets Management (0%)
**Location:** `internal/app/secrets.go`

**Functions at 0%:**
- `CreateSecret()` - Creates secrets in Parameter Store
- `GetSecret()` - Retrieves and decrypts secrets
- `ListSecrets()` - Lists all secrets
- `UpdateSecret()` - Updates secret values
- `DeleteSecret()` - Deletes secrets

**Why This Matters:**
- First-party secrets management system
- Integration with AWS Parameter Store and KMS
- Critical for secure credential handling

---

#### D. Secrets Handler (0%)
**Location:** `internal/server/handlers_secrets.go`

**Functions at 0%:**
- `handleCreateSecret()` (25 lines)
- `handleGetSecret()` (26 lines)
- `handleListSecrets()` (22 lines)
- `handleUpdateSecret()` (31 lines)
- `handleDeleteSecret()` (27 lines)
- `handleServiceError()` - Error handling

**Why This Matters:**
- HTTP endpoints for secrets management
- Request/response handling
- Error scenarios and edge cases

---

### High-Priority Gaps (Below 70%)

#### DynamoDB Repository Operations (49.2%)
**Location:** `internal/providers/aws/database/dynamodb/`

**Well-tested files:**
- `connections_test.go` - WebSocket connections
- `users_integration_test.go` - User operations

**Gaps:**
- `executions.go` - Execution CRUD operations (~60% coverage)
- `images.go` - Image metadata operations (~67% coverage)

**Specific function gaps:**
- Execution updates with partial fields
- Error handling paths
- Complex query operations (GSI queries)

---

#### API Request/Response Handling (59.6%)
**Location:** `internal/server/handlers.go`

**Coverage breakdown:**
- `handleRunCommand()` - 57.9%
- `handleGetExecutionLogs()` - 60.0%
- `handleGetExecutionStatus()` - 60.0%
- `handleKillExecution()` - 66.7%
- `handleListExecutions()` - 100.0% ✓
- `handleHealth()` - 100.0% ✓
- `handleCreateUser()` - 26.3%
- `handleClaimAPIKey()` - 0%
- `handleRegisterImage()` - 60.0%
- `handleListImages()` - 45.5%
- `handleGetImage()` - 77.3%
- `handleRemoveImage()` - 59.1%
- `handleRevokeUser()` - 57.1%

**Common gaps:**
- Error path coverage
- Edge case handling
- Missing field scenarios
- Authentication/authorization edge cases

---

#### AWS App Integration (39.0%)
**Location:** `internal/providers/aws/app/`

**Critical functions at 0%:**
- `registerNewImage()` - Task definition registration
- `RegisterImage()` - Image registration workflow
- `registerTaskDefinitionWithRoles()` - Complex task definition creation
- ECS client adapter methods

**Partially covered:**
- `RemoveImage()` - 51.8%
- `GetTaskDefinitionARNForImage()` - 85.7%
- `handleExistingImage()` - 0%

**Why This Matters:**
- Manages ECS task definitions dynamically
- Handles image registration with CPU/memory allocation
- Complex AWS API integration

---

#### Secrets Repositories (0% in app service layer, 90.6% in AWS-specific)

**Location:** `internal/providers/aws/secrets/`

**Status:**
- AWS secrets manager implementation has reasonable coverage (90.6%)
- But app-layer service tests are at 0%

---

### Moderate Coverage Areas (70-80%)

**Well-covered:**
- `internal/logger` - 98.6% ✓
- `internal/errors` - 94.4% ✓
- `internal/constants` - 87.5% ✓
- `internal/auth` - 78.6% ✓
- `internal/client` - 78.6% ✓
- `internal/config` - 78.4% ✓

**Needs improvement:**
- `internal/providers/aws/events` - 75.0% (missing status determination tests)
- `internal/providers/aws/websocket` - 68.2%
- `internal/client/output` - 75.3%

---

## 3. Testing Frameworks and Conventions

### Testing Framework Stack

**Primary Framework:**
- **Standard library `testing` package** - All tests use Go's built-in testing
- **testify** (v1.11.1) - Assertions (`assert`, `require`)

**Key Dependencies:**
```go
github.com/stretchr/testify v1.11.1  // Assertions
```

### Testing Patterns Used

#### 1. Manual Mocking Pattern
No external mocking frameworks (mockgen, testify/mock). All mocks are manually written.

**Example from `internal/app/mocks_test.go`:**
```go
type mockUserRepository struct {
    createUserFunc func(ctx context.Context, user *api.User, apiKeyHash string) error
    getUserByEmailFunc func(ctx context.Context, email string) (*api.User, error)
}

func (m *mockUserRepository) CreateUser(ctx context.Context, user *api.User, apiKeyHash string) error {
    if m.createUserFunc != nil {
        return m.createUserFunc(ctx, user, apiKeyHash)
    }
    return nil
}
```

**Advantages:**
- No code generation required
- Simple and explicit
- Easy to debug
- Compiler catches missing interface methods

**Disadvantages:**
- More boilerplate code
- Manual for each interface
- No automatic verification of calls

---

#### 2. Builder Pattern for Test Data
**Location:** `internal/testutil/fixtures.go`

**Available Builders:**
- `UserBuilder` - Fluent API for creating test users
- `ExecutionBuilder` - Fluent API for creating test executions

**Example:**
```go
user := testutil.NewUserBuilder().
    WithEmail("test@example.com").
    WithLastUsed(time.Now()).
    Build()

execution := testutil.NewExecutionBuilder().
    WithExecutionID("exec-123").
    WithStatus("RUNNING").
    Build()
```

---

#### 3. Table-Driven Tests
Standard Go pattern for parameterized testing.

**Example from `internal/providers/aws/events/backend_test.go`:**
```go
tests := []struct {
    name           string
    event          ECSTaskStateChangeEvent
    expectedStatus string
    expectedExit   int
}{
    {
        name: "successful execution with exit code 0",
        event: ECSTaskStateChangeEvent{
            StopCode: "EssentialContainerExited",
            Containers: []ContainerDetail{{
                Name:     awsConstants.RunnerContainerName,
                ExitCode: intPtr(0),
            }},
        },
        expectedStatus: string(constants.ExecutionSucceeded),
        expectedExit:   0,
    },
    // ... more test cases
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        status, exitCode := determineStatusAndExitCode(&tt.event)
        assert.Equal(t, tt.expectedStatus, status)
        assert.Equal(t, tt.expectedExit, exitCode)
    })
}
```

---

#### 4. Test Utilities
**Location:** `internal/testutil/`

**Available Functions:**
- `TestContext()` - Creates context with timeout
- `TestLogger()` - Creates logger for tests
- `SilentLogger()` - Creates discarding logger
- `AssertErrorType()` - Error type assertions
- `AssertAppErrorCode()` - App error code checks
- `AssertAppErrorStatus()` - HTTP status code checks

---

#### 5. Assertion Style
Consistent use of `assert` (non-fatal) rather than `require` (fatal):

```go
// From tests
assert.NoError(t, err)
assert.Equal(t, expected, actual)
assert.Nil(t, value)
assert.NotNil(t, value)
assert.True(t, condition)
assert.False(t, condition)
```

---

### Assertion Helpers

**Custom assertions in `internal/testutil/assert.go`:**
- `AssertErrorType()` - Check error type with `errors.Is`
- `AssertAppErrorCode()` - Verify error code
- `AssertAppErrorStatus()` - Verify HTTP status code

---

## 4. Current Test Patterns and Best Practices

### Strengths

1. **CLI Commands Well-Tested** (70%+ coverage)
   - Service pattern with dependency injection
   - Separated business logic from cobra integration
   - Clear test structure

2. **Configuration and Constants** (87.5% - 98.6%)
   - Comprehensive validation
   - Edge case handling

3. **Logging and Errors** (94.4% - 98.6%)
   - Structured error handling
   - Consistent error codes and status mapping

4. **Authentication** (78.6% - 92.3%)
   - API key validation
   - User lookup logic

5. **Consistent Naming**
   - Test files follow `*_test.go` convention
   - Test functions follow `Test*` convention
   - Clear, descriptive test names

---

### Weaknesses

1. **Event Processing Undertested**
   - Status determination logic at 0%
   - Only basic utility functions tested
   - Complex business logic paths untested

2. **User and Secrets Management**
   - Critical business logic at 0%
   - No unit tests for user creation, API key management
   - Secrets CRUD operations untested

3. **Database Layer**
   - Mixed coverage (50-90%)
   - Some integration tests but missing unit test isolation
   - Complex query scenarios untested

4. **HTTP Handler Coverage**
   - Average 60% across handlers
   - Error paths often uncovered
   - Request validation gaps

5. **AWS Integration**
   - Task definition management at 0%
   - ECS client adapter methods untested
   - Image registration workflow untested

---

## 5. Recommended Testing Approach

### Based on Current Codebase Patterns

#### For Event Processor (Status Determination)

**Pattern to Follow:**
- Use existing `mockExecutionRepo` and `mockWebSocketHandler` from `backend_test.go`
- Table-driven tests for all `determineStatusAndExitCode()` cases
- Integration tests for full event flow

**Example Template:**
```go
func TestDetermineStatusAndExitCode_AllCases(t *testing.T) {
    tests := []struct {
        name           string
        stopCode       string
        containers     []ContainerDetail
        expectedStatus string
        expectedExit   int
    }{
        // Add missing cases:
        // - Multiple containers with different statuses
        // - Edge cases with nil/missing fields
        // - All stop codes
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            status, exitCode := determineStatusAndExitCode(&tt.event)
            assert.Equal(t, tt.expectedStatus, status)
            assert.Equal(t, tt.expectedExit, exitCode)
        })
    }
}
```

---

#### For User Management

**Pattern to Follow:**
- Reuse `mockUserRepository` from `internal/app/mocks_test.go`
- Builder pattern for test users
- Test user creation flow with validation

**Example Template:**
```go
func TestCreateUser_Valid(t *testing.T) {
    mockRepo := &mockUserRepository{
        createUserWithExpirationFunc: func(...) error { return nil },
        createPendingAPIKeyFunc: func(...) error { return nil },
    }
    
    service := app.NewService(mockRepo, ...)
    
    user, token, err := service.CreateUser(testutil.TestContext(), "user@example.com", "")
    
    assert.NoError(t, err)
    assert.NotNil(t, user)
    assert.NotEmpty(t, token)
    assert.Equal(t, "user@example.com", user.Email)
    assert.False(t, user.Revoked)
}

func TestCreateUser_InvalidEmail(t *testing.T) {
    mockRepo := &mockUserRepository{}
    service := app.NewService(mockRepo, ...)
    
    _, _, err := service.CreateUser(testutil.TestContext(), "invalid", "")
    
    assert.Error(t, err)
    assert.True(t, testutil.AssertAppErrorCode(t, err, "BAD_REQUEST"))
}
```

---

#### For DynamoDB Operations

**Pattern to Follow:**
- Mock DynamoDB client to avoid AWS dependency
- Test conversion functions (toExecutionItem, toAPIExecution)
- Test query building and parsing

**Example Template:**
```go
func TestUpdateExecution(t *testing.T) {
    tests := []struct {
        name      string
        execution *api.Execution
        expectErr bool
    }{
        {
            name: "update running execution to succeeded",
            execution: testutil.NewExecutionBuilder().
                WithStatus("RUNNING").
                WithExitCode(0).
                Completed().
                Build(),
            expectErr: false,
        },
        {
            name: "execution not found",
            execution: testutil.NewExecutionBuilder().Build(),
            expectErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            repo := &executionRepository{client: mockDynamoClient}
            err := repo.UpdateExecution(testutil.TestContext(), tt.execution)
            
            if tt.expectErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

---

#### For HTTP Handlers

**Pattern to Follow:**
- Use `httptest.NewRequest()` and `httptest.NewRecorder()`
- Mock service layer
- Test request/response serialization
- Test error responses

**Example Template:**
```go
func TestHandleCreateUser(t *testing.T) {
    tests := []struct {
        name           string
        body           interface{}
        mockError      error
        expectedStatus int
        expectedBody   string
    }{
        {
            name: "successful creation",
            body: `{"email":"user@example.com"}`,
            mockError: nil,
            expectedStatus: http.StatusCreated,
        },
        {
            name: "invalid email",
            body: `{"email":"invalid"}`,
            mockError: apperrors.ErrBadRequest,
            expectedStatus: http.StatusBadRequest,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockSvc := &mockService{...}
            
            req := httptest.NewRequest("POST", "/api/v1/users/create", bytes.NewBufferString(tt.body))
            req.Header.Set("X-API-Key", "test-key")
            w := httptest.NewRecorder()
            
            handler := server.NewRouter(mockSvc, time.Minute)
            handler.ServeHTTP(w, req)
            
            assert.Equal(t, tt.expectedStatus, w.Code)
        })
    }
}
```

---

#### For AWS Integration (Task Definitions)

**Pattern to Follow:**
- Mock ECS client
- Test task definition registration workflow
- Test sanitization and naming
- Test image metadata persistence

**Example Template:**
```go
func TestRegisterImage(t *testing.T) {
    tests := []struct {
        name           string
        image          string
        cpu            int
        memory         int
        expectedFamily string
        expectErr      bool
    }{
        {
            name: "register ubuntu image",
            image: "ubuntu:22.04",
            cpu: 256,
            memory: 512,
            expectedFamily: "runvoy-image-ubuntu-22-04",
            expectErr: false,
        },
        {
            name: "invalid cpu",
            image: "ubuntu:22.04",
            cpu: 128, // Invalid
            memory: 512,
            expectErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockECS := &mockECSClient{...}
            runner := &AWSRunner{ecsClient: mockECS}
            
            err := runner.RegisterImage(testutil.TestContext(), tt.image, tt.cpu, tt.memory)
            
            if tt.expectErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                // Verify task definition was registered
            }
        })
    }
}
```

---

## 6. Priority Areas to Tackle

### Phase 1 (Critical - 40-50 hours)

1. **Event Processor Status Determination** (0% → 100%)
   - 15-20 hours
   - High impact on execution tracking
   - Relatively isolated logic
   - Clear test patterns already established

2. **User Management Service** (0% → 100%)
   - 15-20 hours
   - Core authentication
   - Uses well-established mock pattern
   - Impacts user creation/revocation workflows

3. **Secrets Management** (0% → 100%)
   - 10-15 hours
   - Reuses user management patterns
   - Critical for secure credential handling

### Phase 2 (Important - 30-40 hours)

4. **HTTP Handler Coverage** (59.6% → 85%+)
   - 15-20 hours
   - Focus on error paths
   - Add edge case scenarios

5. **DynamoDB Repository Coverage** (49.2% → 80%+)
   - 10-15 hours
   - Improve execution repository tests
   - Add image/metadata tests

### Phase 3 (Important - 20-30 hours)

6. **AWS App Integration** (39.0% → 75%+)
   - 15-20 hours
   - Task definition registration
   - Image management workflow
   - Complex AWS API interactions

7. **WebSocket Handling** (68.2% → 85%+)
   - 10-15 hours
   - Connection lifecycle
   - Message routing

---

## 7. Key Test Data and Fixtures

### Predefined Test Data

**Location:** `internal/testutil/fixtures.go`

**Available Builders:**

```go
// User builder
user := testutil.NewUserBuilder().
    WithEmail("user@example.com").
    WithCreatedAt(time.Now()).
    WithLastUsed(time.Now()).
    Revoked().
    Build()

// Execution builder
exec := testutil.NewExecutionBuilder().
    WithExecutionID("exec-123").
    WithCommand("echo test").
    WithStatus("RUNNING").
    WithUserEmail("user@example.com").
    WithLogStreamName("stream/name").
    Completed().
    Build()
```

---

### Suggested Additional Test Data Builders

For improving test coverage, consider adding:

```go
// ImageBuilder for image registration tests
testutil.NewImageBuilder().
    WithName("ubuntu:22.04").
    WithCPU(256).
    WithMemory(512).
    Build()

// ECSTaskEventBuilder for event processor tests
testutil.NewECSTaskEventBuilder().
    WithExecutionID("exec-123").
    WithStatus("STOPPED").
    WithExitCode(0).
    WithStopCode("EssentialContainerExited").
    Build()

// CloudWatchEventBuilder for event routing tests
testutil.NewCloudWatchEventBuilder().
    WithDetailType("ECS Task State Change").
    WithSource("aws.ecs").
    Build()
```

---

## 8. Testing Infrastructure Notes

### Test Execution

**Run tests:**
```bash
just test
```

**Generate coverage:**
```bash
just gen-coverage
```

**Check coverage threshold:**
```bash
just test-coverage
```

**Configuration:** `.testcoverage.yml`
```yaml
profile: coverage.out
threshold:
  total: 45  # Current threshold: 45%
```

---

### Continuous Integration

**Workflow:** `.github/workflows/coverage.yml`
- Runs on pull requests
- Uses `vladopajic/go-test-coverage` action
- Checks against configured threshold

---

## 9. Implementation Roadmap

### Quick Wins (High Impact, Low Effort)

1. **Event Processor Status Logic** (Highest Impact)
   - 20-30 test cases needed
   - Isolated function, easy to test
   - Already has test structure in place

2. **User Creation Flow** (High Impact)
   - Uses existing mock patterns
   - Well-defined requirements
   - 15-20 test cases

3. **Add Missing Test Builders**
   - ImageBuilder
   - ECSTaskEventBuilder
   - Reduce test setup boilerplate

---

### Strategic Improvements

1. **Handler Testing Pattern**
   - Establish standard pattern for all handlers
   - Create handler test template
   - Systematically cover all handlers

2. **Error Path Coverage**
   - Audit all error handling paths
   - Add 2-3 error scenarios per handler
   - Test validation edge cases

3. **Integration Test Suite**
   - End-to-end execution flows
   - Event processor to completion
   - User creation to API usage

---

## 10. Summary Table

| Area | Coverage | Files | Status | Effort |
|------|----------|-------|--------|--------|
| Event Processor | 0% | 1 | CRITICAL | 15-20h |
| User Management | 0% | 1 | CRITICAL | 15-20h |
| Secrets Service | 0% | 2 | CRITICAL | 10-15h |
| HTTP Handlers | 59.6% | 1 | HIGH | 15-20h |
| DynamoDB Repos | 49.2% | 4 | HIGH | 10-15h |
| AWS App | 39.0% | 3 | HIGH | 15-20h |
| WebSocket | 68.2% | 2 | MEDIUM | 10-15h |
| CLI Commands | 70%+ | 13 | GOOD | - |
| Core Services | 85%+ | 8 | EXCELLENT | - |

---

## Key Takeaways

1. **Strong CLI Foundation** - CLI tests are well-structured and use good patterns (service injection, mocking, table-driven tests)

2. **Testing Framework**: Standard Go testing + testify assertions with manual mocking (no external mock generation)

3. **Test Patterns**: Well-established manual mocking, builder pattern, table-driven tests

4. **Critical Gaps**: Event processor (0%), user management (0%), secrets (0%) - relatively isolated, testable functions

5. **Quick Path to 70%+**: Focus on event processor status determination, user management, and handler error paths

6. **Recommended Tools**: Stick with current approach (manual mocks, testify, table-driven), no framework changes needed

7. **Test Data**: Use existing builders in testutil for consistency; can extend with image/event builders
