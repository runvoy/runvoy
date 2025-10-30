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
# Run 'runvoy logs <executionID>' to view logs
```

**Log Viewing:**
```bash
runvoy logs <executionID>

# Example
runvoy logs 72f57686926e4becb89116b0ac72caec

# Default behavior
# - Waits until the execution starts (spinner)
# - Prints all available logs once with a Line column, then exits

# Sample output
🚀 runvoy
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
→ Getting logs for execution: 72f57686926e4becb89116b0ac72caec
⠋ Waiting for execution to start...

Line  Timestamp (UTC)      Message
────  ───────────────────  ───────────────────────────────────────────────────────
1     2025-10-30 13:32:48  Runvoy Runner execution started by requestID 1234567890
2     2025-10-30 13:32:48  terraform plan
3     2025-10-30 13:32:49  Refreshing Terraform state in-memory prior to plan...
...
✓ Logs retrieved successfully

# Follow mode
runvoy logs -f <executionID>
# - Same as above but continues streaming new lines every 5s until completion
# - Prints lines in the format: "[<line>] <timestamp>  <message>"
```

**Configuration:**
```bash
# Configure CLI with API key and endpoint URL
runvoy configure
```

**List Executions:**
```bash
runvoy executions

# Prints a table similar to logs:
# Execution ID  Status     Command        Started (UTC)        Completed (UTC)   Duration  Cloud
# abc123        RUNNING    terraform plan 2025-10-30 13:32:48                   
# def456        SUCCEEDED  echo hello     2025-10-29 09:10:00  2025-10-29 09:10:05 5s      AWS
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

### Admin bootstrap (automated)

After `just update-backend-infra`, the pipeline seeds an admin user into the API keys table:
- Reads `api_key` from `~/.runvoy/config.yaml`
- Hashes with SHA-256 and base64 (same as the service)
- Inserts `{ api_key_hash, user_email, created_at, revoked=false }` into the `${ProjectName}-api-keys` table

Requirements:
- Set `RUNVOY_ADMIN_EMAIL` to the desired admin email before running `just update-backend-infra`.
- Ensure `~/.runvoy/config.yaml` exists and contains `api_key`.

The operation is idempotent (conditional put); if the admin already exists, it’s skipped.

### CLI Installation

1. Clone the repository:
```