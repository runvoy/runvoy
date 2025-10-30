# runvoy

A centralized execution platform that enables teams to run infrastructure commands remotely without sharing (AWS) credentials. An AWS admin deploys runvoy once to the company's AWS account and issues API keys to team members for secure, audited command execution.

Think of the flexibility of invoking `kubectl run` without the need for a Kubernetes cluster (or any other _always-running_ cluster, for that matter).

Think of running commands in an ephemeral environment and sharing execution logs like with Github Actions, but without the need for a CI/CD pipeline nor a 3rd party service.

Think of the locking benefits of Terraform Cloud, but without the need for a Terraform Cloud account.

## Overview

runvoy solves the challenge of giving team members access to run infrastructure commands (terraform, CDK, kubectl, etc.) without distributing AWS credentials. Deploy once, secure forever.

**Key Benefits:**
- **No credential sharing**: Team members never see AWS credentials
- **Complete audit trail**: Every execution logged with user identification
- **Safe stateful operations**: Automatic locking prevents concurrent conflicts
- **Self-service**: Team members don't wait for admins to run commands
- **Self-hosted**: Runs in your AWS account, you control everything

## Features

- **CloudFormation deployment** - Deploy complete backend infrastructure with CloudFormation templates
- **Flexible container images** - Use any Docker image (terraform, python, node, etc.)
- **API key authentication** - Secure access with hashed API keys (SHA-256)
- **Execution isolation** - Commands run in ephemeral ECS Fargate (ARM64) containers
- **CloudWatch integration** - Full execution logs and audit trails
- **Multi-user support** - Centralized execution for entire teams
- **Event-driven architecture** - Automatic execution tracking via EventBridge
- **Cost tracking** - Real-time Fargate cost calculation per execution
- **Execution locking** - Prevent concurrent operations on shared resources (e.g., Terraform state)

## Architecture

runvoy uses a serverless event-driven architecture built on AWS Lambda, ECS Fargate, DynamoDB, and EventBridge:

- **Orchestrator Lambda**: HTTPS endpoint (Function URL) for synchronous API requests
- **Event Processor Lambda**: Asynchronous event handler for ECS task completions
- **DynamoDB**: Stores API keys (hashed), execution records with status and costs
- **ECS Fargate**: Runs commands in isolated, ephemeral ARM64 containers
- **EventBridge**: Captures ECS task state changes for completion tracking
- **CloudWatch**: Logs all executions for audit and debugging

For detailed architecture information, see [ARCHITECTURE.md](ARCHITECTURE.md).

## Usage

### Commands

**User Management:**
```bash
# Create a new user (returns API key - save it immediately!)
runvoy users create <email>

# Revoke a user's API key
runvoy users revoke <email>
```

**Command Execution:**
```bash
runvoy run "<command>"

# Example
runvoy run "echo hello world"
# Output:
# 🚀 runvoy
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# → Running command: echo hello world
# ✓ Command execution started successfully
#   Execution ID: 72f57686926e4becb89116b0ac72caec
#   Status: RUNNING
#
# Note: Log viewing endpoint is not yet implemented
```

**Configuration:**
```bash
# Configure CLI with API key and endpoint URL
runvoy configure
```

**Other:**
```bash
# Show CLI and backend versions
runvoy version
```

### Global Flags

All commands support the following global flags:

- `--timeout <duration>` - Timeout for command execution (default: `10m`, e.g., `30s`, `1h`, `600`)
- `--verbose` - Enable verbose output
- `--debug` - Enable debugging logs

Example:
```bash
runvoy --verbose --timeout 5m users create alice@example.com
```

## Quick Start

### Prerequisites
- AWS account with appropriate permissions
- Go 1.23 or later (for development)
- AWS CLI configured
- [just](https://github.com/casey/just) command runner (optional, for development)

### Backend Deployment

Deploy the backend infrastructure using CloudFormation:
```bash
# See deployments/ directory for CloudFormation templates
# (Specific deployment instructions depend on your CloudFormation setup)
```

### CLI Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/runvoy.git
cd runvoy
```

2. Build the CLI:
```bash
go build -o runvoy ./cmd/runvoy
```

3. Configure the CLI:
```bash
./runvoy configure
# Enter your API endpoint and API key when prompted
```

### Development

The project uses `just` for development automation. Common commands:

```bash
# Install development tools
just dev-setup

# Build all binaries
just build

# Run local HTTP server (development mode)
just run-local

# Run tests
just test

# Run linter
just lint

# Run all checks (lint + test)
just check

# Format code
just fmt

# Install pre-commit hooks
just install-hooks
```

For all available commands, run `just --list`.

## Project Structure

```
bin/                     - Built binaries (temporary storage)
cmd/
  ├── runvoy/            - CLI client (Cobra-based)
  ├── local/             - Local HTTP server for development
  └── backend/aws/
      ├── orchestrator/  - Lambda for synchronous API requests
      └── event_processor/ - Lambda for asynchronous ECS events
internal/
  ├── app/               - Service layer (business logic)
  │   └── aws/           - AWS-specific runner (ECS Fargate)
  ├── api/               - API request/response types
  ├── server/            - HTTP router, handlers, middleware (chi-based)
  ├── lambdaapi/         - Lambda Function URL event adapter
  ├── events/            - Event processor (ECS completion handler)
  ├── database/          - Repository interfaces and DynamoDB implementation
  ├── client/            - Generic HTTP client for CLI
  ├── config/            - Configuration (CLI YAML + environment variables)
  ├── logger/            - Structured logging (slog)
  ├── errors/            - Custom error types with HTTP status codes
  ├── constants/         - Project constants
  └── output/            - CLI output formatting
deployments/             - CloudFormation templates (if available)
```

## Contributing

1. Install pre-commit hooks: `just install-hooks`
2. Make your changes
3. Run checks: `just check`
4. Submit a pull request

## License

[Add your license here]
