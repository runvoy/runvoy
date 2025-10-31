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

## Folder Structure

```text
runvoy/
├── bin/
├── cmd/
├── dist/
├── infra/
├── internal/
├── scripts/
```

- `bin/`: built binaries for the runvoy application (temporary storage for building artifacts during development).
- `cmd/`: main entry points for the various application (CLI, local dev server, lambdas, etc.)
- `dist/`: built binaries for the runvoy application.
- `infra/`: infrastructure as code for the runvoy application (CloudFormation templates, etc.).
- `internal/`: core logic of the runvoy application (business logic, API, database, etc.)
- `scripts/`: scripts for the runvoy application development and deployment

## Execution Provider Abstraction

To support multiple cloud platforms, the service layer now depends on an execution provider interface:

```text
internal/app.Service → uses Runner interface (provider-agnostic)
internal/app/aws     → AWS-specific Runner implementation (ECS Fargate)
```

- The `Runner` interface abstracts starting a command execution and returns both a stable execution ID and provider task ARN.
- The AWS implementation resides in `internal/app/aws` and encapsulates all ECS- and AWS-specific logic and types.
- `internal/app/init.go` wires the chosen provider by constructing the appropriate `Runner` and passing it into `Service`.

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
GET  /api/v1/health                         - Health check
POST /api/v1/users/create                   - Create a new user with an API key
POST /api/v1/users/revoke                   - Revoke a user's API key
POST /api/v1/run                            - Start an execution
GET  /api/v1/executions                     - List executions (queried via DynamoDB GSI)
GET  /api/v1/executions/{id}/logs           - Fetch execution logs (CloudWatch)
GET  /api/v1/executions/{id}/status         - Get execution status (RUNNING/SUCCEEDED/FAILED/STOPPED)
POST /api/v1/executions/{id}/kill           - Terminate a running execution
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

**Post-Authentication Behavior:**
- On successful authentication, the system asynchronously updates the user's `last_used` timestamp in the API keys table (best-effort; failures are logged and do not affect the request).

The request ID middleware automatically:
- Extracts the AWS Lambda request ID from the Lambda context when available
- Adds the request ID to the request context for use by handlers
- Falls back gracefully when not running in Lambda environment

### Execution Records: Compute Platform, and Request ID

- The service includes the request ID (when available) in execution records created in `internal/app.Service.RunCommand()`.
- The `request_id` is persisted in DynamoDB via the `internal/database/dynamodb` repository.
- If a request ID is not present (e.g., non-Lambda environments), the service logs a warning and stores the execution without a `request_id`.
- The `compute_platform` field in execution records is derived from the configured backend provider at initialization time (e.g., `AWS`) rather than being hardcoded in the service logic.

### Execution ID Uniqueness and Write Semantics

- Execution records are written with a conditional create to ensure no overwrite occurs for an existing execution item.
- DynamoDB `PutItem` uses a `ConditionExpression` preventing creation when a record with the same composite key (`execution_id`, `started_at`) already exists.
- On conditional failure, the API surfaces a 409 Conflict (via `ErrConflict`).
- Note: The system creates a single record per `execution_id`. If future designs require multiple items per `execution_id`, a separate uniqueness guard pattern would be needed.

## Logging Architecture

The application uses a unified logging approach with structured logging via `log/slog`:

### Logger Initialization
- Logger is initialized once at application startup in `internal/logger/logger.go`
- Configuration supports both development (human-readable) and production (JSON) formats
- Log level is configurable via `RUNVOY_LOG_LEVEL` environment variable

### Service-Level Logging
- Each `Service` instance contains its own logger instance (`Service.Logger`)
- Service methods that receive a `context.Context` derive a request-scoped logger using the Lambda request ID when available: `reqLogger := s.Logger.With("requestID", AwsRequestID)`
- This keeps logs consistent with router/handler logs and ensures traceability across layers

### Request-Scoped Logging
- A helper `logger.DeriveRequestLogger(ctx, base)` builds a request-scoped logger from context
- Currently extracts AWS Lambda request ID; future providers can be added centrally
- Router/handlers, services, and repositories use this helper to keep logs consistently tagged

### Request Logging Middleware
- **Automatic Request Logging**: The request logging middleware automatically logs all incoming requests
- Logs include: HTTP method, path, remote address, status code, and request duration
- Both incoming requests and completed responses are logged for complete request lifecycle visibility
- Implementation: `internal/server/router.go` lines 115-153
- The middleware uses a response writer wrapper to capture response status codes and measure execution time
- Remote address is automatically available in both local and Lambda executions via the Lambda adapter

### Database Layer Logging
- Database repositories receive the base service logger during initialization
- Repository methods derive a request-scoped logger from the call context (when a Lambda request ID is present) so their logs include `requestID`
- This maintains consistent, end-to-end traceability for a request across middleware, handlers, services, and repositories

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
│  │ Function URL │         header                                │
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
│  └──────┬───────────┘         └──────┬───────┘                │
│         │                             ▲                          │
│         │                             │                          │
│  ┌──────▼───────────┐                │                          │
│  │ ECS Fargate      │                │                          │
│  │                  │                │                          │
│  │ Container:       │         ┌──────────────┐                │
│  │ - Clone git repo │────────►│ S3 Bucket    │                │
│  │   (optional)     │         │ - Code       │                │
│  │ - Run command    │         │   uploads    │                │
│  │ - Stream logs    │         └──────────────┘                │
│  │                  │                                           │
│  │ Task Role:       │                                           │
│  │ - AWS perms for  │         ┌──────────────┐                │
│  │   actual work    │────────►│ CloudWatch   │                │
│  └──────┬───────────┘         │ Logs         │                │
│         │                      └──────────────┘                │
│         │ Task stops                                            │
│         │                                                        │
│  ┌──────▼───────────┐                                          │
│  │ EventBridge      │                                           │
│  │ Rule             │                                           │
│  │ - ECS Task State │                                           │
│  │   Change         │                                           │
│  │ - Filter STOPPED │                                           │
│  └──────┬───────────┘                                          │
│         │ Event                                                 │
│         │                                                        │
│  ┌──────▼───────────┐                                          │
│  │ Lambda           │                                           │
│  │ (Event           │                                           │
│  │  Processor)      │                                           │
│  │                  │                                           │
│  │ - Route event    │────────────────────────────────────────┘│
│  │ - Extract data   │                                           │
│  │ - Calculate cost │                                           │
│  │ - Update exec    │                                           │
│  └──────────────────┘                                          │
│                                                                  │
│  ┌──────────────────────────────────────────┐                 │
│  │ Web Viewer (S3-hosted)                   │                 │
│  │ - Single HTML file with embedded JS/CSS  │                 │
│  │ - Real-time log streaming (5s polling)   │                 │
│  │ - ANSI color support for terminal output │                 │
│  │ - Status tracking and metadata display   │                 │
│  │ - LocalStorage-based authentication      │                 │
│  │ - Pico.css styling framework             │                 │
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

## Event Processor Architecture

The platform uses a dedicated **event processor Lambda** to handle asynchronous events from AWS services. This provides a clean separation between synchronous API requests (handled by the orchestrator) and asynchronous event processing.

### Design Pattern

- **Orchestrator Lambda**: Handles synchronous HTTP API requests
- **Event Processor Lambda**: Handles asynchronous events from EventBridge
- Both Lambdas are independent, scalable, and focused on their specific domain

### Event Processing Flow

1. **ECS Task Completion**: When an ECS Fargate task stops, AWS generates an "ECS Task State Change" event
2. **EventBridge Filtering**: EventBridge rule captures STOPPED tasks from the runvoy cluster only
3. **Lambda Invocation**: EventBridge invokes the event processor Lambda with the task details
4. **Event Routing**: The processor routes events by type (extensible for future event types)
5. **Data Extraction**: 
   - Execution ID extracted from task ARN (last segment)
   - Exit code from container details
   - Timestamps for start/stop times
6. **Cost Calculation**: Fargate cost computed based on vCPU, memory, and duration
7. **DynamoDB Update**: Execution record updated with:
   - Final status (SUCCEEDED, FAILED, STOPPED)
   - Exit code
   - Completion timestamp
   - Duration in seconds
   - Computed cost in USD

### Event Types

Currently handles:
- **ECS Task State Change**: Updates execution records when tasks complete

Designed to be extended for future event types:
- CloudWatch Alarms
- S3 events
- Custom application events

### Implementation

**Entry Point**: `cmd/backend/aws/event-processor/main.go`
- Initializes event processor
- Starts Lambda handler

**Event Routing**: `internal/events/processor.go`
- Routes events by `detail-type`
- Ignores unknown event types (log and continue)
- Extensible switch statement for new handlers

**ECS Handler**: `internal/events/ecs_completion.go`
- Parses ECS task events
- Extracts execution ID from task ARN
- Determines final status from exit code and stop reason
- Calculates duration and cost
- Updates DynamoDB execution record

**Cost Calculator**: `internal/events/cost.go`
- Fargate ARM64 pricing (us-east-1)
- vCPU: $0.04048/hour
- Memory: $0.004445/GB/hour
- Accurate per-second billing

### Status Determination Logic

```go
switch stopCode {
case "UserInitiated":
    status = "STOPPED"    // Manual termination
case "EssentialContainerExited":
    if exitCode == 0:
        status = "SUCCEEDED"
    else:
        status = "FAILED"
case "TaskFailedToStart":
    status = "FAILED"
}
```

### Execution Status Types

Runvoy defines two distinct status type systems:

1. **ExecutionStatus** (`constants.ExecutionStatus`): Business-level execution status used throughout the API
   - `RUNNING`: Command is currently executing
   - `SUCCEEDED`: Command completed successfully (exit code 0)
   - `FAILED`: Command failed with an error (non-zero exit code)
   - `STOPPED`: Command was manually terminated by user

2. **EcsStatus** (`constants.EcsStatus`): AWS ECS task lifecycle status returned by ECS API
   - `PROVISIONING`, `PENDING`, `ACTIVATING`, `RUNNING`, `DEACTIVATING`, `STOPPING`, `DEPROVISIONING`, `STOPPED`
   - These are used internally for ECS task management and should not be confused with execution status

Execution status values are defined as typed constants in `internal/constants/constants.go` to ensure consistency across the codebase and as part of the API contract. This prevents typos and makes the valid status values explicit to developers.

### Error Handling

- **Orphaned Tasks**: Tasks without execution records are logged and skipped (no failure)
- **Parse Errors**: Malformed events are logged and returned as errors
- **Database Errors**: Failed updates are logged and returned as errors (Lambda retries)
- **Unknown Events**: Unhandled event types are logged and ignored

### Benefits

- ✅ **Event-Driven**: No polling, near real-time updates (< 1 second)
- ✅ **Cost-Efficient**: Only pay for Lambda invocations on events
- ✅ **Scalable**: Handles any event volume automatically
- ✅ **Extensible**: Easy to add new event handlers without infrastructure changes
- ✅ **Reliable**: EventBridge guarantees at-least-once delivery
- ✅ **Separation of Concerns**: Sync API vs async events

### CloudFormation Resources

- **`EventProcessorFunction`**: Lambda function for event processing
- **`EventProcessorRole`**: IAM role with DynamoDB and ECS permissions
- **`EventProcessorLogGroup`**: CloudWatch Logs for event processor
- **`TaskCompletionEventRule`**: EventBridge rule filtering ECS task completions
- **`EventProcessorEventPermission`**: Permission for EventBridge to invoke Lambda

## Web Viewer Architecture

The platform includes a minimal web-based log viewer for visualizing execution logs in a browser. This provides an alternative to the CLI for teams who prefer a graphical interface.

### Implementation

**Location**: `cmd/webviewer/index.html`
**Deployment**: Hosted on AWS S3 at `https://runvoy-releases.s3.us-east-2.amazonaws.com/webviewer.html`
**Architecture**: Single-page application (SPA) - standalone HTML file with embedded CSS and JavaScript

### Technology Stack

- **Frontend**: Vanilla JavaScript + HTML5
- **Styling**: Pico.css v2 (CSS framework) - adopted in commit d04515b for improved UI and responsiveness
- **State Management**: Client-side (localStorage + JavaScript variables)
- **Polling**: 5-second intervals for logs and status updates
- **API Integration**: Fetch API for RESTful API calls

### Core Features

1. **Real-time Log Streaming**
   - Polls API every 5 seconds for new logs
   - Auto-scrolls to bottom on new logs
   - Handles rate limiting and backpressure

2. **ANSI Color Support**
   - Parses and displays colored terminal output
   - Maintains terminal formatting in the browser

3. **Status Tracking**
   - Displays execution status (RUNNING, SUCCEEDED, FAILED, STOPPED)
   - Shows execution metadata (ID, start time, exit codes)
   - Updates in real-time as execution progresses

4. **Interactive Controls**
   - Pause/Resume polling
   - Download logs as text file
   - Clear display
   - Toggle metadata (line numbers and timestamps)

5. **Authentication**
   - API endpoint URL configuration
   - API key authentication (stored in localStorage)
   - First-time setup wizard
   - Persistent credentials across sessions

### API Endpoints Used

The web viewer interacts with two API endpoints:

- `GET /api/v1/executions/{id}/status` - Fetch execution status and metadata
- `GET /api/v1/executions/{id}/logs` - Fetch execution logs

Both endpoints require authentication via `X-API-Key` header.

### Access Pattern

Users access the web viewer via URL with execution ID as query parameter:

```
https://runvoy-releases.s3.us-east-2.amazonaws.com/webviewer.html?execution_id={executionID}
```

The CLI automatically provides this link when running commands (see `cmd/runvoy/cmd/logs.go:58-59`).

### Configuration

The web viewer URL is defined as a constant in `internal/constants/constants.go:118`:

```go
const WebviewerURL = "https://runvoy-releases.s3.us-east-2.amazonaws.com/webviewer.html"
```

### Browser Requirements

- Modern browser with Fetch API support
- JavaScript enabled
- localStorage support (for persistent configuration)

### Recent Improvements

**Commit d04515b (October 30, 2025)**: Refactored to adopt Pico.css framework
- Replaced custom dark theme with Pico.css
- Added responsive grid layouts for controls
- Improved mobile/tablet support
- Maintained all existing functionality

### Benefits

- ✅ **No Installation Required**: Runs in any modern browser
- ✅ **Real-time Updates**: Automatic log streaming without manual refresh
- ✅ **User-Friendly**: Visual interface for non-CLI users
- ✅ **Portable**: Single HTML file, easy to deploy anywhere
- ✅ **Responsive**: Works on mobile, tablet, and desktop
- ✅ **Stateless**: No backend server needed, communicates directly with API

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

The `justfile` codifies the common build, deploy, and validation flows. Highlights:

- **CLI passthrough**: `just runvoy <args…>` rebuilds `cmd/runvoy` with version metadata and executes it, making it easy to test commands while always running a fresh binary.
- **Build outputs**: `just build` fans out to `just build-cli`, `just build-local`, `just build-orchestrator`, and `just build-event-processor`, ensuring all deployable binaries are rebuilt with consistent linker flags. Individual targets can be invoked when iterating on a single component.
- **Packaging & deploy**: `just build-orchestrator-zip` and `just build-event-processor-zip` stage Lambda-ready artifacts; `just deploy`, `just deploy-orchestrator`, `just deploy-event-processor`, and `just deploy-webviewer` push those artifacts (and the web viewer) to the release bucket and update Lambda code.
- **Local iteration**: `just run-local` launches the local HTTP server; `just local-dev-server` wraps it with `reflex` for hot reload. Smoke tests (`just smoke-test-*`) exercise the API (local or Lambda URL) once the server is running.
- **Quality gates**: `just test`, `just test-coverage`, `just lint`, `just lint-fix`, `just fmt`, `just check`, and `just clean` provide the standard Go QA loop.
- **Environment & infra helpers**: `just dev-setup`, `just install-hooks`, `just pre-commit-all` prepare developer machines, while `just create-lambda-bucket`, `just update-backend-infra`, `just seed-admin-user`, and `just destroy-backend-infra` manage AWS prerequisites.
- **DX utilities**: `just record-demo` regenerates the terminal cast/GIF assets.

**Environment Configuration:**

The `justfile` uses `set dotenv-required`, which means all `just` commands require a `.env` file to be present in the repository root. Developers should:

1. Copy `.env.example` to `.env`: `cp .env.example .env`
2. Edit `.env` with their actual environment variable values
3. Ensure `.env` is not committed to version control (already in `.gitignore`)

If `.env` is missing, `just` commands will fail with an error, ensuring developers configure their environment before running commands.

### Agent Integration

Automation-friendly targets remain the same: agents should prefer `just lint-fix`, `just fmt`, `just check`, and the smoke tests where appropriate. These commands ensure consistent code quality across contributors and automated systems.

## Current Limitations and Future Enhancements

### Log Viewing and Tailing

The service exposes a logs endpoint that aggregates CloudWatch Logs events for a given execution ID (ECS task ID). The response now includes a monotonically increasing `line` number for each event. The CLI implements client-side tailing behavior on top of this endpoint:

- Auth required via `X-API-Key`
- Returns all available events across discovered streams containing the task ID
- Reads from deterministic stream: `task/<container-name>/<executionID>` (container: `executor`)
- Response is sorted by timestamp ascending

Error behavior:

- 500 Internal Server Error when the configured CloudWatch Logs group does not exist (or other AWS errors)
- 404 Not Found when the expected log stream for the execution ID does not exist yet (clients may retry later)

Example response:

```json
{
  "execution_id": "abc123",
  "events": [
    {"timestamp": 1730250000000, "message": "Execution starting"},
    {"timestamp": 1730250005000, "message": "..."}
  ]
}
```

Environment variables:

```text
RUNVOY_LOG_GROUP           # required (e.g. /aws/ecs/runvoy)
```

Client behavior (CLI `runvoy logs <executionID>`):

- Waits until the execution moves out of pending/queued/starting using the status API, showing a spinner
- By default (no flags), fetches and prints all logs once with a `Line` column and exits
- With `--follow` (`-f`), streams logs, polling every 5 seconds and printing only new lines (based on `line`)
- Stops tailing when the execution reaches a terminal status (COMPLETED/SUCCEEDED/FAILED/etc.) and prints the final status badge

Future enhancements may include server-side filtering and pagination.

2. **Lock Enforcement** - Lock names are stored in execution records but not actively enforced. Future implementation:
   - Acquire locks before starting tasks
   - Release locks on completion
   - Prevent concurrent executions with the same lock

3. **Custom Container Images** - The `image` field in execution requests is accepted but not currently used. Future implementation:
   - Support custom Docker images via task definition overrides
   - Allow per-execution image specification
   - Validate image availability before task start

4. **Comprehensive Test Coverage** - Current test coverage is limited. Areas needing tests:
   - Event processor logic (cost calculation, status determination)
   - DynamoDB repository operations
   - API request/response handling
   - End-to-end integration tests
   - Web viewer functionality and API integration

5. **Request ID in Non-Lambda Environments** - Request ID extraction currently only works in Lambda. Enhancement needed for local server:
   - Generate request IDs in middleware
   - Use X-Request-ID header if present
   - Consistent request tracking across environments

### Implemented and Working

- ✅ **Event Processor Lambda** - Fully implemented with ECS task completion tracking
- ✅ **Cost Tracking** - Fargate cost calculation per execution
- ✅ **API Key Authentication** - SHA-256 hashed keys with last-used tracking
- ✅ **User Management** - Create and revoke users via CLI
- ✅ **Command Execution** - Remote command execution via ECS Fargate
- ✅ **Execution Records** - Complete tracking with status, duration, and cost
- ✅ **Error Handling** - Structured errors with proper HTTP status codes
- ✅ **Logging** - Request-scoped logging with AWS request ID
- ✅ **Local Development** - HTTP server for testing without AWS
- ✅ **Provider Abstraction** - Runner interface for multi-cloud support
- ✅ **Automated Admin Seeding** - Infra update step seeds an admin user into DynamoDB using the SHA-256 + base64 hash of the API key from the global CLI config (idempotent via conditional write). Requires `RUNVOY_ADMIN_EMAIL` to be set during deployment.
- ✅ **Web Viewer** - Browser-based log viewer with real-time streaming, ANSI color support, and responsive UI (Pico.css)

## CLI Client Architecture

The CLI client (`cmd/runvoy`) provides command-line access to the runvoy platform.

### Configuration

The CLI stores configuration in `~/.runvoy/config.yaml`:

```yaml
api_endpoint: "https://your-function-url.lambda-url.us-east-1.on.aws"
api_key: "your-api-key-here"
```

Configuration is loaded once in the root command's `PersistentPreRunE` hook and stored in the command context. All commands retrieve the config from the context using the `getConfigFromContext()` helper function. This ensures config is only loaded once per command invocation and provides a consistent way to access configuration across all commands.

The config is stored in the context with the key `constants.ConfigCtxKey` (of type `constants.ConfigCtxKeyType` to avoid collision with other context keys).

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
To add new commands (logs, etc.), simply:
1. Create `internal/{command}/client.go`
2. Use `client.New()` to create generic client
3. Define command-specific methods using `client.DoJSON()`
4. Add Cobra command that uses the client

**Example for future logs command:**
```go
// internal/logs/client.go
func (l *LogsClient) GetLogs(executionID string) (*api.LogsResponse, error) {
    req := client.Request{
        Method: "GET",
        Path:   fmt.Sprintf("executions/%s/logs", executionID),
    }
    var result api.LogsResponse
    return &result, l.client.DoJSON(req, &result)
}
```

## Command Execution (`runvoy run`)

The `run` command executes commands remotely via the orchestrator Lambda.

**Implementation:** `cmd/runvoy/cmd/run.go`

**Request Flow:**
1. User runs: `runvoy run "echo hello world"`
2. CLI sends POST request to `/api/v1/run` with:
   ```json
   {
     "command": "echo hello world",
     "lock": "optional_lock_name",
     "image": "ubuntu:22.04",
     "env": {"VAR": "value"},
     "timeout": 300
   }
   ```
3. Orchestrator Lambda:
   - Validates API key
   - Creates execution record in DynamoDB (status: RUNNING)
   - Starts ECS Fargate task with the command
   - Returns execution ID

**Response (202 Accepted):**
```json
{
  "execution_id": "abc123...",
  "log_url": "/api/v1/executions/abc123/logs/notimplemented",
  "status": "RUNNING"
}
```

**Note:** Log viewing endpoints are not yet implemented. Execution completion is tracked automatically by the event processor Lambda.

## Execution Termination (`POST /api/v1/executions/{id}/kill`)

The kill endpoint allows users to terminate running executions. This endpoint provides safe termination by validating that executions exist in the database and checking task status before termination.

**Implementation:** `internal/server/handlers.go` → `handleKillExecution`

**Request Flow:**
1. User sends POST request to `/api/v1/executions/{executionID}/kill`
2. Orchestrator Lambda:
   - Validates API key
   - Verifies execution exists in the database (returns 404 if not found)
   - Checks execution status (returns 400 if already terminated)
   - Checks ECS task status (only terminates RUNNING or ACTIVATING tasks)
   - Sends StopTask API call to ECS
   - Returns success response

**Response (200 OK):**
```json
{
  "execution_id": "abc123...",
  "message": "Execution termination initiated"
}
```

**Error Responses:**
- 400 Bad Request: Execution is already terminated or task cannot be terminated in current state
- 404 Not Found: Execution not found in database or task not found in ECS
- 500 Internal Server Error: AWS API errors or other server errors

**Safety Features:**
- Only terminates executions that exist in the execution table
- Avoids killing tasks that are already terminated (STOPPED, STOPPING, DEACTIVATING)
- Only terminates tasks in RUNNING or ACTIVATING states
- Verifies task status via ECS DescribeTasks before termination

**Implementation Details:**
- Service layer (`internal/app/main.go`): `KillExecution` validates execution exists and checks status
- AWS Runner (`internal/app/aws/runner.go`): `KillTask` finds task via ListTasks, checks status, and calls StopTask
- ECS task lifecycle statuses (`LastStatus`) are represented by `constants.EcsStatus` typed constants (e.g., `EcsStatusRunning`, `EcsStatusStopped`) to avoid string literals across the codebase
- Execution status values are represented by `constants.ExecutionStatus` typed constants (e.g., `ExecutionRunning`, `ExecutionSucceeded`, `ExecutionFailed`, `ExecutionStopped`) as part of the API contract
- Requires `ecs:StopTask` and `ecs:ListTasks` IAM permissions (added to CloudFormation template)

**Post-Termination:**
- The event processor Lambda will automatically detect the task termination
- Execution status will be updated to STOPPED with exit code 130 (SIGINT)
- Completion timestamp and duration will be calculated automatically
