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
infra/        - CloudFormation templates for AWS infrastructure
```

## Contributing

1. Install pre-commit hooks: `just install-hooks`
2. Make your changes
3. Run checks: `just check`
4. Submit a pull request

## License

[Add your license here]
