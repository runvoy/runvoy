# Local Testing Guide

This guide shows how to run and test your Lambda-based Go application locally using the proposed code structure.

## Quick Start

### 1. Build and Run Local Server

```bash
# Build all components
just build

# Run local development server
just run-local
```

The local server will start on `http://localhost:8080` with mock implementations of all AWS services.

### 2. Test the API

```bash
# Health check
curl http://localhost:8080/health

# Execute a command (with mock API key)
curl -X POST http://localhost:8080/executions \
  -H "Content-Type: application/json" \
  -H "X-API-Key: test-key" \
  -d '{"command": "echo hello world"}'
```

### 3. Run Integration Tests

```bash
# Run local integration tests
just test-local
```

## Development Workflow

### 1. Making Changes to Business Logic

When you modify code in `internal/services/`, you can test it immediately:

```bash
# Make your changes to internal/services/execution.go
# Then rebuild and test
just build-local
just run-local
```

### 2. Testing with Real AWS Services

To test with real AWS services locally, you can create a hybrid setup:

```go
// In local/main.go, replace mocks with real AWS services
func main() {
    // Use real AWS services for testing
    storage := aws.NewDynamoDBStorage()
    ecs := aws.NewECSService()
    // ... rest of the setup
}
```

### 3. Unit Testing

Each service can be unit tested independently:

```go
// Example unit test for ExecutionService
func TestExecutionService_StartExecution(t *testing.T) {
    // Create mock dependencies
    mockStorage := &MockStorage{}
    mockECS := &MockECSService{}
    mockLock := &MockLockService{}
    mockLog := &MockLogService{}
    
    // Create service under test
    service := services.NewExecutionService(mockStorage, mockECS, mockLock, mockLog)
    
    // Test the business logic
    req := &api.ExecutionRequest{Command: "echo test"}
    user := &api.User{Email: "test@example.com"}
    
    resp, err := service.StartExecution(context.Background(), req, user)
    assert.NoError(t, err)
    assert.NotEmpty(t, resp.ExecutionID)
}
```

## Benefits of This Structure

### 1. **Separation of Concerns**
- Business logic in `internal/services/` is framework-agnostic
- HTTP handling in `internal/handlers/` is reusable
- AWS-specific code is isolated in `lambda/` and `local/`

### 2. **Easy Testing**
- Unit tests for business logic without AWS dependencies
- Integration tests with local server and mocks
- Easy to swap implementations for different test scenarios

### 3. **Local Development**
- Fast feedback loop with local server
- No AWS costs during development
- Easy debugging with standard Go tools

### 4. **Production Deployment**
- Lambda function uses the same business logic
- No code duplication between local and production
- Easy to maintain consistency

## File Organization Benefits

```
cmd/
├── runvoy/        # CLI client
│   └── cmd/       # CLI commands
└── backend/       # Backend service
    ├── main.go    # Backend entry point
    └── aws/       # AWS service implementations

internal/
├── api/           # API contracts (shared between client and server)
├── services/      # Business logic (testable, framework-agnostic)
├── handlers/      # HTTP handling (reusable)
└── config/        # Configuration management

local/             # Local development
├── main.go        # Local server entry point
└── mocks/         # Mock implementations
```

This structure makes it easy to:
- Test business logic in isolation
- Run the same code locally and in the backend service
- Maintain consistency between environments
- Add new features without breaking existing functionality