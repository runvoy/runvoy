# Runner Interface Refactoring Guide

## Overview

The orchestrator now uses four focused interfaces instead of a monolithic Runner interface:

- **TaskExecutor**: Task lifecycle management
- **ImageRegistry**: Image registration and configuration
- **LogAggregator**: Execution log retrieval
- **BackendObservability**: Infrastructure log retrieval

This separation improves testability, enables multi-provider support (GCP Cloud Run, Azure Container Instances), and reduces coupling between concerns.

## Architecture

### Focused Interfaces (internal/backend/orchestrator/main.go)

```go
// Core task execution
type TaskExecutor interface {
    StartTask(ctx, userEmail, *ExecutionRequest) (executionID, *time.Time, error)
    KillTask(ctx, executionID) error
}

// Image management
type ImageRegistry interface {
    RegisterImage(ctx, image, *isDefault, *taskRoleName, *taskExecutionRoleName,
                  *cpu, *memory, *runtimePlatform, createdBy) error
    ListImages(ctx) ([]ImageInfo, error)
    GetImage(ctx, image) (*ImageInfo, error)
    RemoveImage(ctx, image) error
}

// Execution logs
type LogAggregator interface {
    FetchLogsByExecutionID(ctx, executionID) ([]LogEvent, error)
}

// Infrastructure logs
type BackendObservability interface {
    FetchBackendLogs(ctx, requestID) ([]LogEvent, error)
}
```

### Service Structure

The `orchestrator.Service` now holds separate interface fields:

```go
type Service struct {
    taskExecutor        TaskExecutor
    imageRegistry       ImageRegistry
    logAggregator       LogAggregator
    backendObservability BackendObservability
    // ... other fields
}
```

Access via methods:
```go
service.TaskExecutor().StartTask(...)
service.ImageRegistry().ListImages(...)
service.LogAggregator().FetchLogsByExecutionID(...)
service.BackendObservability().FetchBackendLogs(...)
```

### Provider-Agnostic Configuration (internal/backend/orchestrator/image_config.go)

```go
type ImageConfig struct {
    Image        string
    IsDefault    *bool
    Resources    *ResourceConfig
    Runtime      *RuntimeConfig
    Permissions  *PermissionConfig
    RegisteredBy string
}

type ResourceConfig struct {
    CPU    *int  // Provider-specific units (see docs)
    Memory *int  // MB (converted to provider units)
}

type RuntimeConfig struct {
    Platform *string  // "linux/amd64", "linux/arm64"
}

type PermissionConfig struct {
    TaskRole      *string  // App permissions
    ExecutionRole *string  // Infrastructure permissions (AWS-only)
}
```

## Multi-Provider Support

### CPU/Memory Mapping

Each provider interprets resource values differently:

| Provider | CPU Unit | CPU Example | Memory Unit | Memory Example |
|----------|----------|-------------|-------------|----------------|
| AWS ECS | ECS units | 256, 512, 1024, 2048 | MB | 512, 1024, 2048 |
| GCP Cloud Run | Millicores | 1000, 2000, 4000 | MB/GB | 512, 1024, 2048 |
| Azure ACI | Cores | 1, 2, 4 | GB | 0.5, 1, 2, 4 |

### Permission Mapping

| Provider | TaskRole | ExecutionRole |
|----------|----------|---------------|
| AWS ECS | IAM role name | IAM execution role |
| GCP Cloud Run | Service account email | (not used) |
| Azure ACI | Managed identity client ID | (not used) |

## Implementation Examples

### AWS ECS Implementation

```go
// internal/providers/aws/orchestrator/runner.go
type Runner struct {
    ecsClient awsClient.ECSClient
    cwlClient awsClient.CloudWatchLogsClient
    iamClient awsClient.IAMClient
    imageRepo ImageTaskDefRepository
    cfg       *Config
    logger    *slog.Logger
}

// Implements TaskExecutor
func (r *Runner) StartTask(...) (string, *time.Time, error) { ... }
func (r *Runner) KillTask(...) error { ... }

// Implements ImageRegistry
func (r *Runner) RegisterImage(...) error { ... }
func (r *Runner) ListImages(...) ([]api.ImageInfo, error) { ... }
func (r *Runner) GetImage(...) (*api.ImageInfo, error) { ... }
func (r *Runner) RemoveImage(...) error { ... }

// Implements LogAggregator
func (r *Runner) FetchLogsByExecutionID(...) ([]api.LogEvent, error) { ... }

// Implements BackendObservability
func (r *Runner) FetchBackendLogs(...) ([]api.LogEvent, error) { ... }
```

### Future GCP Cloud Run Implementation

```go
// internal/providers/gcp/orchestrator/runner.go
type CloudRunExecutor struct {
    client     *run.ServicesClient
    logger     *slog.Logger
    projectID  string
    region     string
}

func (e *CloudRunExecutor) StartTask(...) (string, *time.Time, error) {
    // Create Cloud Run Job execution
    // Map ExecutionRequest to Cloud Run Job spec
    // Return execution ID from Cloud Run
}

func (e *CloudRunExecutor) KillTask(...) error {
    // Cancel running Cloud Run Job execution
}
```

### Future Azure Container Instances Implementation

```go
// internal/providers/azure/orchestrator/executor.go
type ACIExecutor struct {
    client         *armcontainerinstance.ContainerGroupsClient
    logger         *slog.Logger
    subscriptionID string
    resourceGroup  string
}

func (e *ACIExecutor) StartTask(...) (string, *time.Time, error) {
    // Create Container Group
    // Map ExecutionRequest to ACI Container spec
    // Return container group name as execution ID
}

func (e *ACIExecutor) KillTask(...) error {
    // Delete Container Group
}
```

## Benefits

### 1. Testability
Mock only what you need:

```go
// Before: Must mock entire Runner (9 methods)
// After: Mock only TaskExecutor (2 methods)
type mockTaskExecutor struct {
    startTaskFunc func(context.Context, string, *api.ExecutionRequest) (string, *time.Time, error)
    killTaskFunc  func(context.Context, string) error
}

func (m *mockTaskExecutor) StartTask(ctx context.Context, email string, req *api.ExecutionRequest) (string, *time.Time, error) {
    return m.startTaskFunc(ctx, email, req)
}

func (m *mockTaskExecutor) KillTask(ctx context.Context, execID string) error {
    return m.killTaskFunc(ctx, execID)
}
```

### 2. Independent Interface Evolution

Each interface can evolve independently:

```go
// Add streaming logs without touching TaskExecutor
type LogAggregator interface {
    FetchLogsByExecutionID(ctx, executionID) ([]LogEvent, error)
    StreamLogs(ctx, executionID) (<-chan LogEvent, error) // NEW
}
```

### 3. Mix-and-Match Providers

Use different providers for different concerns:

```go
service := orchestrator.NewService(
    taskExecutor:        gcpExecutor,        // Run tasks on GCP
    imageRegistry:       awsImageRegistry,   // Store images in AWS
    logAggregator:       gcpLogAggregator,   // Fetch logs from GCP
    backendObservability: awsObservability,  // Monitor AWS backend
    ...
)
```

### 4. Clear Separation of Concerns

- **Execution** is independent of **image management**
- **User logs** are separate from **infrastructure logs**
- Each component has a single, well-defined responsibility

## Testing

Run the test suite to verify changes:

```bash
# Run all checks (linting, formatting, tests)
just check

# Test specific packages
go test ./internal/backend/orchestrator/... -v
go test ./internal/providers/aws/orchestrator/... -v
```

## Migration from Previous Code

If migrating from code that used the old unified Runner interface:

1. **Update imports**: Ensure you're importing the correct orchestrator package
2. **Use Service accessors**: Call `service.TaskExecutor()`, `service.ImageRegistry()`, etc.
3. **Update constructors**: `NewService` now takes 4 separate interface parameters

Example:

```go
// Before
runner := aws.NewRunner(...)
service := orchestrator.NewService(..., runner, ...)

// After
awsRunner := aws.NewRunner(...)
service := orchestrator.NewService(
    ...,
    awsRunner, // TaskExecutor
    awsRunner, // ImageRegistry
    awsRunner, // LogAggregator
    awsRunner, // BackendObservability
    ...
)
```

The AWS Runner implements all four interfaces, so you pass the same instance four times until you implement separate providers.
