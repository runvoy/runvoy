# runvoy Architecture

## Overview

runvoy is a centralized execution platform that allows teams to run infrastructure commands without sharing AWS credentials. An AWS admin deploys runvoy once to the company's AWS account, then issues API keys to team members who can execute commands safely with full audit trails.

## Design Principles

1. **Centralized Execution, Distributed Access**: One deployment per company, multiple users with API keys
2. **No Credential Sharing**: Team members never see AWS credentials
3. **Complete Audit Trail**: Every execution logged with user identification
4. **Safe Stateful Operations**: Automatic locking prevents concurrent operations on shared resources
5. **Self-Service**: Team members don't wait for admins to run commands
6. **Extensible Authorization**: Architecture supports fine-grained permissions (to be added later)

## Execution Provider Abstraction

To support multiple cloud platforms, the service layer now depends on an execution provider interface:

```text
internal/app.Service → uses Executor interface (provider-agnostic)
internal/app/aws     → AWS-specific Executor implementation (ECS Fargate)
```

- The `Executor` interface abstracts starting a command execution and returns both a stable execution ID and provider task ARN.
- The AWS implementation resides in `internal/app/aws` and encapsulates all ECS- and AWS-specific logic and types.
- `internal/app/init.go` wires the chosen provider by constructing the appropriate `Executor` and passing it into `Service`.

This change removes direct AWS SDK coupling from `internal/app` and makes adding providers (e.g., GCP) straightforward.

## Router Architecture

The application uses **chi** (github.com/go-chi/chi/v5) as the HTTP router for both Lambda and local HTTP server implementations. This provides a consistent routing API across deployment models.

### Components

- **`internal/server/router.go`**: Shared chi-based router configuration with all API routes
- **`internal/server/middleware.go`**: Middleware for request ID extraction and logging context
- **`internal/lambdaapi/adapter.go`**: Adapter to convert between Lambda events and standard http.Handler
- **`internal/lambdaapi/handler.go`**: Lambda handler that uses the chi router via adapter
- **`cmd/local/main.go`**: Local HTTP server implementation using the same router
- **`cmd/backend/aws/main.go`**: Lambda entry point that uses the chi-based handler

### Route Structure

All routes are defined in `internal/server/router.go`:

```text
GET  /api/v1/health      - Health check endpoint
POST /api/v1/users/create - Create a new user with an API key
POST /api/v1/users/revoke - Revoke a user's API key
```

Both Lambda and local HTTP server use identical routing logic, ensuring development/production parity.

### Lambda Event Adapter

The Lambda adapter (`internal/lambdaapi/adapter.go`) converts AWS Lambda Function URL events into standard `http.Request` objects that work with the chi router. This enables the same router and middleware to work in both local and AWS Lambda environments.

**Key Adaptations:**

1. **Request ID Extraction**: The Lambda request ID from `req.RequestContext.RequestID` is extracted and stored in the Lambda context using `lambdacontext.NewContext()`. This allows the request ID middleware to access it later.

2. **Remote Address**: The client source IP is extracted from `req.RequestContext.HTTP.SourceIP` and set as `httpReq.RemoteAddr`, ensuring the logging middleware can access the client's IP address in Lambda executions just like in local HTTP servers.

3. **Event to HTTP Request Conversion**:
   - URL construction from domain name, path, and query parameters
   - Base64 body decoding when needed
   - Header and query parameter copying
   - HTTP method mapping

The adapter ensures that all middleware (logging, request ID extraction, authentication) work identically in both environments without any conditional logic in the router or middleware code.

### User Management API

The system provides endpoints for creating and managing users:

#### Create User (`POST /api/v1/users/create`)

Creates a new user with an API key. The API key is only returned in the response and should be stored securely by the client.

**Request:**
```json
{
  "email": "user@example.com",
  "api_key": "optional_custom_key"  // Optional, generated if omitted
}
```

**Response (201 Created):**
```json
{
  "user": {
    "email": "user@example.com",
    "created_at": "2024-01-01T00:00:00Z",
    "revoked": false
  },
  "api_key": "abc123..."  // Only returned once!
}
```

**Error Responses:**
- 400 Bad Request: Invalid email format or missing email
- 409 Conflict: User already exists
- 503 Service Unavailable: Database errors (transient failures)

Implementation:
- Email validation using Go's `mail.ParseAddress`
- API key generation using crypto/rand if not provided
- API keys are hashed with SHA-256 before storage
- Database enforces uniqueness via ConditionalExpression
- Database errors return 503 (Service Unavailable) rather than 500, indicating transient failures

#### Revoke User (`POST /api/v1/users/revoke`)

Revokes a user's API key, preventing further authentication. The user record is preserved for audit trails.

**Request:**
```json
{
  "email": "user@example.com"
}
```

**Response (200 OK):**
```json
{
  "message": "User API key revoked successfully",
  "email": "user@example.com"
}
```

**Error Responses:**
- 400 Bad Request: Missing email
- 404 Not Found: User not found
- 503 Service Unavailable: Database errors (transient failures)

Implementation:
- Checks for user existence before revocation
- Updates the `revoked` field in DynamoDB
- Revoked users cannot authenticate (checked in `AuthenticateUser`)
- Database errors return 503 (Service Unavailable) rather than 500, indicating transient failures

### Middleware Stack

The router uses a middleware stack for cross-cutting concerns:

1. **Content-Type Middleware**: Sets `Content-Type: application/json` for all responses
2. **Request ID Middleware**: Extracts AWS Lambda request ID and adds it to logging context
3. **Authentication Middleware**: Validates API keys and adds user context
4. **Request Logging Middleware**: Logs incoming requests and their responses with method, path, status code, and duration

**Authentication Middleware Error Handling:**
- Invalid API key → 401 Unauthorized (INVALID_API_KEY)
- Revoked API key → 401 Unauthorized (API_KEY_REVOKED)
- Database failures during authentication → 503 Service Unavailable (DATABASE_ERROR)
- This ensures database errors are properly distinguished from authentication failures

The request ID middleware automatically:
- Extracts the AWS Lambda request ID from the Lambda context when available
- Adds the request ID to the request context for use by handlers
- Falls back gracefully when not running in Lambda environment

### Execution Records: Task ARN and Request ID

- The service includes the request ID (when available) in execution records created in `internal/app.Service.RunCommand()`.
- The `request_id` is persisted in DynamoDB via the `internal/database/dynamodb` repository.
- If a request ID is not present (e.g., non-Lambda environments), the service logs a warning and stores the execution without a `request_id`.
- The service now also persists the ECS `task_arn` at creation time; the executor returns both `executionID` and `taskARN` which are stored immediately.

## Logging Architecture

The application uses a unified logging approach with structured logging via `log/slog`:

### Logger Initialization
- Logger is initialized once at application startup in `internal/logger/logger.go`
- Configuration supports both development (human-readable) and production (JSON) formats
- Log level is configurable via `RUNVOY_LOG_LEVEL` environment variable

### Service-Level Logging
- Each `Service` instance contains its own logger instance (`Service.Logger`)
- This eliminates the need for context-based logger extraction
- All service methods can directly access `s.Logger` for consistent logging

### Request-Scoped Logging
- Router handlers create request-scoped loggers by combining the service logger with request ID
- Pattern: `logger := r.svc.Logger.With("requestID", requestID)`
- This ensures all log messages within a request include the Lambda request ID for tracing

### Request Logging Middleware
- **Automatic Request Logging**: The request logging middleware automatically logs all incoming requests
- Logs include: HTTP method, path, remote address, status code, and request duration
- Both incoming requests and completed responses are logged for complete request lifecycle visibility
- Implementation: `internal/server/router.go` lines 115-153
- The middleware uses a response writer wrapper to capture response status codes and measure execution time
- Remote address is automatically available in both local and Lambda executions via the Lambda adapter

### Database Layer Logging
- Database repositories receive the service logger during initialization
- This maintains consistent logging across all application layers
- Database operations are logged with the same structured format

### Benefits
- **Consistency**: All logging uses the same logger instance and format
- **Simplicity**: No need for `GetLoggerFromContext()` or global logger access
- **Traceability**: Request ID is automatically included in all request-scoped logs
- **Maintainability**: Clear separation between service-level and request-scoped logging

## System Architecture

```text
┌─────────────────────────────────────────────────────────────────┐
│                         AWS Account                              │
│                                                                  │
│  ┌──────────────┐                                               │
│  │ Lambda       │◄─────── HTTPS Function URL with X-API-Key    │
│  │ Function URL │     header                                  │
│  └──────┬───────┘                                               │
│         │                                                        │
│  ┌──────▼───────────┐                                          │
│  │ Lambda           │                                           │
│  │ (Orchestrator)   │                                           │
│  │                  │                                           │
│  │ - Validate API   │         ┌──────────────┐                │
│  │   key            │────────►│ DynamoDB     │                │
│  │ - Check lock     │         │ - API Keys   │                │
│  │ - Start ECS task │         │ - Locks      │                │
│  │ - Record exec    │         │ - Executions │                │
│  └──────┬───────────┘         └──────────────┘                │
│         │                                                        │
│  ┌──────▼───────────┐                                          │
│  │ ECS Fargate      │                                           │
│  │                  │                                           │
│  │ Container:       │         ┌──────────────┐                │
│  │ - Clone git repo │────────►│ S3 Bucket    │                │
│  │   (optional)     │         │ - Code       │                │
│  │ - Run command    │         │   uploads    │                │
│  │ - Stream logs    │         └──────────────┘                │
│  │                  │                                           │
│  │ Task Role:       │                                           │
│  │ - AWS perms for  │         ┌──────────────┐                │
│  │   actual work    │────────►│ CloudWatch   │                │
│  └──────────────────┘         │ Logs         │                │
│                                └──────────────┘                │
│                                                                  │
│  ┌──────────────────────────────────────────┐                 │
│  │ Web UI (S3 + CloudFront)                 │                 │
│  │ - Static site for viewing logs           │                 │
│  │ - Token-based access (no login)          │                 │
│  │ - Real-time log streaming                │                 │
│  └──────────────────────────────────────────┘                 │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────┐
│ Users           │
│                 │
│ - CLI with API  │
│   key (no AWS   │
│   credentials)  │
│                 │
│ - Web browser   │
│   for viewing   │
│   logs          │
└─────────────────┘

## Error Handling System

The application uses a structured error handling system (`internal/errors`) that distinguishes between client errors (4xx) and server errors (5xx), ensuring proper HTTP status codes are returned.

### Error Types

**Client Errors (4xx):**
- `ErrUnauthorized` (401): General unauthorized access
- `ErrInvalidAPIKey` (401): Invalid API key provided
- `ErrAPIKeyRevoked` (401): API key has been revoked
- `ErrNotFound` (404): Resource not found
- `ErrConflict` (409): Resource conflict (e.g., user already exists)
- `ErrBadRequest` (400): Invalid request parameters

**Server Errors (5xx):**
- `ErrInternalError` (500): Internal server errors
- `ErrDatabaseError` (503): Database/transient service failures

### Error Structure

All errors are wrapped in `AppError` which includes:
- `Code`: Programmatic error code (e.g., `INVALID_API_KEY`, `DATABASE_ERROR`)
- `Message`: User-friendly error message
- `StatusCode`: HTTP status code to return
- `Cause`: Underlying error (for error wrapping)

### Error Propagation

**Database Layer (`internal/database/dynamodb`):**
- DynamoDB errors are wrapped as `ErrDatabaseError` (503 Service Unavailable)
- Conditional check failures (e.g., user already exists) become `ErrConflict` (409)
- User not found scenarios return `nil` user (not an error)

**Service Layer (`internal/app`):**
- Validates input and returns appropriate client errors (400, 401, 404, 409)
- Propagates database errors as-is (preserving 503 status codes)
- Maps business logic failures to appropriate error types

**HTTP Layer (`internal/server`):**
- Extracts status codes from errors using `GetStatusCode()`
- Extracts error codes using `GetErrorCode()`
- Returns structured error responses with codes in JSON

### Key Distinction: Database Errors vs Authentication Failures

**Critical Behavior:**
- Database errors during authentication (e.g., DynamoDB unavailable) → **503 Service Unavailable**
- Invalid or revoked API keys → **401 Unauthorized**
- This prevents database failures from being misinterpreted as authentication failures

**Example Flow:**
1. User provides API key
2. Database query fails (network timeout) → Returns 503 Service Unavailable
3. User provides invalid API key → Database query succeeds but user not found → Returns 401 Unauthorized

### Error Response Format

All error responses follow this JSON structure:
```json
{
  "error": "Error message",
  "code": "ERROR_CODE",
  "details": "Detailed error information"
}
```

The `code` field is optional and provides programmatic error codes for clients.

## Development Tools

### Code Quality and Linting

The project uses **golangci-lint** for comprehensive Go code analysis with the following configuration:

- **Configuration**: `.golangci.yml` with reasonable defaults for production Go code
- **Enabled Linters**: 30+ linters including staticcheck, govet, gosec, gocritic, and more
- **Exclusions**: Test files and command packages have relaxed rules for complexity and magic numbers
- **Timeout**: 5-minute timeout for large codebases

### Pre-commit Hooks

**Pre-commit** hooks ensure code quality before commits:

- **Configuration**: `.pre-commit-config.yaml`
- **Hooks**: 
  - golangci-lint for Go code analysis
  - gofmt for code formatting
  - goimports for import organization
  - Standard file checks (trailing whitespace, YAML validation, etc.)

### Development Commands

The `justfile` provides convenient development commands:

```bash
# Setup development environment
just dev-setup

# Install pre-commit hooks
just install-hooks

# Lint all code
just lint

# Lint and auto-fix issues
just lint-fix

# Format code
just fmt

# Run all checks (lint + test)
just check

# Run pre-commit on all files
just pre-commit-all
```

### Agent Integration

AI agents can automatically:
- Run `just lint-fix` to fix auto-fixable issues
- Run `just fmt` to format code
- Run `just check` to validate changes
- Use pre-commit hooks to ensure quality before commits

This setup ensures consistent code quality across all contributors and automated systems.

## CLI Client Architecture

The CLI client (`cmd/runvoy`) provides command-line access to the runvoy platform.

### Configuration

The CLI stores configuration in `~/.runvoy/config.yaml`:

```yaml
api_endpoint: "https://your-function-url.lambda-url.us-east-1.on.aws"
api_key: "your-api-key-here"
```

Configuration is loaded on-demand for each command execution and requires authentication for all operations.

### Generic HTTP Client Architecture

The CLI uses a generic HTTP client abstraction (`internal/client`) that can be reused across all commands, providing a simple and consistent way to make API requests.

#### Package Structure

- **`internal/client/client.go`**: Generic HTTP client for all API operations
- **`internal/user/client.go`**: User-specific operations using the generic client

#### Generic Client (`internal/client/client.go`)

The generic client provides a simple abstraction for all API operations:

- **`Client`**: Contains configuration and logger instances
- **`Do()`**: Makes raw HTTP requests and returns response data
- **`DoJSON()`**: Makes requests and automatically unmarshals JSON responses
- **Error Handling**: Standardized error parsing for all API responses
- **Logging**: Consistent request/response logging across all commands

#### Command-Specific Clients

Each command type has its own client that uses the generic client:

- **`internal/user/client.go`**: User management operations (create, revoke, etc.)
- **Future clients**: `internal/exec/client.go`, `internal/logs/client.go`, etc.

**Benefits:**
- **Consistency**: All commands use the same HTTP client logic
- **Simplicity**: New commands only need to define their specific operations
- **Maintainability**: HTTP client logic is centralized and reusable
- **Testability**: Generic client can be easily mocked for testing

#### User Management Commands

##### Create User (`users create <email>`)

The command uses the generic HTTP client through the user client:

**Implementation:**
- Located in `cmd/runvoy/cmd/addUser.go`
- Uses `user.New()` to create a user client
- Calls `userClient.CreateUser(email)` for the actual operation
- Simplified from 80+ lines to ~15 lines

**How it works:**
1. Command loads configuration
2. Creates user client with generic HTTP client
3. User client uses `client.DoJSON()` to make API request
4. Generic client handles all HTTP details (headers, error parsing, etc.)

**Error Handling:**
- 400 Bad Request: Invalid email format or missing email
- 401 Unauthorized: Invalid API key
- 409 Conflict: User already exists
- 500 Internal Server Error: Server errors

**Adding New Commands:**
To add new commands (exec, logs, etc.), simply:
1. Create `internal/{command}/client.go`
2. Use `client.New()` to create generic client
3. Define command-specific methods using `client.DoJSON()`
4. Add Cobra command that uses the client

**Example for future exec command:**
```go
// internal/exec/client.go
func (e *ExecClient) RunCommand(cmd string) error {
    req := client.Request{
        Method: "POST",
        Path:   "exec/run",
        Body:   api.ExecutionRequest{Command: cmd},
    }
    return e.client.DoJSON(req, &result)
}
```
