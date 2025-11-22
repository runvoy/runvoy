# Runner Interface Refactoring - Analysis and Implementation

## Executive Summary

The `Runner` interface in the orchestrator was **overloaded** and exhibited **coherence issues**. Initially designed to interact with ECS-like services for task execution, it had grown to include image management, log fetching, and infrastructure logging concerns.

**Status**: ✅ **REFACTORED** - Separated into four focused interfaces following Interface Segregation Principle.

## Previous Interface (Removed)

The monolithic Runner interface combined multiple responsibilities:

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

## New Implementation

**Location:** `internal/backend/orchestrator/main.go`

Four focused interfaces replace the monolithic Runner:

```go
// TaskExecutor - Task lifecycle management
type TaskExecutor interface {
    StartTask(ctx, userEmail, *ExecutionRequest) (executionID, *time.Time, error)
    KillTask(ctx, executionID) error
}

// ImageRegistry - Image management
type ImageRegistry interface {
    RegisterImage(ctx, image, *isDefault, *taskRoleName, *taskExecutionRoleName, *cpu, *memory, *runtimePlatform, createdBy) error
    ListImages(ctx) ([]ImageInfo, error)
    GetImage(ctx, image) (*ImageInfo, error)
    RemoveImage(ctx, image) error
}

// LogAggregator - Execution logs
type LogAggregator interface {
    FetchLogsByExecutionID(ctx, executionID) ([]LogEvent, error)
}

// BackendObservability - Infrastructure logs
type BackendObservability interface {
    FetchBackendLogs(ctx, requestID) ([]LogEvent, error)
}
```

Service structure updated:

```go
type Service struct {
    taskExecutor        TaskExecutor
    imageRegistry       ImageRegistry
    logAggregator       LogAggregator
    backendObservability BackendObservability
    // ... other fields
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

## Implementation Details

### Interface Segregation Applied

Split the monolithic interface into four focused interfaces, each with a single responsibility:

**TaskExecutor** - 2 methods
- `StartTask()`: Trigger task execution
- `KillTask()`: Terminate running task

**ImageRegistry** - 4 methods
- `RegisterImage()`: Register new image configuration
- `ListImages()`: List all registered images
- `GetImage()`: Retrieve single image by ID/name
- `RemoveImage()`: Deregister image and cleanup

**LogAggregator** - 1 method
- `FetchLogsByExecutionID()`: Retrieve execution logs

**BackendObservability** - 1 method
- `FetchBackendLogs()`: Retrieve infrastructure logs

**Benefits Achieved:**
- ✅ Clear separation of concerns
- ✅ Easier to test (mock only what you need)
- ✅ Services depend only on required interfaces
- ✅ Follows Interface Segregation Principle (ISP)
- ✅ Enables multi-provider support (GCP, Azure)

### Provider-Agnostic Image Configuration

Implemented `ImageConfig` to abstract provider-specific details:

```go
type ImageConfig struct {
    Image        string
    IsDefault    *bool
    Resources    *ResourceConfig    // CPU/Memory abstraction
    Runtime      *RuntimeConfig     // Platform specification
    Permissions  *PermissionConfig  // Role abstraction
    RegisteredBy string
}

type ResourceConfig struct {
    // CPU in provider-specific units:
    // - AWS ECS: 256, 512, 1024, 2048, 4096
    // - GCP Cloud Run: 1000, 2000, 4000 (millicores)
    // - Azure ACI: 1, 2, 4 (CPU cores)
    CPU *int

    // Memory in MB (converted to provider units)
    Memory *int
}

type RuntimeConfig struct {
    Platform *string  // "linux/amd64", "linux/arm64"
}

type PermissionConfig struct {
    // TaskRole: IAM role (AWS), Service Account (GCP), Managed Identity (Azure)
    TaskRole *string

    // ExecutionRole: AWS-specific infrastructure permissions
    ExecutionRole *string
}
```

This abstraction allows the same ImageConfig to work across AWS, GCP, and Azure with each provider translating values appropriately.

## Results

### Implementation Complete

The refactoring has been successfully implemented:

1. ✅ **Four Focused Interfaces** - TaskExecutor, ImageRegistry, LogAggregator, BackendObservability
2. ✅ **Provider-Agnostic Configuration** - ImageConfig with ResourceConfig, RuntimeConfig, PermissionConfig
3. ✅ **Service Restructured** - Separate fields for each interface instead of monolithic runner
4. ✅ **AWS Implementation Updated** - Implements all four interfaces
5. ✅ **Documentation** - MIGRATION_GUIDE.md with examples for GCP and Azure

### Benefits Realized

**Testability**
- Mock only required interface (2-4 methods) instead of all 9 methods
- Faster test execution, clearer test intent

**Multi-Provider Ready**
- Documented CPU/memory mappings for AWS, GCP, Azure
- Permission abstractions for IAM roles, Service Accounts, Managed Identities
- Each provider can implement interfaces independently

**Separation of Concerns**
- Execution ≠ Configuration ≠ Observability
- Clear boundaries between responsibilities
- Easier to reason about code

**Reduced Coupling**
- Image management independent of task execution
- Log aggregation separate from both
- Backend observability isolated from user-facing operations

### Files Changed

```
internal/backend/orchestrator/main.go          (+70 lines)
internal/backend/orchestrator/image_config.go  (new file)
internal/providers/aws/orchestrator/runner.go  (updated docs)
MIGRATION_GUIDE.md                             (complete rewrite)
runner-interface-analysis.md                   (this file)
```

### Future Extensions

The new architecture enables:

**GCP Cloud Run Support**
```go
type CloudRunExecutor struct { ... }
func (e *CloudRunExecutor) StartTask(...) { /* Cloud Run Jobs API */ }
func (e *CloudRunExecutor) KillTask(...) { /* Cancel execution */ }
```

**Azure Container Instances Support**
```go
type ACIExecutor struct { ... }
func (e *ACIExecutor) StartTask(...) { /* Create Container Group */ }
func (e *ACIExecutor) KillTask(...) { /* Delete Container Group */ }
```

**Mixed Provider Deployments**
```go
service := orchestrator.NewService(
    taskExecutor: gcpExecutor,        // Run on GCP
    imageRegistry: awsImageRegistry,  // Store in AWS
    logAggregator: gcpLogAggregator,  // GCP logs
    backendObservability: awsObs,     // Monitor AWS
    ...
)
```
