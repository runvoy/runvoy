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
GET  /api/v1/health     - Health check endpoint
POST /api/v1/users      - Create a new user with an API key
```

Both Lambda and local HTTP server use identical routing logic, ensuring development/production parity.

### Middleware Stack

The router uses a middleware stack for cross-cutting concerns:

1. **Content-Type Middleware**: Sets `Content-Type: application/json` for all responses
2. **Request ID Middleware**: Extracts AWS Lambda request ID and adds it to logging context
3. **Authentication Middleware**: Validates API keys and adds user context

The request ID middleware automatically:
- Extracts the AWS Lambda request ID from the Lambda context when available
- Adds the request ID to the request context for use by handlers
- Falls back gracefully when not running in Lambda environment

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
```
