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
- **`internal/lambdaapi/adapter.go`**: Adapter to convert between Lambda events and standard http.Handler
- **`internal/lambdaapi/handler.go`**: Lambda handler that uses the chi router via adapter
- **`cmd/local/main.go`**: Local HTTP server implementation using the same router
- **`cmd/backend/aws/main.go`**: Lambda entry point that uses the chi-based handler

### Route Structure

All routes are defined in `internal/server/router.go`:

```text
GET  /api/v1/health     - Health check endpoint
GET  /api/v1/greet/{name} - Example greeting endpoint
```

Both Lambda and local HTTP server use identical routing logic, ensuring development/production parity.

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
```
