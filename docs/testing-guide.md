# Testing Guide

This guide explains the testing strategy and tools used in the runvoy project.

## Testing Strategy

We use a multi-layered testing approach:

1. **Unit Tests** - Test individual functions and methods in isolation
2. **Integration Tests** - Test component interactions with mocks
3. **End-to-End Tests** - Test with real services using testcontainers

## Testing Tools

### 1. **httptest** (Standard Library)
- **Purpose**: HTTP server testing
- **Location**: `internal/testing/testserver.go`
- **Benefits**: Built-in, no dependencies, automatic port management

```go
func TestAPI(t *testing.T) {
    ts := testing.NewTestServer(t)
    defer ts.Close()
    
    resp, err := ts.Client().Post(ts.URL()+"/executions", "application/json", body)
    // Test response
}
```

### 2. **testify/mock**
- **Purpose**: Mock implementations
- **Location**: `internal/testing/mocks.go`
- **Benefits**: Auto-generated mocks, built-in assertions, less boilerplate

```go
// Setup mock expectations
mockAuth.On("ValidateAPIKey", mock.Anything, "test-key").Return(user, nil)

// Verify calls
mockAuth.AssertExpectations(t)
```

### 3. **testcontainers-go** (Optional)
- **Purpose**: Integration testing with real services
- **Location**: `tests/integration/testcontainers_test.go`
- **Benefits**: Real service testing, Docker-based, more realistic

```go
func TestWithRealDynamoDB(t *testing.T) {
    dynamoContainer, err := dynamodb.RunContainer(ctx, "amazon/dynamodb-local:latest")
    // Test with real DynamoDB
}
```

## Running Tests

### All Tests
```bash
just test
```

### Unit Tests Only
```bash
just test-unit
```

### Integration Tests Only
```bash
just test-integration
```

### With Coverage
```bash
just test-coverage
```

## Test Structure

```
tests/
├── unit/                    # Unit tests
│   └── services/           # Service layer tests
│       └── execution_test.go
└── integration/            # Integration tests
    ├── execution_test.go   # API integration tests
    └── testcontainers_test.go  # Real service tests
```

## Writing Tests

### Unit Test Example

```go
func TestExecutionService_StartExecution(t *testing.T) {
    // Create mocks
    mockStorage := testing.NewMockStorageService(t)
    mockECS := testing.NewMockECSService(t)
    
    // Create service under test
    service := services.NewExecutionService(mockStorage, mockECS, ...)
    
    // Setup expectations
    mockECS.On("StartTask", mock.Anything, req, mock.Anything, mock.Anything).Return("arn:aws:ecs:test", nil)
    
    // Execute
    resp, err := service.StartExecution(ctx, req, user)
    
    // Assert
    assert.NoError(t, err)
    assert.NotEmpty(t, resp.ExecutionID)
    
    // Verify mocks
    mockECS.AssertExpectations(t)
}
```

### Integration Test Example

```go
func TestExecutionAPI(t *testing.T) {
    // Create test server
    ts := testing.NewTestServer(t)
    defer ts.Close()
    
    // Setup mocks
    ts.Mocks.Auth.On("ValidateAPIKey", mock.Anything, "test-key").Return(user, nil)
    
    // Make HTTP request
    resp, err := ts.Client().Post(ts.URL()+"/executions", "application/json", body)
    
    // Assert response
    assert.Equal(t, http.StatusOK, resp.StatusCode)
}
```

## Benefits of This Approach

### 1. **Fast Unit Tests**
- Mocks eliminate AWS dependencies
- Tests run in milliseconds
- Perfect for TDD workflow

### 2. **Realistic Integration Tests**
- Test real HTTP interactions
- Verify API contracts
- Catch integration issues early

### 3. **Optional Real Service Testing**
- Use testcontainers when needed
- Test with real DynamoDB, ECS, etc.
- More confidence in production behavior

### 4. **Maintainable**
- testify/mock reduces boilerplate
- Clear separation of test types
- Easy to add new tests

## Mock vs Real Service Testing

### Use Mocks When:
- Testing business logic
- Fast feedback needed
- AWS services not available
- Unit testing

### Use Real Services When:
- Testing integration points
- Verifying AWS SDK usage
- End-to-end validation
- Pre-production testing

## Test Data Management

### Fixtures
- Store test data in `tests/fixtures/`
- Use consistent test data across tests
- Make tests deterministic

### Test Database
- Use testcontainers for real database testing
- Clean up after each test
- Use transactions when possible

## Continuous Integration

### GitHub Actions Example
```yaml
- name: Run Tests
  run: |
    just test-unit
    just test-integration
    
- name: Run Tests with Coverage
  run: just test-coverage
```

### Local Development
```bash
# Run tests in watch mode
go test -v ./tests/unit/... -count=1

# Run specific test
go test -v ./tests/unit/services -run TestExecutionService
```

## Best Practices

1. **Test Naming**: Use descriptive test names that explain the scenario
2. **Arrange-Act-Assert**: Structure tests clearly
3. **One Assertion Per Test**: Keep tests focused
4. **Mock External Dependencies**: Don't test AWS, test your code
5. **Clean Up**: Always clean up resources in tests
6. **Deterministic**: Make tests repeatable and predictable
7. **Fast**: Keep unit tests fast (< 100ms)
8. **Independent**: Tests should not depend on each other

## Troubleshooting

### Common Issues

1. **Mock Not Called**: Check `AssertExpectations(t)`
2. **Test Hangs**: Check for goroutines not cleaned up
3. **Flaky Tests**: Make tests deterministic
4. **Slow Tests**: Use mocks instead of real services

### Debug Tips

```go
// Enable mock logging
mockAuth.On("ValidateAPIKey", mock.Anything, mock.Anything).Return(user, nil).Maybe()

// Check mock calls
calls := mockAuth.Calls
t.Logf("Mock called %d times", len(calls))

// Use testify debug
assert.Equal(t, expected, actual, "Debug message")
```