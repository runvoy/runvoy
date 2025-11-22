# Runner Interface Analysis

## Executive Summary

The current `Runner` interface in the orchestrator is **overloaded** and exhibits **coherence issues**. Initially designed to interact with ECS-like services for task execution, it has grown to include image management, log fetching, and infrastructure logging concerns.

## Current Interface

**Location:** `internal/backend/orchestrator/main.go:18-64`

```go
type Runner interface {
    // Execution lifecycle
    StartTask(ctx, userEmail, *ExecutionRequest) (executionID, *time.Time, error)
    KillTask(ctx, executionID) error

    // Image management
    RegisterImage(ctx, image, *isDefault, *taskRoleName, *taskExecutionRoleName, *cpu, *memory, *runtimePlatform, createdBy) error
    ListImages(ctx) ([]ImageInfo, error)
    GetImage(ctx, image) (*ImageInfo, error)
    RemoveImage(ctx, image) error

    // Logging
    FetchLogsByExecutionID(ctx, executionID) ([]LogEvent, error)
    FetchBackendLogs(ctx, requestID) ([]LogEvent, error)
}
```

## Problems Identified

### 1. **Overloaded - Mixed Concerns**

The interface violates the Single Responsibility Principle by combining:

- **Execution Management**: Task lifecycle operations (start, kill)
- **Image Registry**: Docker image registration and management
- **Log Aggregation**: Both execution logs and backend infrastructure logs

These are three distinct responsibilities that would benefit from separation.

### 2. **Incoherent Abstraction Levels**

- `StartTask` and `KillTask` operate on **execution instances**
- `RegisterImage` and `ListImages` operate on **image definitions**
- `FetchBackendLogs` fetches **Lambda infrastructure logs** (unrelated to task execution)

The interface mixes operational concerns (running tasks) with configuration concerns (managing images) and observability concerns (logs).

### 3. **ECS Implementation Leakage**

While the interface attempts to be provider-agnostic, several aspects leak AWS ECS specifics:

#### RegisterImage Parameters
```go
RegisterImage(
    ctx context.Context,
    image string,
    isDefault *bool,
    taskRoleName, taskExecutionRoleName *string,  // ← ECS-specific IAM roles
    cpu, memory *int,                              // ← ECS task sizing
    runtimePlatform *string,                       // ← ECS platform selection
    createdBy string,
)
```

These parameters are tightly coupled to ECS task definitions. Other providers (GCP Cloud Run, Kubernetes) have different configuration models.

#### Hidden Implementation Details
The AWS implementation has additional methods not in the interface:
- `GetDefaultImageFromDB(ctx)` - DynamoDB-specific
- `GetTaskDefinitionARNForImage(ctx, image)` - Returns ECS ARN
- `GetImagesByRequestID(ctx, requestID)` - DynamoDB query pattern

### 4. **Image Management Complexity**

Image management involves:
- **DynamoDB operations** (via `imageRepo`)
- **ECS task definition management** (register, deregister, tag)
- **IAM role validation**
- **Default image tracking**

All of this is embedded within the Runner implementation, making it difficult to:
- Test image operations independently
- Swap storage backends
- Support other orchestration platforms

### 5. **Log Fetching Inconsistency**

Two log methods with different scopes:
- `FetchLogsByExecutionID`: Gets logs from the **user's task execution** (CloudWatch Logs stream)
- `FetchBackendLogs`: Gets logs from **Lambda backend infrastructure** (CloudWatch Insights query)

These serve different purposes:
- One is for end-user task output
- The other is for platform debugging/observability

## Implementation Files Breakdown

**AWS Runner Implementation** spans multiple files:

| File | Responsibilities | Lines |
|------|-----------------|-------|
| `runner.go` | StartTask, KillTask, FetchLogsByExecutionID | ~705 |
| `images_dynamodb.go` | RegisterImage, ListImages, GetImage, RemoveImage, helper methods | ~754 |
| `logs.go` | FetchBackendLogs | ~324 |
| `taskdef.go` | Task definition helpers (used by image management) | ~427 |

**Total: ~2,210 lines** implementing a single interface across 4 files.

## Recommended Refactoring

### Option 1: Interface Segregation (Recommended)

Split into focused interfaces:

```go
// Core execution interface - what Runner should be
type TaskExecutor interface {
    StartTask(ctx, userEmail, *ExecutionRequest) (executionID, *time.Time, error)
    KillTask(ctx, executionID) error
}

// Separate image management
type ImageRegistry interface {
    RegisterImage(ctx, *ImageConfig) error
    ListImages(ctx) ([]ImageInfo, error)
    GetImage(ctx, imageRef) (*ImageInfo, error)
    RemoveImage(ctx, imageRef) error
    GetDefaultImage(ctx) (*ImageInfo, error)
}

// Separate log aggregation
type LogAggregator interface {
    FetchExecutionLogs(ctx, executionID) ([]LogEvent, error)
}

// Separate infrastructure observability
type BackendObservability interface {
    FetchBackendLogs(ctx, requestID) ([]LogEvent, error)
}
```

**Benefits:**
- Clear separation of concerns
- Easier to test each component independently
- Can compose services with only the interfaces they need
- Follows Interface Segregation Principle (ISP)

### Option 2: Simplified Image Configuration

Abstract away provider-specific details:

```go
type ImageConfig struct {
    Image           string
    IsDefault       *bool
    Resources       *ResourceConfig  // Abstracts CPU/memory
    RuntimeConfig   *RuntimeConfig   // Abstracts platform/arch
    Permissions     *PermissionConfig // Abstracts IAM roles
    RegisteredBy    string
}

type ResourceConfig struct {
    CPU    *int
    Memory *int
}

type RuntimeConfig struct {
    Platform     *string  // "linux/amd64", "linux/arm64"
    Architecture *string
}

type PermissionConfig struct {
    TaskRole      *string
    ExecutionRole *string
}
```

This makes provider-specific parameters explicit while keeping the interface signature simpler.

### Option 3: Extract Image Management to Separate Service

Move image operations entirely out of Runner:

```go
// runner.go - only execution
type Runner interface {
    StartTask(ctx, userEmail, *ExecutionRequest) (executionID, *time.Time, error)
    KillTask(ctx, executionID) error
    FetchExecutionLogs(ctx, executionID) ([]LogEvent, error)
}

// images/registry.go - separate service
type ImageService interface {
    RegisterImage(ctx, *ImageConfig) error
    UnregisterImage(ctx, imageID) error
    ListImages(ctx) ([]ImageInfo, error)
    GetImage(ctx, imageRef) (*ImageInfo, error)
    SetDefaultImage(ctx, imageID) error
}
```

The orchestrator `Service` would depend on both:
```go
type Service struct {
    runner       Runner
    imageService ImageService
    // ... other deps
}
```

## Migration Strategy

### Phase 1: Extract Interfaces (No Breaking Changes)
1. Define new interfaces alongside existing `Runner`
2. AWS Runner implements all interfaces
3. Update orchestrator service to use specific interfaces internally

### Phase 2: Gradual Adoption
1. New code uses specific interfaces
2. Mark `Runner` as deprecated
3. Update API handlers to depend on specific interfaces

### Phase 3: Complete Separation
1. Move image management to separate package
2. Move log aggregation to separate package
3. Remove deprecated `Runner` interface

## Conclusion

The current `Runner` interface is **overloaded** with multiple responsibilities and **incoherent** in mixing execution, configuration, and observability concerns. While it started focused on ECS-like task execution, it has accumulated image management and logging responsibilities that would be better served by separate, focused interfaces.

**Recommended Actions:**
1. ✅ Split into `TaskExecutor`, `ImageRegistry`, and `LogAggregator` interfaces
2. ✅ Extract image management logic to a separate service/package
3. ✅ Simplify `StartTask` to accept just an imageID (lookup handled by ImageRegistry)
4. ✅ Move `FetchBackendLogs` to a dedicated observability/monitoring service
5. ✅ Use composition in the orchestrator service to coordinate between components

This refactoring will:
- Improve testability (mock only what you need)
- Enable multi-provider support (swap implementations independently)
- Reduce coupling between execution and configuration
- Make the codebase easier to understand and maintain
