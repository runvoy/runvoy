# Go Test Coverage Assessment

## Current State

### Overall Coverage Summary

Based on the latest coverage analysis, here's the current state:

**Well-Covered Packages (>70%):**
- `internal/logger`: 98.6%
- `internal/errors`: 94.4%
- `internal/constants`: 87.5%
- `internal/client/playbooks`: 87.3%
- `internal/client`: 81.8%
- `internal/auth`: 80.0%
- `internal/client/output`: 73.5%
- `internal/server`: 57.9%
- `internal/app`: 57.0%
- `cmd/cli/cmd`: 55.5%

**Moderately Covered Packages (30-70%):**
- `internal/providers/aws/events`: 49.1%
- `internal/config/aws`: 47.7%
- `internal/providers/aws/websocket`: 18.8%
- `internal/providers/aws/secrets`: 16.3%
- `internal/config`: 15.5%

**Low Coverage Packages (<30%):**
- `internal/providers/aws/app`: 6.4%
- `internal/providers/aws/database/dynamodb`: 6.5%

**No Coverage (0%):**
- `cmd/backend/providers/aws/event_processor`: 0.0% (main entry point)
- `cmd/backend/providers/aws/orchestrator`: 0.0% (main entry point)
- `cmd/cli`: 0.0% (main entry point)
- `cmd/local`: 0.0% (main entry point)
- `internal/app/events`: 0.0%
- `internal/providers/aws/database`: 0.0%
- `internal/providers/aws/lambdaapi`: 0.0%
- `internal/testutil`: 0.0% (test utilities)
- All `scripts/` packages: 0.0% (utility scripts)

## Easy Wins (Implemented)

### 1. Config Package (`internal/config`)
**Status:** ✅ Partially improved

**Added Tests:**
- `normalizeBackendProvider` - Added test for whitespace handling
- `GetConfigPath` - Added basic test

**Remaining Gaps:**
- `Load()` - Complex function requiring file system and environment variable mocking
- `LoadCLI()` - Requires config file mocking
- `LoadOrchestrator()` - Requires environment variable setup
- `LoadEventProcessor()` - Requires environment variable setup
- `Save()` - Requires file system mocking
- `MustLoadOrchestrator()` - Requires os.Exit mocking (challenging)
- `MustLoadEventProcessor()` - Requires os.Exit mocking (challenging)

### 2. AWS App Utilities (`internal/providers/aws/app`)
**Status:** ✅ Improved

**Added Tests:**
- `SanitizeImageNameForTaskDef` - Image name sanitization
- `TaskDefinitionFamilyName` - Family name generation
- `ExtractImageFromTaskDefFamily` - Image extraction from family name
- `buildTaskDefinitionTags` - Tag building logic
- `renderScript` - Template rendering

**Remaining Gaps:**
- Functions requiring AWS SDK mocks (ECS client):
  - `listTaskDefinitionsByPrefix`
  - `GetDefaultImage`
  - `unmarkExistingDefaultImages`
  - `GetTaskDefinitionForImage`
  - `RegisterTaskDefinitionForImage`
  - `DeregisterTaskDefinitionsForImage`
  - `handleDefaultImageTagging`
  - `updateExistingTaskDefTags`
  - `getRoleARNsFromExistingTaskDef`
  - `checkIfImageIsDefault`
  - `deregisterAllTaskDefRevisions`
  - `markLastRemainingImageAsDefault`

### 3. WebSocket Manager (`internal/providers/aws/websocket`)
**Status:** ✅ Improved

**Added Tests:**
- `getClientIPFromWebSocketRequest` - IP extraction
- `newWebSocketConnection` - Connection creation

**Remaining Gaps:**
- Functions requiring AWS SDK mocks (API Gateway Management API):
  - `SendLogsToExecution`
  - `NotifyExecutionCompletion`
  - `sendLogToConnection`
  - `sendDisconnectToConnection`
  - `handleDisconnectNotification`
  - `GenerateWebSocketURL`

### 4. Secrets Manager (`internal/providers/aws/secrets`)
**Status:** ⚠️ Needs improvement

**Current Coverage:** 16.3%

**Remaining Gaps:**
- `StoreSecret` - Requires SSM client mocking
- `RetrieveSecret` - Requires SSM client mocking
- `DeleteSecret` - Requires SSM client mocking

## Harder Parts - Proposals

### 1. Main Entry Points (0% coverage)

**Packages:**
- `cmd/backend/providers/aws/event_processor`
- `cmd/backend/providers/aws/orchestrator`
- `cmd/cli`
- `cmd/local`

**Challenge:** These are application entry points that call `os.Exit()` and are difficult to test in isolation.

**Proposal:**
1. **Extract business logic** from `main()` functions into testable functions
2. **Use dependency injection** to make entry points testable
3. **Create integration tests** that test the full flow with mocked dependencies
4. **Accept low coverage** for entry points if they're thin wrappers

**Example Pattern:**
```go
// Instead of:
func main() {
    cfg := config.MustLoad()
    app.Run(cfg)
    os.Exit(0)
}

// Do:
func main() {
    if err := run(); err != nil {
        os.Exit(1)
    }
}

func run() error {
    cfg, err := config.Load()
    if err != nil {
        return err
    }
    return app.Run(cfg)
}
```

### 2. AWS Integration Code

**Packages:**
- `internal/providers/aws/app` (ECS operations)
- `internal/providers/aws/websocket` (API Gateway)
- `internal/providers/aws/secrets` (SSM Parameter Store)
- `internal/providers/aws/database/dynamodb` (DynamoDB)

**Challenge:** Requires mocking AWS SDK clients, which can be complex.

**Proposal:**
1. **Use interface-based design** - Already partially done with `ValueStore` interface
2. **Create mock implementations** for AWS clients using libraries like:
   - `github.com/aws/aws-sdk-go-v2/service/ecs/ecsiface` (if available)
   - Custom mock structs implementing AWS client interfaces
3. **Use dependency injection** to inject mock clients in tests
4. **Create test fixtures** for common AWS responses

**Example Pattern:**
```go
// Define interface for ECS operations
type ECSClient interface {
    ListTaskDefinitions(ctx context.Context, input *ecs.ListTaskDefinitionsInput) (*ecs.ListTaskDefinitionsOutput, error)
    RegisterTaskDefinition(ctx context.Context, input *ecs.RegisterTaskDefinitionInput) (*ecs.RegisterTaskDefinitionOutput, error)
    // ... other methods
}

// Use interface in code
func RegisterTaskDefinitionForImage(ctx context.Context, ecsClient ECSClient, ...) error {
    // Use ecsClient instead of concrete *ecs.Client
}

// Create mock for tests
type mockECSClient struct {
    listTaskDefinitionsFunc func(...) (*ecs.ListTaskDefinitionsOutput, error)
    // ...
}
```

### 3. WebSocket Streaming Logic

**Package:** `internal/providers/aws/websocket`

**Challenge:** Complex async behavior, connection management, error handling.

**Proposal:**
1. **Unit test individual functions** with mocked dependencies
2. **Integration tests** with local WebSocket server (using libraries like `gorilla/websocket`)
3. **Test error paths** - connection failures, timeouts, invalid tokens
4. **Test concurrent operations** - multiple connections, race conditions

### 4. Event Processing

**Package:** `internal/app/events` (0% coverage)

**Challenge:** Event-driven architecture with multiple event types.

**Proposal:**
1. **Unit test event handlers** individually
2. **Test event routing** logic
3. **Test event validation** and error handling
4. **Integration tests** with event mocks

### 5. Database Operations

**Package:** `internal/providers/aws/database/dynamodb` (6.5% coverage)

**Challenge:** DynamoDB operations require AWS SDK mocking.

**Proposal:**
1. **Use DynamoDB Local** for integration tests
2. **Create mock implementations** of repository interfaces
3. **Test error handling** - throttling, conditional check failures
4. **Test pagination** logic

## Recommendations

### Short-term (Easy Wins)
1. ✅ Add tests for pure utility functions (string manipulation, validation)
2. ✅ Add tests for helper functions that don't require external dependencies
3. ✅ Improve test coverage for `internal/config` package
4. ✅ Add tests for `internal/providers/aws/app` utility functions

### Medium-term (Moderate Effort)
1. **Refactor entry points** to extract testable logic
2. **Create mock implementations** for AWS SDK clients
3. **Add integration tests** for critical paths using local services
4. **Improve secrets manager tests** with SSM mocks

### Long-term (Significant Effort)
1. **Implement comprehensive AWS mocking strategy**
2. **Add integration test suite** with local AWS services (LocalStack, DynamoDB Local)
3. **Add end-to-end tests** for critical user flows
4. **Set up CI/CD coverage reporting** and enforce minimum coverage thresholds

## Coverage Goals

### Target Coverage by Package Type

- **Core business logic:** >80%
- **Utility functions:** >90%
- **API handlers:** >70%
- **AWS integrations:** >60% (with mocks)
- **Entry points:** >50% (after refactoring)
- **Test utilities:** N/A (excluded from coverage)

### Overall Project Goal
- **Current:** ~40-50% (estimated)
- **Short-term target:** 60%
- **Long-term target:** 75%

## Testing Strategy

### Unit Tests
- Fast, isolated tests
- Mock external dependencies
- Test error paths and edge cases
- Target: >80% coverage for pure functions

### Integration Tests
- Test with real AWS services (or LocalStack)
- Test critical user flows
- Run in CI/CD pipeline
- Target: Cover main user journeys

### Test Organization
- Keep tests close to source code (`*_test.go` files)
- Use table-driven tests for multiple scenarios
- Create test fixtures for complex data structures
- Use test helpers for common setup/teardown

## Tools and Libraries

### Recommended Testing Tools
- `github.com/stretchr/testify` - Already in use, good for assertions
- `github.com/aws/aws-sdk-go-v2` - Official AWS SDK (for mocking)
- LocalStack - Local AWS services for integration tests
- DynamoDB Local - Local DynamoDB for testing

### Coverage Tools
- `go test -coverprofile` - Built-in coverage
- `go-test-coverage` - Coverage threshold enforcement (already configured)
- `golangci-lint` - Already configured for code quality

## Notes

- Entry points (`main()` functions) are intentionally low coverage - they're thin wrappers
- Scripts in `scripts/` directory are utility scripts, not part of main application
- Test utilities (`internal/testutil`) are excluded from coverage calculations
- Some packages have "no statements" because they only contain interfaces or type definitions
