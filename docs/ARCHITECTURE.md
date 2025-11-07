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
├── deployments/
├── docs/
├── internal/
├── scripts/
```

- `bin/`: built binaries for the runvoy application (temporary storage for building artifacts during development).
- `cmd/`: main entry points for the various application (CLI, local dev server, lambdas, etc.)
- `deployments/`: infrastructure as code for the runvoy application (CloudFormation templates, etc.).
- `docs/`: project documentation (architecture, testing strategy, etc.).
- `internal/`: core logic of the runvoy application (business logic, API, database, etc.)
- `scripts/`: scripts for the runvoy application development and deployment

## Execution Provider Abstraction

To support multiple cloud platforms, the service layer now depends on an execution provider interface:

```text
internal/app.Service → uses Runner interface (provider-agnostic)
internal/app/aws     → AWS-specific Runner implementation (ECS Fargate)
```

- The `Runner` interface abstracts starting a command execution and returns a stable execution ID and the task creation timestamp.
- The AWS implementation resides in `internal/app/aws` and encapsulates all ECS- and AWS-specific logic and types.
- `internal/app/init.go` wires the chosen provider by constructing the appropriate `Runner` and passing it into `Service`.

This change removes direct AWS SDK coupling from `internal/app` and makes adding providers (e.g., GCP) straightforward.

## Router Architecture

The application uses **chi** (github.com/go-chi/chi/v5) as the HTTP router for both Lambda and local HTTP server implementations. This provides a consistent routing API across deployment models.

### Components

- **`internal/server/router.go`**: Shared chi-based router configuration with all API routes
- **`internal/server/middleware.go`**: Middleware for request ID extraction and logging context
- **`internal/lambdaapi/handler.go`**: Lambda handler that uses algnhsa to adapt the chi router
- **`cmd/local/main.go`**: Local HTTP server implementation using the same router
- **`cmd/backend/aws/orchestrator/main.go`**: Lambda entry point for the orchestrator (uses the chi-based handler)

### Route Structure

All routes are defined in `internal/server/router.go`:

```text
GET    /api/v1/health                   - Health check
GET    /api/v1/users                    - List all users (admin)
POST   /api/v1/users/create             - Create a new user with a claim URL (admin)
POST   /api/v1/users/revoke             - Revoke a user's API key (admin)
GET    /api/v1/images                   - List all registered Docker images (admin)
POST   /api/v1/images/register          - Register a new Docker image (admin)
DELETE /api/v1/images/{image}           - Remove a registered Docker image (admin)
POST   /api/v1/run                      - Start an execution
GET    /api/v1/executions               - List executions (queried via DynamoDB GSI)
GET    /api/v1/executions/{id}/logs     - Fetch execution logs (CloudWatch)
GET    /api/v1/executions/{id}/status   - Get execution status (RUNNING/SUCCEEDED/FAILED/STOPPED)
POST   /api/v1/executions/{id}/kill     - Terminate a running execution
GET    /api/v1/claim/{token}            - Claim a pending API key (public, no auth required)
```

Both Lambda and local HTTP server use identical routing logic, ensuring development/production parity.

### Lambda Event Adapter

The platform uses **algnhsa** (`github.com/akrylysov/algnhsa`), a well-maintained open-source library that adapts standard Go `http.Handler` implementations (like chi routers) to work with AWS Lambda. This eliminates the need for custom adapter code and provides robust support for multiple Lambda event types.

**Implementation:** `internal/lambdaapi/handler.go` creates the Lambda handler by wrapping the chi router with `algnhsa.New()`:

```go
func NewHandler(svc *app.Service, requestTimeout time.Duration) lambda.Handler {
    router := server.NewRouter(svc, requestTimeout)
    return algnhsa.New(router.Handler(), nil)
}
```

**Supported Event Types:**
- Lambda Function URLs (current deployment model)
- API Gateway v1 (REST API)
- API Gateway v2 (HTTP API)
- Application Load Balancer (ALB)

**Benefits:**
- ✅ **Zero custom adapter code**: The library handles all event type detection and conversion
- ✅ **Battle-tested**: Actively maintained and widely used in production
- ✅ **Automatic conversion**: Transforms Lambda events into standard `http.Request` and `http.ResponseWriter` objects
- ✅ **Transparent to middleware**: All middleware (logging, request ID extraction, authentication) work identically in both Lambda and local environments
- ✅ **Future-proof**: Easy migration between Lambda invocation models without code changes

The adapter ensures that the same router and middleware work seamlessly in both local development (HTTP server) and production (Lambda) environments without any conditional logic.

### User Management API

The system provides endpoints for creating and managing users:

#### List Users (`GET /api/v1/users`)

Lists all users in the system with their basic information. This endpoint requires admin authentication.

**Request:**
- GET request to `/api/v1/users`
- Requires authentication via `X-API-Key` header

**Response (200 OK):**
```json
{
  "users": [
    {
      "email": "user1@example.com",
      "created_at": "2025-10-31T12:00:00Z",
      "revoked": false,
      "last_used": "2025-11-02T10:30:00Z"
    },
    {
      "email": "user2@example.com",
      "created_at": "2025-10-28T09:15:00Z",
      "revoked": true,
      "last_used": "2025-10-30T14:20:00Z"
    }
  ]
}
```

**Error Responses:**
- 401 Unauthorized: Invalid or revoked API key
- 503 Service Unavailable: Database errors (transient failures)

**Security:**
- API key hashes are intentionally excluded from the response to protect sensitive authentication data
- Only non-sensitive user information is returned: email, created_at, revoked status, and last_used timestamp
- Requires valid admin API key for authentication

**Implementation:**
- Uses DynamoDB Scan operation to retrieve all users from the `api-keys` table
- Filters out sensitive fields (api_key_hash) before returning response
- Database errors return 503 (Service Unavailable) rather than 500, indicating transient failures

#### Create User (`POST /api/v1/users/create`)

Creates a new user with an API key and returns a secure one-time claim URL.

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
  "claim_token": "abc-123-def-456"  // One-time token (client constructs URL as {endpoint}/claim/{token})
}
```

**Error Responses:**
- 400 Bad Request: Invalid email format or missing email
- 409 Conflict: User already exists
- 503 Service Unavailable: Database errors (transient failures)

Implementation:
- Email validation using Go's `mail.ParseAddress`
- API key generation using crypto/rand if not provided
- API keys are hashed with SHA-256 before storage in `api-keys` table
- User record created with TTL of 15 minutes (will auto-delete if not claimed)
- Secret token generated and stored in `pending-api-keys` table
- Claim token returned to admin for secure distribution (client constructs URL)
- Database errors return 503 (Service Unavailable) rather than 500, indicating transient failures

#### Claim API Key (`GET /api/v1/claim/{token}`)

Retrieves a pending API key via a one-time claim token. This is a public endpoint that does not require authentication.

**Request:**
- GET request to `/api/v1/claim/{token}` where `token` is the secret token provided by the admin

**Response (200 OK):**
- JSON response containing the API key:
  ```json
  {
    "api_key": "p_75LzCL...",
    "user_email": "alice@example.com",
    "message": "API key claimed successfully"
  }
  ```

**Error Responses (JSON):**
- 400 Bad Request: Invalid token format
- 404 Not Found: Invalid or expired token
- 409 Conflict: Token already claimed

**Implementation:**
- Validates token exists and has not expired (15 minute TTL)
- Checks that token has not been viewed before
- Atomically marks token as viewed with IP address and timestamp
- Removes TTL from user record in `api-keys` table (makes user permanent)
- Returns JSON response with the API key

**CLI Usage:**
Users can claim their API key using the CLI command:
```bash
runvoy claim <token>
```

This command:
1. Calls the `/api/v1/claim/{token}` endpoint
2. Receives the API key in the response
3. Automatically saves it to `~/.runvoy/config.yaml`
4. Allows the user to immediately start using runvoy commands

#### Secure API Key Distribution Flow

1. Admin runs `runvoy users create alice@example.com`
2. System generates API key and secret token
3. User record created with 15-minute TTL in `api-keys` table
4. Pending claim record created in `pending-api-keys` table
5. Claim token returned to admin in JSON response
6. Admin shares claim token with user (e.g., via Slack, email, etc.)
7. User runs `runvoy claim <token>` (user must have endpoint configured first with `runvoy configure`)
8. CLI constructs claim URL: `{endpoint}/api/v1/claim/{token}`
9. System validates token, marks as viewed, removes user TTL
10. User receives API key in JSON response
11. CLI automatically saves API key to `~/.runvoy/config.yaml`
12. User can immediately start using runvoy commands
13. Second attempt to claim shows "already claimed" error

**Security Features:**
- Secret tokens are cryptographically random (base64url encoded, 32 chars)
- Single-use enforcement via atomic DynamoDB operations
- Short expiration (15 minutes)
- IP address logging for audit trail
- HTTPS only (enforced by infrastructure)
- No API keys in URLs, logs, or error messages
- Unclaimed users automatically deleted via TTL

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
│         │
│         │
│  ┌──────▼───────────┐
│  │ ECS Fargate      │
│  │                  │
│  │ Containers:      │
│  │ - Sidecar: git   │
│  │   clone/setup    │
│  │ - Runner: exec   │
│  │ - Stream logs    │
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
6. **DynamoDB Update**: Execution record updated with:
   - Final status (SUCCEEDED, FAILED, STOPPED)
   - Exit code
   - Completion timestamp
   - Duration in seconds
7. **WebSocket Disconnect Notification**: When execution reaches a terminal status, the event processor reuses the WebSocket manager to notify connected clients and clean up connections without invoking a separate Lambda

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
- Handles missing `startedAt` timestamps: When ECS task events have an empty `startedAt` field (e.g., when containers fail before starting, such as sidecar git puller failures), falls back to the execution's `StartedAt` timestamp that was set at creation time
- Calculates duration (with safeguards for negative durations)
- Updates DynamoDB execution record
- Signals WebSocket termination: When execution reaches a terminal status (SUCCEEDED, FAILED, STOPPED), calls `notifyDisconnect()` which delegates to the shared WebSocket manager to send disconnect notifications to all connected clients and clean up connection records

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

- **`EventProcessorFunction`**: Lambda function for event processing and WebSocket lifecycle handling
- **`EventProcessorRole`**: IAM role with DynamoDB, ECS, and API Gateway Management API permissions
- **`EventProcessorLogGroup`**: CloudWatch Logs for event processor
- **`TaskCompletionEventRule`**: EventBridge rule filtering ECS task completions
- **`EventProcessorEventPermission`**: Permission for EventBridge to invoke Lambda

## WebSocket Architecture

The platform uses WebSocket connections for real-time log streaming to clients (CLI and web viewer). The architecture consists of three main components: the event processor Lambda (reusing the WebSocket manager package), the Log Forwarder Lambda, and the API Gateway WebSocket API.

### Design Pattern

- **Event Processor Lambda**: Handles WebSocket connection lifecycle events and execution completion notifications via the shared WebSocket manager package
- **Log Forwarder Lambda**: Forwards CloudWatch Logs events to connected WebSocket clients
- **API Gateway WebSocket API**: Provides WebSocket endpoints for client connections

### Components

#### WebSocket Manager Package

**Purpose**: Manages WebSocket connection lifecycle and sends disconnect notifications when executions complete. It is embedded inside the event processor Lambda rather than deployed as a separate Lambda function.

**Implementation**: `internal/websocket/websocket_manager.go`
- **`HandleRequest()`**: Main entry point that routes WebSocket events by route key

**Route Keys Handled**:

1. **`$connect`** (`handleConnect`):
   - Stores WebSocket connection in DynamoDB with execution ID
   - Sets TTL for connection record (24 hours by default)
   - Returns success response

2. **`$disconnect`** (`handleDisconnect`):
   - Removes WebSocket connection from DynamoDB when client disconnects
   - Cleans up connection record

3. **`$disconnect-execution`** (`handleDisconnectExecution`):
   - Invoked internally by the event processor when execution reaches terminal status
   - Sends disconnect notifications to all connected clients for an execution
   - Deletes all connection records for the execution from DynamoDB
   - Uses `handleDisconnectNotification()` to send messages concurrently to all connections

**Connection Management**:
- Connections stored in DynamoDB `{project-name}-websocket-connections` table
- Each connection record includes: `connection_id`, `execution_id`, `functionality`, `expires_at`
- Connections are queried by execution ID for log forwarding

#### API Gateway WebSocket API

**Configuration**: Defined in CloudFormation template (`deployments/cloudformation-backend.yaml`)

**Routes**:
- **`$connect`**: Routes to the event processor Lambda
- **`$disconnect`**: Routes to the event processor Lambda
- **`$disconnect-execution`**: Custom route invoked programmatically by the event processor

**Integration**:
- Uses AWS_PROXY integration type
- All routes integrate with the event processor Lambda, which delegates to the shared WebSocket manager package
- API Gateway Management API used for sending messages to connections

### Execution Completion Flow

When an execution reaches a terminal status (SUCCEEDED, FAILED, STOPPED):

1. **Event Processor** (`internal/events/ecs_completion.go`):
   - Updates execution record in DynamoDB with final status
   - Calls `notifyDisconnect()` which invokes the shared WebSocket manager package in-process
   - Passes execution ID as query parameter

2. **WebSocket Manager Package** (`handleDisconnectExecution`):
   - Handles `$disconnect-execution` route key event
   - Queries DynamoDB for all connections for the execution ID
   - Sends disconnect notification message to all connections using `api.WebSocketMessage` type (format: `{"type":"disconnect","reason":"execution_completed"}`)
   - Deletes all connection records from DynamoDB
   - Uses concurrent sending for performance

3. **Clients** (CLI or web viewer):
   - Receive disconnect notification message
   - Close WebSocket connection gracefully
   - Stop polling/log streaming

### Connection Lifecycle

**Connection Establishment**:
1. Client connects to WebSocket URL with `execution_id` query parameter
2. API Gateway routes `$connect` event to the event processor Lambda
3. Lambda stores connection record in DynamoDB
4. Connection ready for log streaming

**Log Streaming**:
1. Log streaming functionality will be reimplemented in the event processor
2. Clients receive log events in real-time via WebSocket

**Connection Termination**:
- **Manual disconnect**: Client closes connection → API Gateway routes `$disconnect` → Lambda removes connection record via the embedded WebSocket manager
- **Execution completion**: Event processor invokes `$disconnect-execution` → Lambda notifies clients and deletes records via the embedded WebSocket manager

### Error Handling

- **Connection failures**: Failed sends are logged but don't fail the Lambda handler
- **Missing connections**: If no connections exist for an execution, operations return successfully
- **Concurrent sending**: Uses error group with rate limiting to prevent overwhelming API Gateway
- **Best-effort notifications**: Disconnect notifications are best-effort; failures don't prevent execution completion

### Benefits

- ✅ **Real-time Streaming**: Logs appear instantly in clients without polling
- ✅ **Efficient**: Only forwards logs when clients are connected
- ✅ **Scalable**: Handles multiple concurrent connections per execution
- ✅ **Clean Termination**: Clients are notified when executions complete
- ✅ **Automatic Cleanup**: Connection records are cleaned up on execution completion

### CloudFormation Resources

- **`WebSocketApi`**: API Gateway WebSocket API
- **`WebSocketApiStage`**: WebSocket API stage
- **`WebSocketConnectRoute`**: `$connect` route
- **`WebSocketDisconnectRoute`**: `$disconnect` route
- **`WebSocketConnectionsTable`**: DynamoDB table for connection records

## Web Viewer Architecture

The platform includes a minimal web-based log viewer for visualizing execution logs in a browser. This provides an alternative to the CLI for teams who prefer a graphical interface.

### Implementation

**Location**: `cmd/webapp/dist/index.html`
**Deployment**: Hosted on Netlify at `https://runvoy.site/`
**Architecture**: Single-page application (SPA) - standalone HTML file with embedded CSS and JavaScript

### Technology Stack

- **Frontend**: Vanilla JavaScript + HTML5
- **Styling**: Pico.css v2 (CSS framework) - adopted in commit d04515b for improved UI and responsiveness
- **State Management**: Client-side (localStorage + JavaScript variables)
- **Polling**: 5-second intervals for logs and status updates
- **API Integration**: Fetch API for RESTful API calls

### Core Features

1. **Real-time Log Streaming**
   - Connects to WebSocket API for real-time log streaming
   - Also polls API every 5 seconds for new logs as fallback
   - Auto-scrolls to bottom on new logs
   - Handles rate limiting and backpressure
   - Receives disconnect notifications when execution completes

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

The web viewer interacts with multiple API endpoints:

**REST API Endpoints**:
- `GET /api/v1/executions/{id}/status` - Fetch execution status and metadata
- `GET /api/v1/executions/{id}/logs` - Fetch execution logs and WebSocket URL

**WebSocket API**:
- Connects to WebSocket URL returned in logs response
- Receives real-time log events via WebSocket
- Receives disconnect notification when execution completes

All endpoints require authentication via `X-API-Key` header.

### Access Pattern

Users access the web viewer via URL with execution ID as query parameter:

```
https://runvoy.site/?execution_id={executionID}
```

The CLI automatically provides this link when running commands (see `cmd/cli/cmd/logs.go:58-59`).

### Configuration

The web application URL is configurable and can be set via:

1. **Environment Variable**: `RUNVOY_WEB_URL`
2. **Config File**: `web_url` field in `~/.runvoy/config.yaml`

If not configured, it defaults to `https://runvoy.site/`.

**Default URL** is defined in `internal/constants/constants.go`:
```go
const DefaultWebURL = "https://runvoy.site/"
```

**Usage in CLI commands** (see `cmd/cli/cmd/run.go` and `cmd/cli/cmd/logs.go`):
```go
output.Infof("View logs in web viewer: %s?execution_id=%s", cfg.WebURL, executionID)
```

This allows users to deploy their own web viewer instance and configure the CLI to point to it.

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

- **CLI passthrough**: `just runvoy <args…>` rebuilds `cmd/cli` with version metadata and executes it, making it easy to test commands while always running a fresh binary.
- **Build outputs**: `just build` fans out to `just build-cli`, `just build-local`, `just build-orchestrator`, and `just build-event-processor`, ensuring all deployable binaries are rebuilt with consistent linker flags. Individual targets can be invoked when iterating on a single component.
- **Packaging & deploy**: `just build-orchestrator-zip` and `just build-event-processor-zip` stage Lambda-ready artifacts; `just deploy`, `just deploy-orchestrator`, `just deploy-event-processor`, and `just deploy-webviewer` push those artifacts (and the web viewer) to the release bucket and update Lambda code.
- **Local iteration**: `just run-local` launches the local HTTP server; `just local-dev-server` wraps it with `reflex` for hot reload. You can use the CLI for full API testing once the server is running.
- **Quality gates**: `just test`, `just test-coverage`, `just lint`, `just lint-fix`, `just fmt`, `just check`, and `just clean` provide the standard Go QA loop.
- **Environment & infra helpers**: `just dev-setup`, `just install-hook`, `just pre-commit-all` prepare developer machines, while `just create-lambda-bucket`, `just update-backend-infra`, `just seed-admin-user`, and `just destroy-backend-infra` manage AWS prerequisites.
- **DX utilities**: `just record-demo` regenerates the terminal cast/GIF assets.

**Environment Configuration:**

The `justfile` uses `set dotenv-required`, which means all `just` commands require a `.env` file to be present in the repository root. Developers should:

1. Copy `.env.example` to `.env`: `cp .env.example .env`
2. Edit `.env` with their actual environment variable values
3. Ensure `.env` is not committed to version control (already in `.gitignore`)

If `.env` is missing, `just` commands will fail with an error, ensuring developers configure their environment before running commands.

### Agent Integration

Automation-friendly targets remain the same: agents should prefer `just lint-fix`, `just fmt`, and `just check`. These commands ensure consistent code quality across contributors and automated systems.

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

3. **Custom Container Images** - ✅ **IMPLEMENTED** - The `image` field in execution requests is now supported:
   - Custom Docker images are supported via dynamic task definition registration
   - Images must be registered via `/api/v1/images/register` before use
   - The default image should be registered manually after deployment (see `just init` output for instructions)
   - Task definitions are created via ECS API when images are registered
   - Images are managed via `/api/v1/images` endpoints (admin-only)
   - Executions with unregistered images will fail with a clear error message

4. **Comprehensive Test Coverage** - Current test coverage is limited. Areas needing tests:
   - Event processor logic (status determination)
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
- ✅ **API Key Authentication** - SHA-256 hashed keys with last-used tracking
- ✅ **User Management** - Create and revoke users via CLI
- ✅ **Command Execution** - Remote command execution via ECS Fargate
- ✅ **Execution Records** - Complete tracking with status and duration
- ✅ **Error Handling** - Structured errors with proper HTTP status codes
- ✅ **Logging** - Request-scoped logging with AWS request ID
- ✅ **Local Development** - HTTP server for testing without AWS
- ✅ **Provider Abstraction** - Runner interface for multi-cloud support
- ✅ **Automated Admin Seeding** - Infra update step seeds an admin user into DynamoDB using the SHA-256 + base64 hash of the API key from the global CLI config (idempotent via conditional write). Requires `RUNVOY_ADMIN_EMAIL` to be set during deployment.
- ✅ **Web Viewer** - Browser-based log viewer with real-time streaming, ANSI color support, and responsive UI (Pico.css)

## CLI Client Architecture

The CLI client (`cmd/cli`) provides command-line access to the runvoy platform.

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

#### Client Interface for Testing

The `internal/client/client.go` package defines an `Interface` type (`internal/client/interface.go`) that abstracts all client operations. This enables dependency injection and makes CLI commands fully testable with mocks:

- **Location**: `internal/client/interface.go`
- **Implementation**: The `Client` struct automatically implements `Interface` via compile-time check
- **Benefits**: Commands can use `client.Interface` type instead of concrete `*client.Client` for dependency injection

**Usage in Commands:**
```go
// In command handler
var c client.Interface = client.New(cfg, slog.Default())

// In service (for testing)
type StatusService struct {
    client client.Interface  // Can be mocked in tests
    output OutputInterface
}
```

#### CLI Command Testing Architecture

All CLI commands have been refactored to use a **service pattern** with dependency injection for testability. This separates business logic from cobra command handlers.

**Design Pattern:**

```12:50:cmd/cli/cmd/status.go
func statusRun(cmd *cobra.Command, args []string) {
	executionID := args[0]
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	c := client.New(cfg, slog.Default())
	service := NewStatusService(c, NewOutputWrapper())
	if err := service.DisplayStatus(cmd.Context(), executionID); err != nil {
		output.Errorf(err.Error())
	}
}

// StatusService handles status display logic
type StatusService struct {
	client client.Interface
	output OutputInterface
}

// NewStatusService creates a new StatusService with the provided dependencies
func NewStatusService(client client.Interface, output OutputInterface) *StatusService {
	return &StatusService{
		client: client,
		output: output,
	}
}

// DisplayStatus retrieves and displays the status of an execution
func (s *StatusService) DisplayStatus(ctx context.Context, executionID string) error {
	status, err := s.client.GetExecutionStatus(ctx, executionID)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}
```

**Refactored Commands:**

All CLI commands follow the same service pattern:

1. **ClaimCommand** (`claim.go`):
   - `ClaimService` - Handles API key claiming logic
   - Includes `ConfigSaver` interface for testable config saving
   - Full test coverage with error scenarios

2. **KillCommand** (`kill.go`):
   - `KillService` - Handles execution termination logic
   - Tests cover success, not found, and network error cases

3. **ListCommand** (`list.go`):
   - `ListService` - Handles execution listing and table formatting
   - Tests cover empty lists, formatting, and error handling

4. **ConfigureCommand** (`configure.go`):
   - `ConfigureService` - Handles interactive configuration flow
   - Includes `ConfigLoader` and `ConfigPathGetter` interfaces
   - Supports mocking of prompts for testing
   - Tests cover new config creation, updates, and error scenarios

5. **UsersCommand** (`users.go`):
   - `UsersService` - Handles user management (create, list, revoke)
   - Includes `formatUsers` helper for table formatting
   - Comprehensive tests for all subcommands

6. **ImagesCommand** (`images.go`):
   - `ImagesService` - Handles image management (register, list, unregister)
   - Includes `formatImages` helper for table formatting
   - Tests cover all image operations

**Key Components:**

1. **Output Interface** (`cmd/cli/cmd/output_interface.go`):
   - Defines `OutputInterface` for all output operations including `Prompt()` for interactive commands
   - Provides `outputWrapper` that implements the interface using global `output.*` functions
   - Enables capturing and verifying output in tests

2. **Service Pattern**:
   - Each command has an associated service (e.g., `StatusService`, `ClaimService`, `UsersService`)
   - Services contain business logic separated from cobra integration
   - Services accept dependencies via constructor injection

3. **Configuration Interfaces**:
   - `ConfigSaver` - Interface for saving configuration (used by `claim.go`)
   - `ConfigLoader` - Interface for loading configuration (used by `configure.go`)
   - `ConfigPathGetter` - Interface for getting config path (used by `configure.go`)

4. **Manual Mocking**:
   - Tests use manual mocks that implement `client.Interface` and `OutputInterface`
   - Mocks are simple structs with function fields for test-specific behavior
   - No external mocking framework required

**Testing Example:**

```go
func TestStatusService_DisplayStatus(t *testing.T) {
    mockClient := &mockClientInterface{}
    mockClient.getExecutionStatusFunc = func(ctx context.Context, executionID string) (*api.ExecutionStatusResponse, error) {
        return &api.ExecutionStatusResponse{
            ExecutionID: "exec-123",
            Status:      "running",
        }, nil
    }
    
    mockOutput := &mockOutputInterface{}
    service := NewStatusService(mockClient, mockOutput)
    
    err := service.DisplayStatus(context.Background(), "exec-123")
    assert.NoError(t, err)
    assert.True(t, len(mockOutput.calls) > 0)
}
```

**Refactored Commands:**
- ✅ `status.go` - Refactored with `StatusService` and full test coverage
- ✅ `logs.go` - Refactored with `LogsService` and full test coverage
- ✅ `claim.go` - Refactored with `ClaimService` and full test coverage
- ✅ `kill.go` - Refactored with `KillService` and full test coverage
- ✅ `list.go` - Refactored with `ListService` and full test coverage
- ✅ `configure.go` - Refactored with `ConfigureService` and full test coverage
- ✅ `users.go` - Refactored with `UsersService` and full test coverage
- ✅ `images.go` - Refactored with `ImagesService` and full test coverage

All commands now follow the same service pattern with dependency injection for testability.

**Benefits:**
- **100% Backward Compatible**: CLI behavior unchanged, only internal structure refactored
- **Testable**: Business logic can be tested with mocks without HTTP calls
- **Maintainable**: Clear separation of concerns (cobra vs business logic)
- **Extensible**: Easy to add new commands following the same pattern

#### User Management Commands

##### Create User (`users create <email>`)

The command uses the generic HTTP client through the user client:

**Implementation:**
- Located in `cmd/cli/cmd/addUser.go`
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

**Implementation:** `cmd/cli/cmd/run.go`

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
   - Starts ECS Fargate task with the command and sidecar
   - Returns execution ID

**Response (202 Accepted):**
```json
{
  "execution_id": "abc123...",
  "log_url": "/api/v1/executions/abc123/logs/notimplemented",
  "status": "RUNNING"
}
```

**sidecar Architecture:**
- Tasks use dynamically registered ECS task definitions (one per Docker image) with a sidecar container
- The sidecar command is dynamically generated in `internal/app/aws/runner.go` via `buildSidecarContainerCommand()`
- The sidecar handles auxiliary tasks (git cloning, .env file generation, etc.)
- If no git repository is specified, the sidecar simply exits successfully (exit 0)
- Task definitions are registered on-demand via ECS API when images are added
- The sidecar is named generically ("sidecar") for future extensibility

**Note:** Execution completion is tracked automatically by the event processor Lambda.

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

## ECS Task Architecture

The platform uses dynamically managed ECS Fargate task definitions with a sidecar pattern. Task definitions are registered on-demand via the API when images are added, eliminating the need for CloudFormation-managed task definitions.

### Task Definition Management

**Task Definition Naming and Storage:**
- Task definitions follow the naming pattern `runvoy-image-{sanitized-image-name}`
  - Example: Image `hashicorp/terraform:1.6` creates task definition family `runvoy-image-hashicorp-terraform-1-6`
  - Image names are sanitized to comply with ECS task definition family name requirements (alphanumeric, hyphens, underscores only)
- **Docker image storage**: The actual Docker image is stored in the task definition container definition (runner container's `Image` field)
  - This is the source of truth - the exact image name used at runtime
  - Images are extracted from container definitions when listing (reliable, no lossy reconstruction)
  - Task definitions are also tagged with `DockerImage=<full-image-name>` for metadata, though container definitions are primary
- **Default image marking**: The default image is marked with tag `IsDefault=true` on the task definition resource
  - When listing images, the `is_default` field indicates which image is the default
  - **Single default enforcement**: Only one image can be marked as default at a time
    - When registering a new image as default, any existing default tags are automatically removed
    - This prevents multiple images from being tagged as default simultaneously

**Dynamic Registration:**
- Task definitions are registered via the ECS API when images are added through the `/api/v1/images/register` endpoint
- The default image must be registered manually after deployment using `just init` or via the API with the `--set-default` flag
- When an execution requests an image, the system looks up the existing task definition
- If an image is not registered, the execution will fail with an error directing the admin to register it first
- Task definitions are reused across executions using the same image
- The ECS task definition API is the source of truth - no caching, queries happen on-demand

**Task Definition Structure:**
Each task definition follows a consistent structure:

- **Sidecar Container ("sidecar")**: Handles auxiliary tasks before main execution
  - Essential: `false` (task continues after sidecar completes)
  - Image: Alpine Linux (lightweight, includes git)
  - Command is dynamically generated by `internal/app/aws/runner.go` via `buildSidecarContainerCommand()`
  - Creates `.env` file from user environment variables (variables prefixed with `RUNVOY_USER_`)
  - Conditionally clones git repository if `GIT_REPO` environment variable is set
  - If git repo is cloned and `.env` exists, copies `.env` to the repo directory
  - If no git repo specified, exits successfully (exit 0)
  - Logs prefixed with "### runvoy sidecar:"
  - Future extensibility: credential fetching, etc.

- **Main Container ("runner")**: Executes user commands
  - Essential: `true` (task fails if this container fails)
  - Image: Specified by user
  - Depends on sidecar completing successfully
  - Working directory: `/workspace` (or `/workspace/repo` if git used)
  - Command overridden at runtime via Lambda
  - Logs prefixed with "### runvoy:"

**Shared Volume:**
- Both containers mount `/workspace` volume
- Sidecar creates `.env` file at `/workspace/.env` (if user env vars provided)
- Sidecar clones git repo to `/workspace/repo` (if specified) and copies `.env` to repo directory
- Main container accesses cloned repo and reads `.env` files created by sidecar

**Benefits of Dynamic Task Definition Approach:**
- ✅ Support for multiple Docker images without CloudFormation changes
- ✅ Self-service image registration via API
- ✅ No infrastructure drift - task definitions managed programmatically
- ✅ Flexible - users can request any registered image
- ✅ Consistent execution model across all images
- ✅ Easy cleanup - task definitions can be deregistered via API

## Database Schema

The platform uses DynamoDB tables for data persistence. All tables are defined in the CloudFormation template (`deployments/cloudformation-backend.yaml`).

### API Keys Table (`{project-name}-api-keys`)

Stores user records with hashed API keys.

**Primary Key:**
- `api_key_hash` (String) - Hash of the API key (SHA-256, base64 encoded)

**Global Secondary Index:**
- `user_email-index` - Lookup users by email

**Attributes:**
- `api_key_hash` (String) - Primary key, SHA-256 hash of the API key
- `user_email` (String) - User's email address
- `created_at` (String) - ISO 8601 timestamp when user was created
- `last_used` (String, optional) - ISO 8601 timestamp of last authentication
- `revoked` (Boolean) - Whether the API key has been revoked
- `expires_at` (Number, optional) - Unix timestamp for TTL (unclaimed users)

**TTL:**
- Enabled on `expires_at` attribute
- Unclaimed users are automatically deleted after 15 minutes
- TTL is removed when user claims their API key

**Code Reference:** `internal/database/dynamodb/users.go`

### Executions Table (`{project-name}-executions`)

Stores execution records for all command runs.

**Primary Key:**
- `execution_id` (String) - Unique execution identifier
- `started_at` (String) - ISO 8601 timestamp (range key)

**Global Secondary Indexes:**
- `user_email-started_at` - Lookup executions by user
- `status-started_at` - Lookup executions by status

**Attributes:**
- `execution_id` (String) - Primary key, unique execution identifier
- `started_at` (String) - Range key, ISO 8601 timestamp
- `user_email` (String) - Email of the user who ran the execution
- `command` (String) - The command that was executed
- `lock_name` (String, optional) - Lock name if specified
- `completed_at` (String, optional) - ISO 8601 timestamp when execution completed
- `status` (String) - Current status (RUNNING, SUCCEEDED, FAILED, STOPPED)
- `exit_code` (Number) - Exit code from the command
- `duration_seconds` (Number, optional) - Execution duration in seconds
- `log_stream_name` (String, optional) - CloudWatch Logs stream name
- `cost_usd` (Number, optional) - Estimated cost in USD
- `request_id` (String, optional) - AWS Lambda request ID for tracing
- `cloud` (String, optional) - Compute platform (AWS, etc.)

**Code Reference:** `internal/database/dynamodb/executions.go`

### Pending API Keys Table (`{project-name}-pending-api-keys`)

Stores pending API key claims for secure distribution.

**Primary Key:**
- `secret_token` (String) - Cryptographically random token

**Attributes:**
- `secret_token` (String) - Primary key, base64url encoded secret token
- `api_key` (String) - Plain API key (encrypted at rest via SSE)
- `user_email` (String) - Email of the user this key belongs to
- `created_by` (String) - Email of the admin who created the user
- `created_at` (Number) - Unix timestamp when record was created
- `expires_at` (Number) - Unix timestamp for TTL
- `viewed` (Boolean) - Whether the key has been claimed
- `viewed_at` (Number, optional) - Unix timestamp when claimed
- `viewed_from_ip` (String, optional) - IP address of the user who claimed it

**TTL:**
- Enabled on `expires_at` attribute
- Pending keys are automatically deleted after 15 minutes

**Code Reference:** `internal/database/dynamodb/pending_keys.go`

## Task Definition Cleanup

**Important:** Task definitions are now managed dynamically via the ECS API and are no longer managed by CloudFormation. When destroying the infrastructure stack, task definitions are **not automatically cleaned up** by CloudFormation.

**Manual Cleanup Required:**
Before destroying the CloudFormation stack, admins should:

1. List all registered images: `GET /api/v1/images`
2. Remove all registered images: `DELETE /api/v1/images/{image}` for each image
3. Alternatively, manually deregister task definitions via AWS CLI or console

Task definitions with family prefix `runvoy-image-*` should be deregistered to avoid orphaned resources.

**Future Enhancement:** Automated cleanup could be added via a CloudFormation custom resource or destroy-time script. See GitHub issue for tracking.

## IAM Permissions Security

The orchestrator Lambda has scoped IAM permissions for ECS resources:

- **Task Definitions**: Permissions are scoped to `arn:aws:ecs:*:*:task-definition/runvoy-image-*`
  - This restricts operations to only runvoy-managed task definitions
  - `RegisterTaskDefinition` cannot be resource-scoped (AWS limitation), but we control family names via code
  - `ListTaskDefinitions` cannot be resource-scoped (AWS limitation), but we filter results by family prefix

- **Tasks**: Permissions are scoped to the runvoy cluster ARN
  - All task operations (RunTask, StopTask, DescribeTasks, ListTasks) are limited to tasks in our cluster
  - This prevents operations on tasks in other ECS clusters

This ensures the Lambda can only manage runvoy resources and cannot affect other ECS resources in the account.
