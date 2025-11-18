# Code Smell Analysis Report

**Date**: 2025-11-18
**Codebase**: runvoy
**Analyzer**: Claude Code

---

## Overview

**Codebase**: Go-based serverless command execution platform
**Size**: 211 Go files, 74 test files
**Architecture**: Provider-agnostic design with AWS-specific implementations

---

## Critical Issues (High Priority)

### 1. Excessive Code Duplication in HTTP Handlers (~60+ DRY violations)

The most significant issue is repetitive patterns in the handler layer.

#### Error extraction pattern (repeated 14+ times)

```go
statusCode := apperrors.GetStatusCode(err)
errorCode := apperrors.GetErrorCode(err)
errorDetails := apperrors.GetErrorDetails(err)
```

**Files affected**:
- `internal/server/handlers_executions.go`: Lines 36-38, 52-54, 68-70, 106-108, 132-134, 166-168, 226-228
- `internal/server/handlers_images.go`: Lines 37-39, 81-83, 118-120
- `internal/server/handlers_users.go`: Lines 29-31, 54-56
- `internal/server/handlers_api_keys.go`: Lines 27-29
- `internal/server/handlers_health.go`: Lines 27-29

**Recommended solution**:
```go
func extractErrorInfo(err error) (int, string, string) {
    return apperrors.GetStatusCode(err),
           apperrors.GetErrorCode(err),
           apperrors.GetErrorDetails(err)
}
```

#### Request body decoding (repeated 6+ times)

```go
if err := json.NewDecoder(req.Body).Decode(&structReq); err != nil {
    writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())
    return
}
```

**Files affected**:
- `internal/server/handlers_users.go`: Lines 16-18, 48-50
- `internal/server/handlers_images.go`: Lines 20-22
- `internal/server/handlers_executions.go`: Lines 29-31
- `internal/server/handlers_secrets.go`: Lines 17-19, 84-86

**Recommended solution**:
```go
func decodeRequestBody(w http.ResponseWriter, req *http.Request, v interface{}) error {
    if err := json.NewDecoder(req.Body).Decode(v); err != nil {
        writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())
        return err
    }
    return nil
}
```

#### User context extraction (repeated 6+ times)

```go
user, ok := r.getUserFromContext(req)
if !ok {
    writeErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "user not found in context")
    return
}
```

**Files affected**:
- `internal/server/handlers_executions.go`: Lines 22-26
- `internal/server/handlers_images.go`: Lines 25-29
- `internal/server/handlers_secrets.go`: Lines 22-26, 89-93
- `internal/server/handlers_users.go`: Lines 21-25

**Recommended solution**:
```go
func (r *Router) requireAuthenticatedUser(w http.ResponseWriter, req *http.Request) (*api.User, bool) {
    user, ok := r.getUserFromContext(req)
    if !ok {
        writeErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "user not found in context")
        return nil, false
    }
    return user, true
}
```

#### URL parameter extraction (multiple occurrences)

**ExecutionID extraction** (3 occurrences):
```go
executionID := strings.TrimSpace(chi.URLParam(req, "executionID"))
if executionID == "" {
    writeErrorResponse(w, http.StatusBadRequest, "invalid execution id", "executionID is required")
    return
}
```

**Files affected**:
- `internal/server/handlers_executions.go`: Lines 90-94, 124-128, 158-162

**Image path extraction** (2 occurrences):
- `internal/server/handlers_images.go`: Lines 63-77, 100-114

---

### 2. Complex Functions Requiring Refactoring

| Function | File | Lines | Issues |
|----------|------|-------|--------|
| `NewRouter` | `internal/server/router.go:32-105` | 73 | Mixed responsibilities, marked `//nolint:funlen` |
| `markLastRemainingImageAsDefault` | `internal/providers/aws/orchestrator/taskdef.go:285-362` | 77 | 3+ levels of nesting |
| `KillTask` | `internal/providers/aws/orchestrator/runner.go:657-735` | 78 | Duplicated AWS API patterns |
| `handleRunCommand` | `internal/server/handlers_executions.go:19-84` | 65 | Repetitive error handling |
| `parseImageReference` | `internal/providers/aws/database/dynamodb/images.go:181-224` | 43 | 4 levels of nesting |

#### Functions with too many parameters (5+)

| Function | File | Parameter Count |
|----------|------|-----------------|
| `buildTaskDefinitionInput` | `internal/providers/aws/orchestrator/runner.go:176` | 9 |
| `PutImageTaskDef` | `internal/providers/aws/orchestrator/runner.go:44` | 11 |
| `RemoveFilteredPolicies` | `internal/auth/authorization/enforcer.go:88` | 6 |

**Recommended solution**: Use struct parameters:
```go
type BuildTaskDefParams struct {
    Family          string
    Image           string
    TaskExecRoleARN string
    // ... other fields
}

func BuildTaskDefinitionInput(ctx context.Context, params *BuildTaskDefParams) ...
```

---

## Medium Priority Issues

### 3. Inconsistent Error Handling Approaches

Mixed error patterns across the codebase:

1. **Standardized** (preferred): `apperrors.ErrInternalError("message", err)`
2. **Non-standardized**: `fmt.Errorf("error: %w", err)`
3. **Raw errors**: `errors.New("message")`

**Locations using non-standardized patterns**:
- `internal/config/config.go:63` - uses `fmt.Errorf`
- `internal/auth/authorization/roles.go:47` - uses `fmt.Errorf`

**Locations using standardized patterns** (correct):
- `internal/providers/aws/database/secrets.go:70` - uses `appErrors`
- `internal/providers/aws/database/dynamodb/images.go:538` - uses `apperrors`

**Recommendation**: Standardize on `apperrors.*` constructors throughout the codebase.

---

### 4. Mixed Naming Conventions

#### JSON field naming inconsistency

**snake_case** (in `internal/api/executions.go:24-27`):
```go
ExecutionID string `json:"execution_id"`
Status      string `json:"status"`
ImageID     string `json:"image_id"`
```

**camelCase** (in `internal/api/health.go:11-17`):
```go
Timestamp      string `json:"timestamp"`
ComputeStatus  HealthReconcileComputeStatus `json:"computeStatus"`
SecretsStatus  HealthReconcileSecretsStatus `json:"secretsStatus"`
IdentityStatus HealthReconcileIdentityStatus `json:"identityStatus"`
```

**Recommendation**: Standardize on snake_case for JSON fields (more common in REST APIs).

---

### 5. Inconsistent Logging Patterns

#### Map-based logging (verbose)
```go
logger.Error("message", "context", map[string]string{
    "error": err.Error(),
    "status_code": strconv.Itoa(statusCode),
})
```

#### Key-value logging (preferred)
```go
logger.Debug("message", "user", user, "role", role)
```

**Inconsistent log levels**: Some failures logged at `Debug` level, others at `Error` level.

**Files affected**:
- `internal/server/handlers_executions.go:110` - `logger.Debug("failed to get execution logs", ...)`
- `internal/server/handlers_executions.go:40` - `logger.Error("failed to resolve image", ...)`

---

### 6. CLI Command Initialization Duplication

Repeated pattern across command files:
```go
cfg, err := getConfigFromContext(cmd)
if err != nil {
    output.Errorf("failed to load configuration: %v", err)
    return
}

c := client.New(cfg, slog.Default())
service := NewService(c, NewOutputWrapper())
if err = service.Operation(cmd.Context()); err != nil {
    output.Errorf(err.Error())
}
```

**Files affected**:
- `cmd/cli/cmd/users.go`: Lines 29-40, 50-62
- `cmd/cli/cmd/secrets.go`: Lines 47-62, 77-90+
- `cmd/cli/cmd/images.go`: Similar patterns throughout

**Recommended solution**:
```go
func executeServiceCommand(cmd *cobra.Command, operation func(context.Context, *client.Client) error) {
    cfg, err := getConfigFromContext(cmd)
    if err != nil {
        output.Errorf("failed to load configuration: %v", err)
        return
    }

    c := client.New(cfg, slog.Default())
    if err = operation(cmd.Context(), c); err != nil {
        output.Errorf(err.Error())
    }
}
```

---

## Low Priority Issues

### 7. TODO Comments to Address

| Location | Note |
|----------|------|
| `scripts/seed-admin-user/main.go:1` | Temporary script - consider removing or documenting |
| `internal/config/config.go:259` | Log level default should be INFO, not DEBUG for production |
| `internal/providers/aws/processor/ecs_events.go:53` | Orphaned task handling needs implementation |

### 8. Unused API Parameter

`internal/server/handlers.go:27` - `denialMsg` parameter marked unused but kept for API compatibility:
```go
func (r *Router) handleListWithAuth(
    w http.ResponseWriter,
    req *http.Request,
    _ string, // denialMsg - no longer used, kept for API compatibility
    serviceCall func() (any, error),
    operationName string,
)
```

---

## Recommended Refactoring Priorities

### Phase 1: Handler Layer Cleanup (High Impact)

1. Create `internal/server/helpers.go` with extracted helper functions:
   - `extractErrorInfo(err error) (int, string, string)`
   - `decodeRequestBody(w, req, v interface{}) error`
   - `requireAuthenticatedUser(w, req) (*api.User, bool)`
   - `getRequiredURLParam(w, req, name string) (string, bool)`

2. Refactor all handlers to use helpers

**Estimated reduction**: ~20-30% code in handler files

### Phase 2: Function Decomposition

1. Break down `NewRouter` into:
   - `registerUsersRoutes()`
   - `registerImagesRoutes()`
   - `registerExecutionsRoutes()`
   - `registerSecretsRoutes()`

2. Refactor `markLastRemainingImageAsDefault` into smaller, testable functions

3. Create parameterized helper for AWS API call patterns in `runner.go`

### Phase 3: Standardization

1. Standardize error handling on `apperrors.*` constructors
2. Standardize JSON naming on snake_case
3. Standardize logging on key-value pairs (not maps)
4. Align log levels (errors should use `Error`, not `Debug`)

### Phase 4: Cleanup

1. Address TODO comments or convert to tracked issues
2. Remove unused parameters with proper deprecation
3. Consolidate CLI initialization patterns

---

## Summary Statistics

| Category | Count |
|----------|-------|
| DRY Violations | 60+ |
| Functions > 50 lines | 4 |
| Functions with 5+ parameters | 5 |
| Marked with `//nolint:funlen` | 3 |
| Inconsistent patterns | 25+ |
| TODO/FIXME comments | 3 |

---

## Conclusion

**Overall Code Health**: Good structure and architecture, but the HTTP handler layer needs significant DRY cleanup. The provider-agnostic design is well-implemented, and test coverage is solid (74 test files).

The most impactful improvement would be creating helper functions for the handler layer, which would eliminate ~60+ instances of duplicated code and reduce the handler files by approximately 20-30%.
