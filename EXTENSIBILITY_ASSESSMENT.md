# Cloud Provider Extensibility Assessment

**Project:** Runvoy
**Date:** 2025-11-11
**Assessment Focus:** Multi-cloud provider support without changing core structure

---

## Executive Summary

**Overall Grade: B+ (Good Foundation, Minor Refactoring Needed)**

Runvoy demonstrates a **strong architectural foundation** for multi-cloud extensibility. The core abstractions (Runner, Repository patterns, Events Backend) are well-designed and provider-agnostic. However, there are **3-4 key areas** where AWS-specific coupling exists that would require refactoring before cleanly adding a second cloud provider (GCP, Azure, etc.).

**Estimated Effort:**
- **Adding GCP support:** 3-4 weeks (with recommended refactoring)
- **Per additional provider:** 2-3 weeks (after patterns established)

---

## Architecture Scorecard

| Component | Abstraction Quality | Multi-Cloud Ready | Priority |
|-----------|-------------------|-------------------|----------|
| **Execution (Runner)** | ⭐⭐⭐⭐⭐ Excellent | ✅ Ready | Low |
| **Database Repositories** | ⭐⭐⭐⭐⭐ Excellent | ✅ Ready | Low |
| **Configuration** | ⭐⭐⭐⭐ Good | ✅ Ready | Low |
| **WebSocket Manager** | ⭐⭐⭐⭐ Good | ✅ Ready | Low |
| **Event Processing** | ⭐⭐⭐ Adequate | ⚠️ Needs Work | **High** |
| **Logger Integration** | ⭐⭐ Poor | ❌ Needs Refactor | **High** |
| **Constants Organization** | ⭐⭐ Poor | ❌ Needs Refactor | **Medium** |

---

## Strengths (What Works Well)

### 1. Excellent Core Abstractions ✅

#### Runner Interface
**Location:** `internal/app/main.go:22-44`

The `Runner` interface is **completely provider-agnostic**:

```go
type Runner interface {
    StartTask(ctx context.Context, userEmail string, req *api.ExecutionRequest)
        (executionID string, createdAt *time.Time, err error)
    KillTask(ctx context.Context, executionID string) error
    RegisterImage(ctx context.Context, image string, isDefault *bool) error
    ListImages(ctx context.Context) ([]api.ImageInfo, error)
    RemoveImage(ctx context.Context, image string) error
    FetchLogsByExecutionID(ctx context.Context, executionID string) ([]api.LogEvent, error)
}
```

**Why this is excellent:**
- No AWS types in signatures
- Generic operation names (StartTask vs RunECSTask)
- Returns platform-agnostic types (executionID string, not TaskARN)
- Clean separation of concerns

**AWS Implementation:** `internal/providers/aws/app/runner.go`
**Future:** Add `internal/providers/gcp/app/runner.go`, etc.

#### Database Repository Interfaces
**Location:** `internal/database/repository.go`

All 4 repository interfaces are completely abstract:
- `UserRepository` - User management operations
- `ExecutionRepository` - Execution tracking
- `ConnectionRepository` - WebSocket connections
- `TokenRepository` - Authentication tokens

**Current implementations:**
- `internal/providers/aws/database/dynamodb/` - AWS DynamoDB
- **Future:** `internal/providers/gcp/database/firestore/` - GCP Firestore

### 2. Clean Directory Structure ✅

```
internal/
├── app/                  # ✅ Provider-agnostic business logic
├── api/                  # ✅ Provider-agnostic API types
├── database/             # ✅ Abstract repository interfaces
└── providers/
    └── aws/              # ✅ ALL AWS code isolated here
        ├── app/          # ECS Runner implementation
        ├── database/     # DynamoDB repositories
        ├── events/       # Lambda event handling
        └── websocket/    # API Gateway WebSocket

cmd/backend/
└── providers/
    └── aws/              # ✅ AWS-specific entry points
        ├── orchestrator/
        └── event_processor/
```

**Future structure:**
```
internal/providers/
├── aws/
├── gcp/                  # Mirror AWS structure
│   ├── app/
│   ├── database/
│   ├── events/
│   └── websocket/
└── azure/
```

### 3. Recent Configuration Refactoring ✅

**Location:** `internal/config/config.go`

The recent refactoring (commit: `c0e8ad1`) established a good pattern:

```go
type Config struct {
    // Generic fields
    BackendProvider constants.BackendProvider
    LogLevel        string
    Port            int

    // Provider-specific configs (nested)
    AWS *awsconfig.Config `mapstructure:"aws" yaml:"aws,omitempty"`
    // Future: GCP *gcpconfig.Config `mapstructure:"gcp" yaml:"gcp,omitempty"`
}
```

**Why this works:**
- Provider configs are optional (omitempty)
- Validation is provider-specific (`ValidateOrchestrator`, `ValidateEventProcessor`)
- Environment variable binding is modular (`awsconfig.BindEnvVars`)

---

## Weaknesses (Areas Needing Improvement)

### ❌ 1. Logger Package - AWS Lambda Coupling (HIGH PRIORITY)

**Location:** `internal/logger/context.go:49-54`

**Problem:**
```go
import (
    "github.com/aws/aws-lambda-go/lambdacontext"  // ❌ Direct AWS dependency
)

func DeriveRequestLogger(ctx context.Context, base *slog.Logger) *slog.Logger {
    // ...

    // ❌ Hard-coded AWS Lambda context extraction
    if lc, ok := lambdacontext.FromContext(ctx); ok {
        if lc.AwsRequestID != "" {
            return base.With("requestID", lc.AwsRequestID)
        }
    }

    return base
}
```

**Impact:**
- Every log call uses this function (`DeriveRequestLogger` appears 50+ times in codebase)
- Forces AWS Lambda SDK dependency even for non-AWS deployments
- GCP Cloud Functions / Azure Functions have different context extraction

**Recommended Solution:**

Create a context extractor interface:

```go
// internal/logger/context.go

// ContextExtractor extracts request metadata from provider-specific contexts
type ContextExtractor interface {
    ExtractRequestID(ctx context.Context) string
}

var extractors []ContextExtractor

// RegisterExtractor adds a provider-specific context extractor
func RegisterExtractor(e ContextExtractor) {
    extractors = append(extractors, e)
}

func DeriveRequestLogger(ctx context.Context, base *slog.Logger) *slog.Logger {
    if base == nil {
        return slog.Default()
    }

    // Try generic context value first
    if requestID := GetRequestID(ctx); requestID != "" {
        return base.With("requestID", requestID)
    }

    // Try registered extractors
    for _, extractor := range extractors {
        if requestID := extractor.ExtractRequestID(ctx); requestID != "" {
            return base.With("requestID", requestID)
        }
    }

    return base
}
```

**AWS implementation:**
```go
// internal/providers/aws/logger/extractor.go

type AWSLambdaExtractor struct{}

func (e *AWSLambdaExtractor) ExtractRequestID(ctx context.Context) string {
    if lc, ok := lambdacontext.FromContext(ctx); ok {
        return lc.AwsRequestID
    }
    return ""
}

func init() {
    logger.RegisterExtractor(&AWSLambdaExtractor{})
}
```

**GCP implementation (future):**
```go
// internal/providers/gcp/logger/extractor.go

type GCPCloudFunctionExtractor struct{}

func (e *GCPCloudFunctionExtractor) ExtractRequestID(ctx context.Context) string {
    // Extract from GCP metadata server or function context
    return ""
}
```

**Files to modify:**
- `internal/logger/context.go` - Add extractor pattern
- `internal/providers/aws/logger/extractor.go` - New file
- `cmd/backend/providers/aws/orchestrator/main.go` - Import AWS extractor

---

### ⚠️ 2. Events Backend Interface - AWS Response Type (HIGH PRIORITY)

**Location:** `internal/events/backend.go:28`

**Problem:**
```go
type Backend interface {
    HandleCloudEvent(...) (bool, error)
    HandleLogsEvent(...) (bool, error)

    // ❌ Returns AWS-specific type
    HandleWebSocketEvent(ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger)
        (events.APIGatewayProxyResponse, bool)
}
```

**Impact:**
- Interface is contaminated with AWS type `events.APIGatewayProxyResponse`
- GCP/Azure would need to construct AWS types or change interface
- Breaks abstraction principle

**Recommended Solution:**

Create a generic response type:

```go
// internal/events/backend.go

type WebSocketResponse struct {
    StatusCode int
    Headers    map[string]string
    Body       string
}

type Backend interface {
    HandleCloudEvent(ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger) (bool, error)
    HandleLogsEvent(ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger) (bool, error)

    // ✅ Returns generic type
    HandleWebSocketEvent(ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger)
        (WebSocketResponse, bool)
}
```

**AWS implementation adapter:**
```go
// internal/providers/aws/events/backend.go

func (b *Backend) HandleWebSocketEvent(...) (events.WebSocketResponse, bool) {
    // ... AWS-specific logic ...

    return events.WebSocketResponse{
        StatusCode: 200,
        Body:       "OK",
    }, true
}

// Convert to AWS type in Lambda handler:
func convertToAWSResponse(resp events.WebSocketResponse) awsevents.APIGatewayProxyResponse {
    return awsevents.APIGatewayProxyResponse{
        StatusCode: resp.StatusCode,
        Headers:    resp.Headers,
        Body:       resp.Body,
    }
}
```

**Files to modify:**
- `internal/events/backend.go` - Add `WebSocketResponse` type, change interface
- `internal/providers/aws/events/backend.go` - Update return type
- `cmd/backend/providers/aws/event_processor/main.go` - Add conversion

---

### ⚠️ 3. Constants Package - Mixed Concerns (MEDIUM PRIORITY)

**Location:** `internal/constants/constants.go`

**Problem:**

AWS-specific constants are mixed with generic ones:

```go
// ❌ AWS-specific (should move to internal/providers/aws/constants/)
const RunnerContainerName = "runner"          // Line 81 - ECS specific
const SidecarContainerName = "sidecar"        // Line 86 - ECS specific
const CloudWatchLogsDescribeLimit = int32(50) // Line 238 - CloudWatch specific
const CloudWatchLogsEventsLimit = int32(10000)// Line 241 - CloudWatch specific

type EcsStatus string                         // Lines 97-118 - ECS specific
const (
    EcsStatusProvisioning EcsStatus = "PROVISIONING"
    EcsStatusPending      EcsStatus = "PENDING"
    // ... etc
)

// ✅ Generic (can stay)
type ExecutionStatus string
const (
    ExecutionRunning   ExecutionStatus = "RUNNING"
    ExecutionSucceeded ExecutionStatus = "SUCCEEDED"
    // ...
)
```

**Impact:**
- Creates false impression of AWS-only system
- Forces other providers to reference AWS constants
- Pollutes import graph

**Recommended Solution:**

**Split into:**

```
internal/
├── constants/
│   └── constants.go              # ✅ Generic only
└── providers/
    ├── aws/
    │   └── constants/
    │       └── constants.go      # AWS-specific
    └── gcp/
        └── constants/
            └── constants.go      # GCP-specific
```

**Generic constants (keep in internal/constants/):**
```go
// internal/constants/constants.go

type ExecutionStatus string
const (
    ExecutionRunning   ExecutionStatus = "RUNNING"
    ExecutionSucceeded ExecutionStatus = "SUCCEEDED"
    ExecutionFailed    ExecutionStatus = "FAILED"
    ExecutionStopped   ExecutionStatus = "STOPPED"
)

const ProjectName = "runvoy"
const APIKeyHeader = "X-API-Key"
// ... etc
```

**AWS-specific constants (move to internal/providers/aws/constants/):**
```go
// internal/providers/aws/constants/ecs.go

const RunnerContainerName = "runner"
const SidecarContainerName = "sidecar"
const SharedVolumeName = "workspace"
const SharedVolumePath = "/workspace"

type EcsStatus string
const (
    EcsStatusProvisioning EcsStatus = "PROVISIONING"
    EcsStatusPending      EcsStatus = "PENDING"
    // ...
)
```

```go
// internal/providers/aws/constants/cloudwatch.go

const CloudWatchLogsDescribeLimit = int32(50)
const CloudWatchLogsEventsLimit = int32(10000)
const LogStreamPrefix = "task"
```

**Files to modify:**
- `internal/constants/constants.go` - Remove AWS-specific constants
- `internal/providers/aws/constants/ecs.go` - New file
- `internal/providers/aws/constants/cloudwatch.go` - New file
- `internal/providers/aws/app/runner.go` - Update imports
- ~10-15 other files importing constants

---

### ℹ️ 4. Minor Issues (LOW PRIORITY)

#### a) WebSocket URL Generation

**Location:** `internal/providers/aws/websocket/manager.go`

The WebSocket manager currently assumes API Gateway URL format. This is acceptable since the interface is clean, but document the expected format.

**Recommendation:** Add documentation comments about URL format expectations.

---

## Implementation Roadmap

### Phase 1: Core Refactoring (1-2 weeks)

**Goal:** Remove AWS coupling from core packages

1. **Logger Context Extractor** (2-3 days)
   - Add `ContextExtractor` interface to `internal/logger/`
   - Move AWS Lambda logic to `internal/providers/aws/logger/`
   - Update `cmd/backend/providers/aws/*/main.go` to register extractor
   - Add tests

2. **Events Backend Response Type** (1-2 days)
   - Add `WebSocketResponse` to `internal/events/`
   - Update `events.Backend` interface
   - Update AWS implementation
   - Add conversion in Lambda handler

3. **Constants Reorganization** (3-4 days)
   - Create `internal/providers/aws/constants/`
   - Move AWS-specific constants
   - Update imports across codebase (use search/replace)
   - Verify tests pass

### Phase 2: GCP Implementation (2-3 weeks)

**Goal:** Prove extensibility by adding GCP support

1. **GCP App Runner** (1 week)
   - Implement `Runner` interface using Cloud Run Jobs
   - Handle container execution
   - Map GCP job states to `ExecutionStatus`

2. **GCP Database Repositories** (1 week)
   - Implement 4 repositories using Firestore
   - Handle TTL/expiration
   - Add integration tests

3. **GCP Events Backend** (3-4 days)
   - Handle Pub/Sub events
   - Handle Cloud Logging
   - Implement WebSocket via Cloud Run

4. **GCP Entry Points** (2-3 days)
   - Add `cmd/backend/providers/gcp/orchestrator/`
   - Add `cmd/backend/providers/gcp/event_processor/`
   - Deploy to GCP

### Phase 3: Documentation & Testing (1 week)

1. **Multi-Provider Guide**
   - Document provider interface contracts
   - Create "Adding a New Provider" guide
   - Document configuration patterns

2. **Integration Tests**
   - Test provider switching
   - Test mixed deployments (if applicable)

---

## Specific File Changes Needed

### High Priority Changes

| File | Change Type | Description |
|------|-------------|-------------|
| `internal/logger/context.go` | Refactor | Add extractor interface |
| `internal/providers/aws/logger/extractor.go` | New | AWS Lambda extractor |
| `internal/events/backend.go` | Modify | Add generic response type |
| `internal/providers/aws/events/backend.go` | Modify | Update return type |
| `cmd/backend/providers/aws/event_processor/main.go` | Modify | Add response conversion |

### Medium Priority Changes

| File | Change Type | Description |
|------|-------------|-------------|
| `internal/constants/constants.go` | Refactor | Remove AWS-specific constants |
| `internal/providers/aws/constants/ecs.go` | New | ECS-specific constants |
| `internal/providers/aws/constants/cloudwatch.go` | New | CloudWatch constants |
| `internal/providers/aws/app/runner.go` | Modify | Update constant imports |
| ~15 other files | Modify | Update constant imports |

---

## Risk Assessment

### Low Risk ✅
- **Database repositories:** Already well-abstracted
- **Runner interface:** Clean, ready for extension
- **Configuration:** Recently refactored, good foundation

### Medium Risk ⚠️
- **Constants refactoring:** Touches many files, but changes are mechanical
- **Event processor:** Some AWS assumptions in Lambda handler flow

### High Risk ❌
- **Logger refactoring:** Used throughout codebase (50+ call sites)
  - **Mitigation:** Use extractor pattern to maintain backward compatibility
  - **Testing:** Comprehensive unit tests + integration tests

---

## Best Practices Observed

1. **Interface-First Design** ✅
   - All major components have clean interfaces
   - Implementations are isolated

2. **Dependency Injection** ✅
   - Services receive dependencies via constructors
   - Easy to swap implementations

3. **Clear Package Boundaries** ✅
   - Business logic separated from infrastructure
   - Provider code isolated

4. **Configuration Management** ✅
   - Provider-specific configs nested appropriately
   - Environment variable binding is modular

---

## Recommendations Summary

### Immediate Actions (Before Adding Next Provider)

1. ✅ **Refactor logger context extraction** (2-3 days)
   - Highest ROI for extensibility
   - Removes forced AWS dependency

2. ✅ **Fix events.Backend return type** (1-2 days)
   - Restores interface purity
   - Simple change with big impact

3. ✅ **Reorganize constants** (3-4 days)
   - Clarifies architecture
   - Makes AWS just one of many providers

### Long-Term Improvements

1. **Add provider selection tests**
   - Verify clean switching between providers
   - Catch accidental coupling early

2. **Document provider interface contracts**
   - Help future contributors understand expectations
   - Specify behavior for edge cases

3. **Consider provider registration pattern**
   - Dynamic provider discovery
   - Plugin-like architecture for future

---

## Conclusion

Runvoy has a **strong architectural foundation** for multi-cloud extensibility. The core abstractions (Runner, Repositories, Events) are well-designed and the recent configuration refactoring shows good architectural thinking.

**The main gaps are:**
1. **Logger AWS coupling** (high impact, medium effort)
2. **Events return type** (medium impact, low effort)
3. **Constants organization** (low impact, medium effort)

**Estimated total effort to achieve clean multi-cloud support:**
- Refactoring: 1-2 weeks
- First new provider (GCP): 2-3 weeks
- **Total: 3-5 weeks**

After these changes, adding additional providers should follow a predictable pattern requiring only 2-3 weeks per provider.

---

**Assessment Completed:** 2025-11-11
**Assessed By:** Claude (Runvoy Extensibility Analysis)
