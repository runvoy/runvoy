# Security Vulnerability Analysis Report

## Executive Summary

This document presents a comprehensive security analysis of the Runvoy codebase. Runvoy is a serverless command execution platform that allows teams to run arbitrary commands on remote ephemeral ECS containers. The application uses AWS services (Lambda, ECS, DynamoDB) with a Go backend and SvelteKit webapp.

**Analysis Date:** 2025-11-18
**Analyzed By:** Automated Security Scan

---

## HIGH Severity Vulnerabilities

### 1. IDOR (Insecure Direct Object Reference) - Authorization Bypass

**Location:** `internal/auth/authorization/casbin/policy.csv:21-24, 31-32`

**Issue:** Role-based policies allow users to access ANY execution record, bypassing ownership controls.

```csv
p, role:developer, /api/v1/executions/*, read, allow
p, role:developer, /api/v1/executions/*, delete, allow
p, role:viewer, /api/v1/executions/*, read, allow
```

While ownership policies exist (`p, owner, /api/v1/executions/:id, *, allow`), the role-based policies grant broader access that overrides ownership. A developer can:
- Read any user's execution logs
- Kill any user's running execution
- View execution status of any execution

**Impact:** Information disclosure, unauthorized termination of other users' tasks

**Recommendation:** Restrict role-based policies to only owned resources:
```csv
p, role:developer, /api/v1/executions, read, allow  # List only
# Remove: p, role:developer, /api/v1/executions/*, read, allow
```

---

### 2. Permissive CORS Configuration

**Location:** `internal/server/middleware.go:73-94`

**Issue:** The CORS middleware reflects any Origin header without validation:

```go
func corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
        origin := req.Header.Get("Origin")
        if origin != "" {
            w.Header().Set("Access-Control-Allow-Origin", origin)  // Reflects any origin!
        } else {
            w.Header().Set("Access-Control-Allow-Origin", "*")  // Falls back to wildcard
        }
```

**Impact:** Any website can make authenticated API requests if the user has their API key stored. This enables CSRF-like attacks.

**Recommendation:** Implement an allowlist of trusted origins:
```go
allowedOrigins := []string{"https://app.runvoy.com", "http://localhost:5173"}
if slices.Contains(allowedOrigins, origin) {
    w.Header().Set("Access-Control-Allow-Origin", origin)
}
```

---

### 3. Sensitive Data Logging

**Location:** `internal/providers/aws/database/dynamodb/users.go:169-176`

**Issue:** API key hashes are logged in debug statements:

```go
logArgs := []any{
    "operation", "DynamoDB.GetItem",
    "table", r.tableName,
    "api_key_hash", apiKeyHash,  // Logging sensitive hash!
}
```

**Impact:** If logs are compromised or improperly secured, attackers could use hashes for:
- Timing attacks
- Correlation attacks between systems
- Rainbow table attacks

**Recommendation:** Remove API key hashes from logs or mask them:
```go
"api_key_hash", apiKeyHash[:8]+"...",  // Log only first 8 chars
```

---

## MEDIUM Severity Vulnerabilities

### 4. No Rate Limiting

**Location:** `internal/server/router.go` (entire routing configuration)

**Issue:** No rate limiting middleware is implemented on any endpoints.

**Impact:**
- Brute force attacks on API keys
- Denial of service through resource exhaustion
- Abuse of claim token endpoints

**Recommendation:** Add rate limiting middleware using a library like `golang.org/x/time/rate`:
```go
r.Use(rateLimitMiddleware(10, time.Minute))  // 10 requests per minute per IP
```

---

### 5. Plaintext API Key in Pending Claims

**Location:** `internal/providers/aws/database/dynamodb/users.go:399-410`

**Issue:** The pending API key table stores the raw API key:

```go
type pendingAPIKeyItem struct {
    SecretToken  string `dynamodbav:"secret_token"`
    APIKey       string `dynamodbav:"api_key"`  // Raw API key stored!
```

While temporary (15-min TTL), this creates a window where plaintext API keys exist in the database.

**Impact:** Database compromise during claim window exposes usable API keys

**Recommendation:** Consider a different claim flow where the API key is only revealed once to the user, or use encryption at rest with a separate key.

---

### 6. Potential Timing Attack in Authentication

**Location:** `internal/backend/orchestrator/users.go:189-211`

**Issue:** API key comparison is done through hash lookup which may not be constant-time:

```go
func (s *Service) AuthenticateUser(ctx context.Context, apiKey string) (*api.User, error) {
    apiKeyHash := auth.HashAPIKey(apiKey)
    user, err := s.userRepo.GetUserByAPIKeyHash(ctx, apiKeyHash)
```

The database query timing varies based on whether the hash exists.

**Impact:** Attackers could potentially distinguish between valid and invalid API key prefixes through timing analysis.

**Recommendation:** Use constant-time comparison after retrieval or add random delay to normalize response times.

---

### 7. WebSocket Token Not Bound to Execution Ownership

**Location:** `internal/providers/aws/websocket/manager.go:616-660`

**Issue:** WebSocket tokens are generated for any authenticated user requesting logs for any execution ID:

```go
func (m *Manager) GenerateWebSocketURL(
    ctx context.Context,
    executionID string,  // Not validated against user ownership
    userEmail *string,
```

Combined with the IDOR vulnerability, this allows users to stream logs from any execution.

**Impact:** Real-time information disclosure

**Recommendation:** Validate execution ownership before generating WebSocket tokens.

---

## LOW Severity Vulnerabilities

### 8. Limited Input Validation on Playbook Names

**Location:** `internal/client/playbooks/loader.go:80-117`

**Issue:** While there's a `//nolint:gosec` comment acknowledging the path construction, playbook names are directly used in filepath:

```go
for _, ext := range constants.PlaybookFileExtensions {
    candidatePath := filepath.Join(playbookDir, name+ext)
```

The `filepath.Join` does sanitize some path traversal, but edge cases may exist.

**Impact:** Potential path traversal in CLI client (local context only)

**Recommendation:** Add explicit validation for playbook names:
```go
if strings.Contains(name, "..") || strings.Contains(name, "/") {
    return nil, fmt.Errorf("invalid playbook name")
}
```

---

### 9. Error Message Information Disclosure

**Location:** Multiple handler files

**Issue:** Some error responses include detailed internal error messages:

```go
writeErrorResponseWithCode(w, statusCode, errorCode, "failed to run command", errorDetails)
```

**Impact:** May reveal internal architecture, table names, or system state to attackers.

**Recommendation:** Use generic error messages for clients; log detailed errors server-side.

---

## Security Strengths Noted

1. **Proper XSS Protection**: The ANSI parser correctly escapes HTML before rendering (`cmd/webapp/src/lib/ansi.js:11`)

2. **Secure Token Generation**: Uses `crypto/rand` for token generation (`internal/auth/auth.go:25-31`)

3. **SHA-256 Hashing**: API keys are hashed with SHA-256 before storage

4. **RBAC Implementation**: Casbin-based authorization is well-structured

5. **Secrets Encryption**: Secrets are stored in AWS Secrets Manager with KMS encryption

6. **No SQL/Command Injection**: No raw SQL queries or shell execution with user input in main application

7. **Request Timeout**: Configurable request timeouts prevent hanging requests

---

## Recommendations Summary

| Priority | Issue | Effort |
|----------|-------|--------|
| HIGH | Fix IDOR in Casbin policies | Low |
| HIGH | Implement CORS allowlist | Low |
| HIGH | Remove sensitive data from logs | Low |
| MEDIUM | Add rate limiting | Medium |
| MEDIUM | Encrypt pending API keys | Medium |
| MEDIUM | Bind WebSocket tokens to ownership | Low |
| LOW | Validate playbook names | Low |
| LOW | Sanitize error messages | Medium |

---

## Next Steps

1. **Immediate**: Fix the IDOR vulnerability in Casbin policies - this is the most critical issue allowing unauthorized access
2. **Short-term**: Implement CORS allowlist and rate limiting
3. **Medium-term**: Review all logging to remove sensitive data
4. **Consider**: Security audit of AWS IAM roles and ECS task permissions
