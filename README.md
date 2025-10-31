# runvoy

A centralized execution platform to run commands remotely without sharing credentials. An AWS admin deploys runvoy once to a company's AWS account, then issues API keys to team members for secure, audited command execution.

Think of Terraform Cloud without the need for a Terraform Cloud account (and monthly bill...).

Think of the flexibility of invoking `kubectl run` without the need for a Kubernetes cluster (or any other _always-running_ cluster, for that matter). Runvoy lets your team come up with a set of shared runbooks which can be executed by all the team members without the need for ssh nor of an AWS account.

Think of running commands in an ephemeral environment and sharing execution logs like with Github Actions, but without the need for a CI/CD pipeline nor a 3rd party service.

![runvoy demo](runvoy-demo.gif)

## Overview

runvoy solves the challenge of giving team members access to run infrastructure commands (terraform, CDK, kubectl, etc.) without distributing admin credentials. Deploy once, secure forever.

**Key Benefits:**
- **No credential sharing**: Team members never see AWS credentials
- **Complete audit trail**: Every execution logged with user identification
- **Safe stateful operations**: Automatic locking prevents concurrent conflicts
- **Self-service**: Team members don't wait for admins to run commands
- **Self-hosted**: The backend runs in your AWS account, you control everything
- **Serverless**: No always-running servers, just pay for the compute your commands comsume
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

### Discovering Commands

To see all available commands and their descriptions, use Cobra's built-in help:

```bash
runvoy --help
```

This will display all available commands. For more details about a specific command, use:

```bash
runvoy [command] --help
```

For example, to see all user management commands:

```bash
runvoy users --help
```

### Common Commands Examples

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
#   Execution ID: 61fb9138466c4212b1e0d763a7f4dfe2
#   Status: RUNNING
# → View logs in web viewer: https://runvoy-releases.s3.us-east-2.amazonaws.com/webviewer.html?execution_id=61fb9138466c4212b1e0d763a7f4dfe2
```

**Log Viewing:**
```bash
runvoy logs <executionID>

# Example
runvoy logs 72f57686926e4becb89116b0ac72caec

# Default behavior
# - Waits until the execution starts (spinner)
# - Prints all available logs once, then exits

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
→ View logs in web viewer: https://runvoy-releases.s3.us-east-2.amazonaws.com/webviewer.html?execution_id=72f57686926e4becb89116b0ac72caec
```

**Web Viewer:**

In addition to the CLI, you can view logs in a browser using the web viewer. The CLI automatically provides a web viewer link when you run a command:

```bash
runvoy run "echo hello world"
# Output includes:
# View logs in web viewer: https://runvoy-releases.s3.us-east-2.amazonaws.com/webviewer.html?execution_id=72f57686926e4becb89116b0ac72caec
```

The web viewer is a minimal, single-page application that provides:
- **Real-time log streaming** - Automatically polls for new logs every 5 seconds
- **ANSI color support** - Displays colored terminal output
- **Status tracking** - Shows execution status (RUNNING, SUCCEEDED, FAILED, STOPPED)
- **Execution metadata** - Displays execution ID, start time, and exit codes
- **Interactive controls**:
  - Pause/Resume polling
  - Download logs as text file
  - Clear display
  - Toggle metadata (line numbers and timestamps)

**Setup (first-time only):**
1. Open the web viewer URL in your browser
2. Enter your API endpoint URL (same as in `~/.runvoy/config.yaml`)
3. Enter your API key (same as in `~/.runvoy/config.yaml`)
4. Settings are saved in browser's localStorage for future use

The web viewer is hosted on AWS S3 and requires no installation - just open the URL in any modern browser.

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

### Environment Configuration

The project requires a `.env` file in the repository root for development. Create it from the example:

```bash
cp .env.example .env
```

Edit `.env` with your actual values. The `justfile` uses `dotenv-required`, so all `just` commands will fail if `.env` is missing.

See `.env.example` for all available environment variables and their descriptions.

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

### Development with `just`

The repository ships with a `justfile` to streamline common build, deploy, and QA flows. The default recipe (`just runvoy`) rebuilds the CLI before running any arguments you pass through, so you can quickly exercise commands locally:

```bash
# equivalent to: go build ./cmd/runvoy && ./bin/runvoy logs <id>
just runvoy logs <execution-id>
```

Key targets, grouped by workflow:

- **Build & run**: `just build` (all binaries), `just build-cli`, `just build-local`, `just run-local` (local HTTP server with freshly built binary)
- **Deploy artifacts**: `just deploy` (all), `just deploy-orchestrator`, `just deploy-event-processor`, `just deploy-webviewer`
- **Quality gates**: `just test`, `just test-coverage`, `just lint`, `just lint-fix`, `just fmt`, `just check`, `just clean`
- **Environment setup**: `just dev-setup`, `just install-hooks`, `just pre-commit-all`
- **Infrastructure helpers**: `just create-lambda-bucket`, `just update-backend-infra`, `just destroy-backend-infra`
- **Operational tooling**: `just seed-admin-user`, `just local-dev-server` (hot reloading), smoke tests such as `just smoke-test-local-create-user`, `just smoke-test-backend-run-command`
- **Miscellaneous**: `just record-demo` (captures CLI demo as cast and GIF)

All commands honor the environment variables described in the `justfile`; AWS credentials and profiles must already be configured in your shell.

**Note:** The `justfile` requires a `.env` file to be present (see [Environment Configuration](#environment-configuration)). All `just` commands will fail if `.env` is missing.

### CLI Installation

1. Clone the repository:
```