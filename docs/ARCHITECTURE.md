# runvoy Architecture

## Overview

runvoy is a centralized execution platform that allows teams to run infrastructure commands without sharing credentials. An admin deploys runvoy once to the company's cloud provider account, then issues API keys to team members who can execute commands safely with full audit trails.

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
├── deploy/
├── docs/
├── internal/
├── scripts/
```

- `bin/`: built binaries for the runvoy application (temporary storage for building artifacts during development).
- `cmd/`: main entry points for the various application (CLI, local dev server, provider-specific lambdas, etc.)
- `deploy/`: infrastructure as code grouped by provider (CloudFormation templates, etc.).
- `docs/`: project documentation (architecture, testing strategy, etc.).
- `internal/`: core logic of the runvoy application (business logic, API, database, etc.)
- `scripts/`: scripts for the runvoy application development and deployment

## Services

- Orchestrator: Handles API requests and orchestrates task executions.
- Event Processor: Handles asynchronous events from cloud services (ECS task completions, CloudWatch logs, WebSocket notifications, etc.).

## Execution Provider Abstraction

To support multiple cloud platforms, the service layer now depends on an execution provider interface:

```text
internal/app.Service           → uses Runner interface (provider-agnostic)
internal/providers/aws/app     → AWS-specific Runner implementation (ECS Fargate)
```

- The `Runner` interface abstracts starting a command execution and returns a stable execution ID and the task creation timestamp.
- The AWS implementation resides in `internal/providers/aws/app` and encapsulates all ECS- and AWS-specific logic and types.
- `internal/app/init.go` wires the chosen provider by constructing the appropriate `Runner` and passing it into `Service`.

## Router Architecture

The application uses **chi** (github.com/go-chi/chi/v5) as the HTTP router for both Lambda and local HTTP server orchestrator implementations. This provides a consistent routing API across deployment models.

### Components

- **`internal/server/router.go`**: Shared chi-based router configuration with all API routes
- **`internal/server/middleware.go`**: Middleware for request ID extraction and logging context
- **`internal/providers/aws/lambdaapi/handler.go`**: Lambda handler that uses algnhsa to adapt the chi router
- **`cmd/local/main.go`**: Local HTTP server implementation using the same router
- **`cmd/backend/providers/aws/orchestrator/main.go`**: Lambda entry point for the orchestrator (uses the chi-based handler)

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
GET    /api/v1/executions/{id}/logs     - Fetch execution logs (CloudWatch + API GW powered websocket)
GET    /api/v1/executions/{id}/status   - Get execution status (RUNNING/SUCCEEDED/FAILED/STOPPED)
POST   /api/v1/executions/{id}/kill     - Terminate a running execution
GET    /api/v1/claim/{token}            - Claim a pending API key (public, no auth required)
```

Both Lambda and local HTTP server use identical routing logic, ensuring development/production parity.

### Lambda Event Adapter

The platform uses **algnhsa** (`github.com/akrylysov/algnhsa`), a well-maintained open-source library that adapts standard Go `http.Handler` implementations (like chi routers) to work with AWS Lambda. This eliminates the need for custom adapter code and provides robust support for multiple Lambda event types.

**Implementation:** `internal/providers/aws/lambdaapi/handler.go` creates the Lambda handler by wrapping the chi router with `algnhsa.New()`:

```go
func NewHandler(svc *app.Service, requestTimeout time.Duration) lambda.Handler {
    router := server.NewRouter(svc, requestTimeout)
    return algnhsa.New(router.Handler(), nil)
}
```

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
- The `request_id` is persisted in DynamoDB via the `internal/providers/aws/database/dynamodb` repository.
- If a request ID is not present (e.g., non-Lambda environments), the service logs a warning and stores the execution without a `request_id`.
- The `compute_platform` field in execution records is derived from the configured backend provider at initialization time (e.g., `AWS`) rather than being hardcoded in the service logic.
- The backend provider is selected via the `RUNVOY_BACKEND_PROVIDER` configuration value (default: `AWS`); provider-specific bootstrapping logic determines which runner and repositories are wired in.

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

> **Note:** For brevity the diagram omits the API Gateway WebSocket path. In production, CloudWatch Logs invoke the event processor, which relays batched log events to CLI and web viewer clients over the WebSocket API.

## Event Processor Architecture

The platform uses a dedicated **event processor Lambda** to handle asynchronous events from AWS services. This provides a clean separation between synchronous API requests (handled by the orchestrator) and asynchronous event processing.

### Design Pattern

- **Orchestrator Lambda**: Handles synchronous HTTP API requests
- **Event Processor Lambda**: Handles asynchronous workloads from EventBridge, CloudWatch Logs subscriptions, and API Gateway WebSocket lifecycle events
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
8. **CloudWatch Logs Streaming**: CloudWatch Logs subscription events deliver batched runner log entries; the processor converts them to `api.LogEvent` records and pushes each entry to active WebSocket connections
9. **WebSocket Lifecycle**: `$connect` and `$disconnect` routes from API Gateway are handled in-process to authenticate clients, persist connection metadata, and fan out disconnect messages

### Event Types

Currently handles:
- **ECS Task State Change**: Updates execution records when tasks complete
- **CloudWatch Logs Subscription**: Streams runner container logs to connected clients in real time
- **API Gateway WebSocket Events**: Manages `$connect` and `$disconnect` routes

Designed to be extended for future event types:
- CloudWatch Alarms
- S3 events
- Custom application events

### Implementation

**Entry Point**: `cmd/backend/providers/aws/event_processor/main.go`
- Initializes event processor
- Starts Lambda handler

**Event Routing**: `internal/providers/aws/events/backend.go`
- Routes events by `detail-type`
- Ignores unknown event types (log and continue)
- Extensible switch statement for new handlers

**ECS Handler**: `internal/providers/aws/events/backend.go`
- Parses ECS task events
- Extracts execution ID from task ARN
- Determines final status from exit code and stop reason
- Handles missing `startedAt` timestamps: When ECS task events have an empty `startedAt` field (e.g., when containers fail before starting, such as sidecar git puller failures), falls back to the execution's `StartedAt` timestamp that was set at creation time
- Calculates duration (with safeguards for negative durations)
- Updates DynamoDB execution record
- Signals WebSocket termination: When execution reaches a terminal status (SUCCEEDED, FAILED, STOPPED), calls `NotifyExecutionCompletion()` which sends disconnect notifications to all connected clients and cleans up connection records

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
   - `STARTING`: Command accepted and infrastructure provisioning in progress
   - `RUNNING`: Command is currently executing
   - `SUCCEEDED`: Command completed successfully (exit code 0)
   - `FAILED`: Command failed with an error (non-zero exit code)
   - `STOPPED`: Command was manually terminated by user
   - `TERMINATING`: Stop requested, waiting for task to fully stop

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
- **`EventProcessorLogsPermission`**: Allows CloudWatch Logs to invoke the event processor
- **`RunnerLogsSubscription`**: Subscribes ECS runner logs (filtered to the `runner` container streams) to the event processor for real-time processing
- **`SecretsMetadataTable`**: DynamoDB table tracking metadata for managed secrets (name, description, env var binding, audit timestamps)
- **`SecretsKmsKey`**: KMS key dedicated to encrypting secret payloads stored as SecureString parameters
- **`SecretsKmsKeyAlias`**: Friendly alias pointing to the secrets KMS key for CLI and configuration usage

## Secrets Management

Runvoy includes a first-party secrets store so admins can centralize credentials that executions or operators need. Secret payloads never live in the database; they are written to AWS Systems Manager Parameter Store as SecureString parameters encrypted with a dedicated KMS key, while descriptive metadata is persisted separately in DynamoDB for fast queries and auditing.

### Components

- **Metadata repository (`SecretsMetadataTable`)**: DynamoDB table keyed by `secret_name`. Stores the environment variable binding (`key_name`), description, and audit fields (`created_by`, `created_at`, `updated_by`, `updated_at`). Conditional writes prevent accidental overwrites.
- **Value store (Parameter Store)**: Secrets are persisted under the configurable prefix (default `/runvoy/secrets/{name}`) using SecureString entries encrypted with `SecretsKmsKey`. Every rotation creates a new Parameter Store version while the CLI/API always surfaces the latest value.
- **Dedicated KMS key**: CloudFormation provisions a scoped CMK and alias for secrets. Lambda execution roles have permission to encrypt/decrypt with this key only for the configured prefix.

### API and CLI Workflow

All secrets endpoints require authentication via the standard `X-API-Key` header.

1. **Create (`POST /api/v1/secrets`)**: The orchestrator stores the value in Parameter Store first and then records metadata in DynamoDB. On metadata failure, it performs best-effort cleanup of the value to avoid orphans.
2. **Read (`GET /api/v1/secrets/{name}`)**: Returns metadata plus the decrypted value. Missing Parameter Store values are logged and surfaced as metadata-only responses.
3. **List (`GET /api/v1/secrets`)**: Scans the metadata table, then hydrates values from Parameter Store in best effort fashion. The CLI formats the result as a table without echoing secret payloads.
4. **Update (`PUT /api/v1/secrets/{name}`)**: Rotates the value (if provided) by overwriting the Parameter Store entry and refreshes metadata, including the optional `key_name` change.
5. **Delete (`DELETE /api/v1/secrets/{name}`)**: Removes the SecureString entry and then deletes the metadata record. Missing payloads are tolerated so cleanup is idempotent.

### Operational Characteristics

- **Auditability**: Metadata captures who created or updated a secret and when. CLI output highlights those fields so teams can spot stale or orphaned credentials.
- **Prefix isolation**: The orchestrator only writes inside the configured prefix, ensuring teams can segregate secrets per environment (e.g., `/runvoy/prod/secrets` vs `/runvoy/dev/secrets`).
- **Error handling**: Parameter Store failures surface as `500` errors. DynamoDB conditional failures map to conflict/not-found errors so the CLI can react appropriately.
- **Infrastructure integration**: `just init` provisions the metadata table, KMS key, and IAM policies. Environment variables (`RUNVOY_AWS_SECRETS_PREFIX`, `RUNVOY_AWS_SECRETS_KMS_KEY_ARN`, `RUNVOY_AWS_SECRETS_METADATA_TABLE`) keep the orchestrator configuration explicit across environments.

## WebSocket Architecture

The platform uses WebSocket connections for real-time log streaming to clients (CLI and web viewer). The architecture consists of two main components: the event processor Lambda (reusing the WebSocket manager package) and the API Gateway WebSocket API.

### Design Pattern

- **Event Processor Lambda**: Handles WebSocket connection lifecycle events, streams CloudWatch log batches, and issues disconnect notifications via the WebSocket manager package
- **CloudWatch Logs subscription**: Invokes the event processor whenever runner container logs are produced, allowing it to push log events to all active connections
- **API Gateway WebSocket API**: Provides WebSocket endpoints for client connections

### Components

#### WebSocket Manager Package

**Purpose**: Manages WebSocket connection lifecycle and sends disconnect notifications when executions complete. It is embedded inside the event processor Lambda rather than deployed as a separate Lambda function.

**Implementation**: `internal/app/websocket/manager.go` (interface), `internal/providers/aws/websocket/manager.go` (AWS implementation)
- **`HandleRequest(ctx, rawEvent, logger)`**: Adapts raw Lambda events for the generic processor, routes WebSocket events by route key, and reports whether the event was handled

**Route Keys Handled**:

1. **`$connect`** (`handleConnect`):
   - Stores WebSocket connection in DynamoDB with execution ID
   - Sets TTL for connection record (24 hours by default)
   - Returns success response

2. **`$disconnect`** (`handleDisconnect`):
   - Removes WebSocket connection from DynamoDB when client disconnects
   - Cleans up connection record

**Connection Management**:
- Connections stored in DynamoDB `{project-name}-websocket-connections` table
- Each connection record includes: `connection_id`, `execution_id`, `functionality`, `expires_at`
- Connections are queried by execution ID for log forwarding

#### API Gateway WebSocket API

**Configuration**: Defined in CloudFormation template (`deploy/providers/aws/cloudformation-backend.yaml`)

**Routes**:

- **`$connect`**: Routes to the event processor Lambda
- **`$disconnect`**: Routes to the event processor Lambda

**Integration**:

- Uses AWS_PROXY integration type
- Both routes integrate with the event processor Lambda
- API Gateway Management API used for sending messages to connections

### Execution Completion Flow

When an execution reaches a terminal status (SUCCEEDED, FAILED, STOPPED):

1. **Event Processor** (`internal/providers/aws/events/backend.go`):
   - Updates execution record in DynamoDB with final status
   - Calls `NotifyExecutionCompletion()`
   - Queries DynamoDB for all connections for the execution ID
   - Sends disconnect notification message to all connections using `api.WebSocketMessage` type (format: `{"type":"disconnect","reason":"execution_completed"}`)
   - Deletes all connection records from DynamoDB
   - Uses concurrent sending for performance

2. **Clients** (CLI or web viewer):
   - Receive disconnect notification message
   - Close WebSocket connection gracefully
   - Stop polling/log streaming

### Connection Lifecycle

**Connection Establishment**:
1. CLI or web viewer calls `GET /api/v1/executions/{id}/logs`; the service creates a reusable WebSocket token and returns a `wss://` URL containing `execution_id` and `token` query parameters
2. Client connects to the returned WebSocket URL
3. API Gateway routes the `$connect` event to the event processor Lambda
4. Lambda validates the token (which persists for reuse), stores the connection record in DynamoDB, and the connection becomes ready for streaming

**Token Reusability**:
- Tokens are **reusable** and persist until expiration via TTL (`constants.ConnectionTTLHours`)
- If a client's WebSocket connection drops, it can reconnect using the same token without calling `/logs` again
- This enables seamless reconnection for transient network issues
- Once the token expires (24 hours by default), clients must call `/logs` again to get a new token

**Log Streaming**:
1. CloudWatch Logs invokes the event processor with batched runner log events
2. The event processor transforms each entry into an `api.LogEvent` and sends it to every active WebSocket connection for that execution in real time

**Connection Termination**:
- **Manual disconnect**: Client closes connection → API Gateway routes `$disconnect` → Lambda removes connection record via the embedded WebSocket manager
- **Execution completion**: Event processor calls `NotifyExecutionCompletion()` → Lambda notifies clients and deletes records via the embedded WebSocket manager
- **Token expiration**: After TTL expires, pending token is automatically deleted; client must call `/logs` to reconnect

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

**`/logs` Response Contract**:
The `/logs` endpoint returns:
- `execution_id`: The execution identifier
- `status`: Current execution status (RUNNING, SUCCEEDED, FAILED, STOPPED)
- `events`: Array of completed log entries
- `websocket_url`: URL for real-time log streaming (only present if available)

Clients should check the `status` field to determine behavior:
- **RUNNING**: WebSocket URL will be present; client should connect to stream new logs in real-time
- **SUCCEEDED/FAILED/STOPPED**: Execution has completed; all logs are in the `events` array; client should not attempt WebSocket connection

**WebSocket API**:
- Connects to WebSocket URL returned in logs response
- Receives real-time log events via WebSocket (only for RUNNING executions)
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

**Database Layer (`internal/providers/aws/database/dynamodb`):**
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

### Log Viewing and Tailing - MVP (Best-Effort Streaming)

The service exposes a logs endpoint that aggregates CloudWatch Logs events for a given execution ID (ECS task ID). Each response includes the full history of log events (`timestamp` in milliseconds and raw `message`) and, when the execution is still running, a temporary WebSocket URL for live streaming. The CLI implements real-time tailing on top of this endpoint:

- Auth required via `X-API-Key`
- Returns all available events across discovered streams containing the task ID
- Reads from deterministic stream: `task/<container-name>/<executionID>` (container: `executor`)
- Response is sorted by timestamp ascending
- For RUNNING executions, `websocket_url` contains a one-time `wss://` endpoint with an embedded token used to establish an authenticated WebSocket connection

Error behavior:

- 500 Internal Server Error when the configured CloudWatch Logs group does not exist (or other AWS errors)
- 503 Service Unavailable when the expected log stream for the execution ID does not exist yet (clients may retry later)

Example response:

```json
{
  "execution_id": "abc123",
  "status": "RUNNING",
  "events": [
    { "timestamp": 1730250000000, "message": "Execution starting" },
    { "timestamp": 1730250005000, "message": "..." }
  ],
  "websocket_url": "wss://abc123.execute-api.us-east-1.amazonaws.com/production?execution_id=abc123&token=..."
}
```

Environment variables:

```text
RUNVOY_AWS_LOG_GROUP       # required (e.g. /aws/ecs/runvoy)
```

#### Streaming Architecture - Mixed Approach (REST + WebSocket Best-Effort)

For MVP, the streaming log feature uses a **mixed approach**: REST API provides authoritative complete logs, while WebSocket offers best-effort real-time overlay.

**REST API (`/logs` endpoint) - Authoritative Backlog:**
- Always returns complete historical log events from CloudWatch Logs
- Guaranteed consistency (all logs written to CloudWatch are returned)
- Always provides a WebSocket URL regardless of execution status (for late-joining clients)
- Clients can use this to catch up on logs missed from the real-time stream

**WebSocket Real-Time Streaming - Best-Effort Overlay:**
- Best-effort delivery: logs may be dropped if connections fail or buffers overflow
- Complements the REST API with real-time updates (reduces polling overhead)
- Available for both RUNNING and completed executions (late-joiners can connect to receive disconnect notification)
- Failures are logged but do not fail the event processor (see `internal/providers/aws/events/backend.go`)

**Client Behavior (CLI `runvoy logs <executionID>`):**
1. Fetches entire log history from `/logs` endpoint with retry logic (handles 503 while execution starts)
2. Displays historical logs with computed line numbers (client-side) - **authoritative backlog**
3. Simultaneously connects to WebSocket URL for real-time streaming
4. New logs received via WebSocket are displayed without line number recomputation
5. If WebSocket disconnects (gracefully or due to error), client gracefully exits and prints web viewer URL
6. Falls back to printing the web viewer URL if WebSocket unavailable
7. **Important**: REST `/logs` endpoint is authoritative; WebSocket is lossy best-effort for real-time overlay

**Web Viewer:**
- Gets complete log backlog from REST `/logs` endpoint (authoritative)
- Connects to WebSocket for real-time updates and disconnect notifications
- Polls REST endpoint every 5 seconds as fallback if WebSocket unavailable or for catch-up
- Backlog always accessible regardless of WebSocket state

**Event Processor (`internal/providers/aws/events/backend.go`):**
- Receives CloudWatch Logs batches and forwards to WebSocket connections (best-effort)
- Connection failures are logged but do not fail processing
- Sends disconnect notifications to all connected clients when execution completes
- Execution completion is tracked reliably via REST endpoint, not WebSocket

#### Why Mixed Approach with Best-Effort Streaming for MVP

1. **Complete Backlog Access**: REST API always provides authoritative complete logs, so late-joining clients get the full picture
2. **Real-time UX Improvement**: WebSocket overlay provides instant updates without polling, improving responsiveness
3. **Resilient Architecture**: If WebSocket fails, REST API ensures no logs are lost; clients can always catch up
4. **Simple Implementation**: Avoids complex state management (sequences, retries, etc.) while providing complete logs
5. **Cost-Efficient**: WebSocket is best-effort, reducing server load from polling without complex delivery guarantees
6. **Sufficient for All Clients**: Terminal output is inherently lossy (small gaps acceptable); web viewer can poll if needed

#### Future Enhancement: Lossless Streaming

A future phase may implement reliable delivery with:
- Sequence numbers on log events for deduplication
- Client-side acking of received logs
- Retransmission of missed events
- Persistent message queue (SQS/Kinesis) for buffering
- Enhanced monitoring and alerting for streaming health

Recommended approach: Client-driven backfill using REST endpoint rather than server-side retransmission, keeping the architecture simple and scalable.

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

4. **Image CPU and Memory Parameters** - ✅ **IMPLEMENTED** - Custom CPU and memory allocation for images:
   - Admins can specify CPU and memory when registering images via `/api/v1/images/register`
   - Parameters are optional with sensible defaults: CPU=256, Memory=512
   - Runtime platform is customizable (e.g., Linux/ARM64, Linux/X86_64), defaults to Linux/ARM64
   - Request structure:
     ```json
     {
       "image": "ubuntu:22.04",
       "cpu": 512,
       "memory": 1024,
       "runtime_platform": "Linux/X86_64",
       "task_role_name": "optional-role",
       "task_execution_role_name": "optional-exec-role",
       "is_default": true
     }
     ```
   - CLI flags for image registration:
     - `--cpu`: Set CPU value (e.g., 256, 512, 1024, 2048, 4096)
     - `--memory`: Set memory value (e.g., 512, 1024, 2048, 4096, 8192)
     - `--runtime-platform`: Set runtime platform (e.g., Linux/ARM64, Linux/X86_64)
   - Image metadata includes CPU, memory, and runtime platform for querying registered images
   - Task definitions are registered with the specified CPU/memory allocation in ECS
   - Allows cost optimization by assigning appropriate resources per workload type

5. **Comprehensive Test Coverage** - Overall project coverage: **56.4%** (2774/4917 statements)

   **Latest Session Improvements (November 2025):**
   - Created 61 new test cases across app service layer
   - App service layer: 55.7% → 87.6% (+31.9 percentage points)
   - DynamoDB package: 49.2% → 68.1% (+18.9 percentage points)
   - Overall improvement: 54.6% → 56.4% (+1.8 percentage points)

   **Well-covered packages (>80%):**
   - `internal/logger`: 98.6%
   - `internal/errors`: 94.4%
   - `internal/config/aws`: 97.8%
   - `internal/app`: 87.6% ⬆️ (from 55.7%)
   - `internal/constants`: 87.5%
   - `internal/client/playbooks`: 87.3%
   - `internal/auth`: 78.6%
   - `internal/client`: 78.6%
   - `internal/config`: 78.4%

   **Areas with good coverage (70-80%):**
   - `internal/app/events`: 80.0%
   - `internal/client/output`: 75.3%
   - `internal/providers/aws/events`: 75.0%
   - `internal/providers/aws/secrets`: 90.6%
   - `internal/providers/aws/websocket`: 68.2%
   - `internal/providers/aws/database/dynamodb`: 68.1% ⬆️ (from 49.2%)

   **Areas needing improvement (<70%):**
   - AWS app integration (task definitions, images) - 42.4%
   - Server/HTTP handlers - 72.9% (good coverage, can be improved further)
   - CLI command implementations - mostly 0% (not in priority for backend)
   - Event processor/orchestrator main functions - 0% (entry points only)

   **Next High-Impact Priorities for Coverage Extension:**
   1. **AWS App Image Registration** (`internal/providers/aws/app/images*.go`)
      - `handleExistingImage`, `registerNewImage` - 0% coverage
      - `ImageParser` methods - Some have 0% coverage
      - Estimated impact: +5-10 percentage points to overall coverage

   2. **DynamoDB User Repository** (`internal/providers/aws/database/dynamodb/users.go`)
      - GetUserByAPIKeyHash, GetUserByEmail - 0% coverage
      - CreateUser, UpdateLastUsed, RevokeUser - 0% coverage
      - Estimated impact: +3-5 percentage points to overall coverage

   3. **Server/Handler Layer Enhancement** (`internal/server/handlers.go`)
      - Currently at 72.9%, can reach 80%+ with additional edge case tests
      - Error scenario testing, validation edge cases
      - Estimated impact: +2-3 percentage points

   4. **Event Processor Integration** (`internal/providers/aws/events/backend.go`)
      - Status determination logic - Currently tested indirectly
      - WebSocket notification flow - 0% coverage
      - Estimated impact: +2-3 percentage points

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
      "secrets": ["github-token", "db-password"],
      "timeout": 300
    }
    ```
3. Orchestrator Lambda:
   - Validates API key
   - Resolves referenced secrets via `database.SecretsRepository` and merges them with user-provided environment variables (user values take precedence when keys overlap)
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
- The sidecar command is dynamically generated in `internal/providers/aws/app/runner.go` via `buildSidecarContainerCommand()`
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
- AWS Runner (`internal/providers/aws/app/runner.go`): `KillTask` finds task via ListTasks, checks status, and calls StopTask
- ECS task lifecycle statuses (`LastStatus`) are represented by `constants.EcsStatus` typed constants (e.g., `EcsStatusRunning`, `EcsStatusStopped`) to avoid string literals across the codebase
- Execution status values are represented by `constants.ExecutionStatus` typed constants (e.g., `ExecutionStarting`, `ExecutionRunning`, `ExecutionSucceeded`, `ExecutionFailed`, `ExecutionStopped`, `ExecutionTerminating`) as part of the API contract
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
  - Command is dynamically generated by `internal/providers/aws/app/runner.go` via `buildSidecarContainerCommand()`
  - Creates `.env` file from user environment variables (variables prefixed with `RUNVOY_USER_`)
  - Injects resolved secrets (and other user-provided environment variables) directly into the process environment
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

The platform uses DynamoDB tables for data persistence. All tables are defined in the CloudFormation template (`deploy/providers/aws/cloudformation-backend.yaml`).

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

**Code Reference:** `internal/providers/aws/database/dynamodb/users.go`

### Executions Table (`{project-name}-executions`)

Stores execution records for all command runs.

**Primary Key:**
- `execution_id` (String) - Unique execution identifier

**Global Secondary Indexes:**
- `all-started_at` - Query all executions sorted by `started_at` (uses `_all` as constant partition key)

**Attributes:**
- `execution_id` (String) - Primary key, unique execution identifier
- `started_at` (Number) - Unix timestamp when execution started (used as sort key in GSI)
- `_all` (String) - Constant value ("1") used as partition key in `all-started_at` GSI for querying all executions
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

**Code Reference:** `internal/providers/aws/database/dynamodb/executions.go`

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

**Code Reference:** `internal/providers/aws/database/dynamodb/pending_keys.go`

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
