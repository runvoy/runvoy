# Runner Interface Refactoring Migration Guide

## Overview

The `Runner` interface has been refactored to separate concerns into focused, cohesive interfaces:

- **TaskExecutor**: Task lifecycle management
- **ImageRegistry**: Image registration and configuration
- **LogAggregator**: Execution log retrieval
- **BackendObservability**: Infrastructure log retrieval

The original `Runner` interface remains for backwards compatibility but is marked as deprecated.

## Changes Summary

### New Interfaces (internal/backend/orchestrator/main.go)

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

// Deprecated: Use specific interfaces above
type Runner interface {
    TaskExecutor
    ImageRegistry
    LogAggregator
    BackendObservability
}
```

### New Configuration Types (internal/backend/orchestrator/image_config.go)

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
    CPU    *int  // Provider-specific units
    Memory *int  // MB
}

type RuntimeConfig struct {
    Platform     *string  // "linux/amd64", "linux/arm64"
    Architecture *string  // Deprecated, use Platform
}

type PermissionConfig struct {
    TaskRole      *string  // App permissions
    ExecutionRole *string  // Infrastructure permissions (AWS-specific)
}
```

Helper functions for backwards compatibility:
- `ImageConfig.ToLegacyParams()` - Convert to old RegisterImage parameters
- `FromLegacyParams()` - Create ImageConfig from old parameters

### Service Updates

Added accessor methods to orchestrator.Service:

```go
func (s *Service) TaskExecutor() TaskExecutor
func (s *Service) ImageRegistry() ImageRegistry
func (s *Service) LogAggregator() LogAggregator
func (s *Service) BackendObservability() BackendObservability
```

## Migration Paths

### Option 1: No Changes (Recommended for now)

Continue using the `Runner` interface as-is. All existing code continues to work.

```go
// Existing code - no changes needed
executionID, createdAt, err := service.runner.StartTask(ctx, userEmail, req)
```

### Option 2: Gradual Migration

Update new code to use specific interfaces through Service accessor methods:

```go
// New code - use specific interfaces
executionID, createdAt, err := service.TaskExecutor().StartTask(ctx, userEmail, req)

images, err := service.ImageRegistry().ListImages(ctx)

logs, err := service.LogAggregator().FetchLogsByExecutionID(ctx, execID)

backendLogs, err := service.BackendObservability().FetchBackendLogs(ctx, reqID)
```

### Option 3: Use ImageConfig (Future)

For image registration, eventually migrate to using ImageConfig:

```go
// Current approach (still works)
err := service.runner.RegisterImage(ctx, image, &isDefault,
    &taskRole, &execRole, &cpu, &memory, &platform, createdBy)

// Future approach (not yet implemented in API layer)
config := &orchestrator.ImageConfig{
    Image: image,
    IsDefault: &isDefault,
    Resources: &orchestrator.ResourceConfig{
        CPU:    &cpu,
        Memory: &memory,
    },
    Runtime: &orchestrator.RuntimeConfig{
        Platform: &platform,
    },
    Permissions: &orchestrator.PermissionConfig{
        TaskRole:      &taskRole,
        ExecutionRole: &execRole,
    },
    RegisteredBy: createdBy,
}
err := service.ImageRegistry().RegisterImageWithConfig(ctx, config)
```

## Benefits

### Testability
Mock only what you need:
```go
// Before: Must mock entire Runner interface (9 methods)
type mockRunner struct { ... }

// After: Mock only what you need
type mockTaskExecutor struct { ... }  // 2 methods
type mockImageRegistry struct { ... } // 4 methods
```

### Separation of Concerns
- Task execution is independent of image management
- Log aggregation is separate from task execution
- Backend observability is isolated from user-facing operations

### Future Provider Support
Each interface can have different implementations:
```go
// AWS implementations
awsExecutor := aws.NewTaskExecutor(...)
awsRegistry := aws.NewImageRegistry(...)

// GCP implementations (future)
gcpExecutor := gcp.NewCloudRunExecutor(...)
gcpRegistry := gcp.NewImageRegistry(...)

// Mix and match
service := orchestrator.NewService(
    taskExecutor: gcpExecutor,
    imageRegistry: awsRegistry,  // Still use AWS for image storage
    ...
)
```

## Testing

Run the test suite to ensure compatibility:

```bash
# Test orchestrator interfaces
go test ./internal/backend/orchestrator/... -v

# Test AWS implementation
go test ./internal/providers/aws/orchestrator/... -v

# Test ImageConfig conversions
go test ./internal/backend/orchestrator/ -run TestImageConfig -v
```

## Timeline

- **Phase 1 (Current)**: New interfaces available, old Runner interface deprecated but functional
- **Phase 2**: Update API handlers to use specific interfaces via Service accessors
- **Phase 3**: Introduce ImageConfig-based API (optional parameter in RegisterImage)
- **Phase 4**: Remove deprecated Runner interface (breaking change, major version bump)

## Questions?

For questions or issues with migration, please:
1. Check the Runner Interface Analysis document
2. Review test examples in `internal/backend/orchestrator/image_config_test.go`
3. Consult the AWS implementation in `internal/providers/aws/orchestrator/runner.go`
