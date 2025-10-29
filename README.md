# runvoy

A centralized execution platform that enables teams to run infrastructure commands remotely without sharing AWS credentials. An AWS admin deploys runvoy once to the company's AWS account and issues API keys to team members for secure, audited command execution.

## Overview

runvoy solves the challenge of giving team members access to run infrastructure commands (terraform, CDK, kubectl, etc.) without distributing AWS credentials. Deploy once, secure forever.

**Key Benefits:**
- **No credential sharing**: Team members never see AWS credentials
- **Complete audit trail**: Every execution logged with user identification
- **Safe stateful operations**: Automatic locking prevents concurrent conflicts
- **Self-service**: Team members don't wait for admins to run commands
- **Self-hosted**: Runs in your AWS account, you control everything

## Features

- **One-command setup** - Deploy complete infrastructure with `runvoy-init`
- **Git-integrated** - Automatically clones your repository before execution
- **Flexible images** - Use any Docker image (terraform, python, node, etc.)
- **API key authentication** - Secure access with encrypted credentials
- **Execution isolation** - Commands run in ephemeral ECS Fargate containers
- **CloudWatch integration** - Full execution logs and audit trails
- **Multi-user support** - Centralized execution for entire teams

## Architecture

runvoy uses a serverless architecture built on AWS Lambda, ECS Fargate, and DynamoDB:

- **Lambda Function URL**: HTTPS endpoint for API requests
- **DynamoDB**: Stores API keys (hashed), users, locks, and execution records
- **ECS Fargate**: Runs commands in isolated, ephemeral containers
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
- Go 1.24 or later (for development)
- AWS CLI configured

### Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/runvoy.git
cd runvoy
```

2. Install development dependencies:
```bash
just dev-setup
```

3. Build the binaries:
```bash
just build
```

### Development

Run the local development server:
```bash
just run-local
```

Run tests:
```bash
just test
```

Run all checks (lint + test):
```bash
just check
```

For more commands, see the `justfile` or run `just --list`.

## Project Structure

```
cmd/          - Entry points (CLI client, Lambda backend, local server)
internal/     - Application code (API, business logic, database, middleware)
deployments/  - CloudFormation templates for AWS infrastructure
```

## Contributing

1. Install pre-commit hooks: `just install-hooks`
2. Make your changes
3. Run checks: `just check`
4. Submit a pull request

## License

[Add your license here]
