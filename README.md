<h1 align="center">
    <p><strong>🚀 Runvoy</strong></p>
    <p>serverless command execution platform</p>
</h1>
<p align="center">
    <em>Run arbitrary commands on remote ephemeral containers.</em>
</p>
<p align="center">
  <a href="https://github.com/runvoy/runvoy/actions?query=workflow%3Atests+event%3Apush+branch%3Amain" target="_blank">
      <img src="https://github.com/runvoy/runvoy/actions/workflows/ci.yml/badge.svg?event=push&branch=main" alt="Tests">
  </a>
  <a href="https://github.com/runvoy/runvoy/actions?query=workflow%3Agolangci-lint+event%3Apush+branch%3Amain" target="_blank">
      <img src="https://github.com/runvoy/runvoy/actions/workflows/golangci-lint.yml/badge.svg?event=push&branch=main" alt="Lint">
  </a>
  <a href="https://golang.org" target="_blank">
      <img src="https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go" alt="Go version">
  </a>
</p>

---

Deploy once, issue API keys, let your team execute arbitrary (admin) commands safely from their terminals. Share playbooks with your team to execute commands consistently and reliably.

## Use cases

- run arbitrary commands in remote containers like with `kubectl run` without the need for a Kubernetes cluster (or any other _always-running_ cluster, for that matter)
- run Terraform "in the cloud" without the need for a Terraform Cloud account (and monthly bill...)
- share execution logs like with Github Actions, but without the need for a CI/CD pipeline nor a 3rd party service

## Overview

Runvoy is composed of 3 main parts:

- a CLI client (`runvoy`) to interact with the runvoy API
- a web app client (<https://runvoy.site>, or self hosted), currently supporting only the logs view, with plans to map 1:1 with the CLI commands
- a backend running on your AWS account (with plans to support other cloud providers in the future) which exposes the HTTP API endpoint and interacts with the cloud resources

It tries to simplify the requirements for running any kind of "privileged" application (Terraform, Ansible, Kubectl, etc.) without distributing admin credentials nor configuration details.

Workstations shouldn't be snowflakes that need complex setups, let remote containers (_run envoys..._) execute the actual commands in a privileged, production grade environment.

**Key Benefits:**

- **Doesn't _run on your computer_**: The actual commands are executed in remote production-grade environments properly configured for access to secrets and other resources, team member's workstations don't need any special configuration with credentials, environment variables, and so on
- **Complete audit trail**: Every interaction with the backend is logged with user identification. All logs stored in read-only database for auditing purposes (currently only CloudWatch Logs is supported, but with plans to extend support to other cloud services in the future)
- **Self-hosted, no black magic**: The backend runs in your cloud provider account, you control everything, including the policies and permissions assigned to the containers
- **Serverless**: No always-running services, just pay for the compute your commands consume (essentially free for infrequent use)

## Features

- **API key authentication** - Secure access with hashed API keys (SHA-256)
- **IaC deployment** - Deploy complete backend infrastructure with IaC templates (currently only AWS CloudFormation is supported, but with plans to extend support to other cloud providers in the future)
- **Flexible container images** - Use any public Docker image (Ubuntu, Python, Node, etc.)
- **Execution isolation** - Commands run in ephemeral containers
- **Native cloud provider logging integration** - Full execution logs and audit trails with request ID tracking
- **Reusable playbooks** - Store and reuse command execution configurations in YAML files, share with your team to execute commands consistently
- **Secrets management** - Centralized encrypted secrets with full CRUD from the CLI
- **Real-time WebSocket streaming** - CLI and web viewer receive live logs over authenticated WebSocket connections
- **Unix-style output streams** - Separate CLI logs (stderr) from data (stdout) for easy piping and scripting
- **RBAC** - Role based access control for the backend API (NOT IMPLEMENTED YET). Runvoy admins define roles and permissions for users, non-admin users can only execute commands / clone repos / select Docker images they are allowed to use

## Quick Start

### CLI Users (requires Go 1.24 or later)

Install the CLI

```bash
go build -o $(go env GOPATH)/bin/runvoy ./cmd/cli
```

then run `runvoy configure` to configure the CLI with the endpoint URL given by your admin.

NOTE: this is a temporary solution until we have a proper release process, install will probably look something like this:

```bash
go install github.com/runvoy/runvoy@latest
```

and a download page to get the latest release from the [releases page](https://github.com/runvoy/runvoy/releases) for users without Go installed.

### Admin user

Requirements:

- Go 1.24 or later
- [just](https://github.com/casey/just) command runner installed
- AWS credentials configured in your shell environment (see [AWS credentials configuration](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html))

This will bootstrap the backend infrastructure and seed the admin user:

```bash
just init
```

#### User Onboarding

The admin API key and endpoint are automatically configured in `~/.runvoy/config.yaml` after running `just init`. You can start using runvoy immediately:

```bash
runvoy run "echo hello world"
```

or

```bash
runvoy users create <email>
```

to create a new user account for a team member. This will generate a claim token that the user can use to claim their API key.

**Important Notes:**

- ⏱  Claim tokens expire after 15 minutes
- 👁  Each token can only be used once

## Usage

<!-- CLI_HELP_START -->
### Available Commands

To see all available commands and their descriptions:

```bash
runvoy --help
```

```text
runvoy - 0.1.0-20251113-fd387da
Isolated, repeatable execution environments for your commands

Usage:
  runvoy [command]

Available Commands:
  claim       Claim a user's API key
  completion  Generate the autocompletion script for the specified shell
  configure   Configure local environment with API key and endpoint URL
  help        Help about any command
  images      Docker images management commands
  kill        Kill a running command execution
  list        List executions
  logs        Get logs for an execution
  playbook    Manage and execute playbooks
  run         Run a command
  secrets     Secrets management commands
  status      Get the status of a command execution
  users       User management commands
  version     Show the version of the CLI

Flags:
      --debug            Enable debugging logs
  -h, --help             help for runvoy
      --timeout string   Timeout for command execution (e.g., 10m, 30s, 1h) (default "10m")
      --verbose          Verbose output

Use "runvoy [command] --help" for more information about a command.
```

For more details about a specific command, use:

```bash
runvoy [command] --help
```

For example, to see all user management commands:

```bash
runvoy users --help
```

<!-- CLI_HELP_END -->

### Common Commands Examples

**Command Execution:**

```bash
runvoy run <command...>

# Example
runvoy run --git-repo https://github.com/mycompany/myproject.git npm run tests
# Output:
# 🚀 runvoy
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# → Running command: npm run tests
# ✓ Command execution started successfully
#   Execution ID: 61fb9138466c4212b1e0d763a7f4dfe2
#   Status: RUNNING
# → Logs not available yet, waiting 10 seconds... (attempt 1/3)
# → Logs not available yet, waiting 10 seconds... (attempt 2/3)
#
# Line  Timestamp (UTC)  Message
# ────  ───────────────  ───────
#
# → Connecting to log stream...
# ✓ Connected to log stream. Press Ctrl+C to exit.
#
# 1     2025-11-08 10:00:00  Execution starting
# 2     2025-11-08 10:00:05  ...
# ... (continues to stream new logs in real-time)
#
# ^C
# → Received interrupt signal, closing connection...
```

Secrets stored via `runvoy secrets` can be mounted into the execution environment:

```bash
# Secrets can be specified multiple times; user-provided env vars override secret values with the same key.
runvoy run --secret github-token --secret db-password terraform plan
```

**Log Viewing:**

`runvoy logs` first retrieves the full log history via the REST API. When the execution is still running, the backend returns a WebSocket URL; the CLI connects to that URL to stream new log events live, and falls back to the web viewer link if the connection closes.

```bash
runvoy logs <executionID>

# Example
runvoy logs 72f57686926e4becb89116b0ac72caec

# Default behavior
# - Waits until the execution starts (spinner)
# - Prints all available logs and start tailing logs in real-time
# - Stops tailing when the execution reaches a terminal status (COMPLETED/SUCCEEDED/FAILED/etc.)

# Sample output
runvoy --verbose logs 2e1c58557c3f4ee1a81c0071fdd0b1e9
```

```text
🚀 runvoy logs
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
→ CLI build: 0.1.0-20251104-b03e68c
→ Verbose output enabled
→ Timeout: 10m0s
→ Loaded configuration from /Users/alex/.runvoy/config.yaml
→ API endpoint: http://localhost:56212/
→ Getting logs for execution: 2e1c58557c3f4ee1a81c0071fdd0b1e9

Line  Timestamp (UTC)      Message
────  ───────────────────  ────────────────────────────────────────────────────────────
1     2025-11-04 18:18:04  ### runvoy command => while true; do date -u; sleep 30; done
2     2025-11-04 18:18:04  Tue Nov  4 18:18:04 UTC 2025
3     2025-11-04 18:18:34  Tue Nov  4 18:18:34 UTC 2025
4     2025-11-04 18:19:04  Tue Nov  4 18:19:04 UTC 2025

→ Connecting to log stream...
✓ Connected to log stream. Press Ctrl+C to exit.

5     2025-11-04 18:19:34  Tue Nov  4 18:19:34 UTC 2025
6     2025-11-04 18:20:04  Tue Nov  4 18:20:04 UTC 2025
7     2025-11-04 18:20:34  Tue Nov  4 18:20:34 UTC 2025
8     2025-11-04 18:21:04  Tue Nov  4 18:21:04 UTC 2025
9     2025-11-04 18:21:34  Tue Nov  4 18:21:34 UTC 2025
... (continues to stream new logs in real-time)

^C
→ Received interrupt signal, closing connection...
→ View logs in web viewer: http://localhost:56212/webapp/index.html?execution_id=2e1c58557c3f4ee1a81c0071fdd0b1e9
```

**Web Viewer:**

The web viewer is a minimal, single-page application that provides:

- **Real-time log streaming** - Automatically get updates from the websocket API in real-time
- **ANSI color support** - Displays colored terminal output
- **Status tracking** - Shows execution status (RUNNING, SUCCEEDED, FAILED, STOPPED)
- **Execution metadata** - Displays execution ID, start time, and exit codes
- **Interactive controls**:
  - Pause/Resume streaming
  - Download logs as text file
  - Clear display
  - Toggle metadata (line numbers and timestamps)

**Setup (first-time only):**

1. Open the web viewer URL in your browser
2. Enter your API endpoint URL (same as in `~/.runvoy/config.yaml`)
3. Enter your API key (same as in `~/.runvoy/config.yaml`)
4. Settings are saved in browser's localStorage for future use

The web viewer is hosted on [Netlify](https://www.netlify.com/) by default, but you can configure a custom URL if you deploy your own instance (see Configuration below).

**Configuration:**

The web application URL can be customized via:

- Environment variable: `RUNVOY_WEB_URL`
- Config file (`~/.runvoy/config.yaml`): `web_url` field

If not configured, it defaults to `https://runvoy.site/`.

`just local-dev-webapp` to run the webapp locally, by default it will be available at <http://localhost:5173>

**User Management:**

```bash
# Create a new user
runvoy users create <email>

# Example
runvoy users create alice@example.com
# Output:
# 🚀 runvoy
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# → Creating user with email alice@example.com...
# ✓ User created successfully
#   Email: alice@example.com
#   Claim Token: abc123def456...
#
# ℹ Share this command with the user => runvoy claim abc123def456...
#
# ⏱  Token expires in 15 minutes
# 👁  Can only be viewed once

# List all users
runvoy users list

# Example output:
# 🚀 runvoy
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# → Listing users…
#
# Email                  Status    Created (UTC)        Last Used (UTC)
# ─────────────────────  ────────  ───────────────────  ───────────────────
# admin@example.com      Active    2025-10-30 10:00:00  2025-11-02 14:30:00
# alice@example.com      Active    2025-11-01 09:15:00  2025-11-02 12:45:00
# bob@example.com        Revoked   2025-10-28 16:20:00  2025-10-29 08:10:00
#
# ✓ Users listed successfully

# Revoke a user's API key
runvoy users revoke <email>

# Example
runvoy users revoke bob@example.com
```

### Secrets Management

The CLI exposes a full secrets workflow backed by AWS Systems Manager Parameter Store (encrypted with a dedicated KMS key) and a DynamoDB metadata catalog. Authenticated users can create, inspect, rotate, and delete shared secrets without distributing raw credentials out of band.

```bash
# Create or rotate a secret (value stored encrypted, metadata tracked for auditing)
runvoy secrets create github-token GITHUB_TOKEN "ghp_xxxxx" --description "GitHub personal access token"

# List available secrets with key bindings and ownership metadata
runvoy secrets list

# Retrieve the latest value when you need to inject it locally or into automation
runvoy secrets get github-token

# Update metadata or rotate the value in place
runvoy secrets update github-token --key-name GITHUB_API_TOKEN --value "ghp_new" --description "Rotated on 2025-11-12"

# Clean up secrets that are no longer required
runvoy secrets delete legacy-token
```

Secrets are versioned at the storage layer and always transmitted over HTTPS. The orchestrator keeps audit fields (created/updated by, timestamps) so teams can see who managed a secret and when.

### Playbooks

Playbooks allow you to define reusable command execution configurations in YAML files. They are stored in a `.runvoy/` directory (in your current working directory) and can be executed via the CLI.

**Creating a Playbook:**

Create a YAML file in the `.runvoy/` directory with a `.yaml` or `.yml` extension. The filename (without extension) becomes the playbook name.

Example playbook (`.runvoy/terraform-plan.yaml`):

```yaml
description: Terraform plan infrastructure
image: hashicorp/terraform:latest
git_repo: https://github.com/mycompany/infrastructure.git
git_ref: main
git_path: terraform/environments/production
secrets:
  - aws-credentials
  - terraform-backend-key
env:
  TF_VAR_environment: production
  TF_VAR_region: us-east-1
commands:
  - terraform init
  - terraform plan -out=plan.tfplan
```

**Playbook Fields:**

- `description` (optional): Human-readable description of the playbook
- `image` (optional): Docker image to use for execution
- `git_repo` (optional): Git repository URL to clone
- `git_ref` (optional): Git branch, tag, or commit SHA (defaults to "main" if not specified)
- `git_path` (optional): Working directory within the cloned repository
- `secrets` (optional): List of secret names to inject into the execution environment
- `env` (optional): Map of environment variables (key-value pairs)
- `commands` (required): List of commands to execute sequentially (combined with `&&`)

**Playbook Commands:**

```bash
# List all available playbooks
runvoy playbook list

# Show detailed information about a playbook
runvoy playbook show terraform-plan

# Execute a playbook
runvoy playbook run terraform-plan

# Execute a playbook with flag overrides
runvoy playbook run terraform-plan --image hashicorp/terraform:1.6.0

# Override multiple playbook values
runvoy playbook run terraform-plan \
  --image hashicorp/terraform:1.6.0 \
  --git-ref develop \
  --git-path terraform/environments/staging \
  --secret additional-secret
```

**Flag Overrides:**

When executing a playbook, you can override any playbook value using CLI flags:

- `--image` / `-i`: Override the Docker image
- `--git-repo` / `-g`: Override the Git repository URL
- `--git-ref` / `-r`: Override the Git reference
- `--git-path` / `-p`: Override the Git path
- `--secret`: Add additional secrets (merged with playbook secrets)

**Environment Variables:**

User environment variables prefixed with `RUNVOY_USER_` are automatically merged with playbook environment variables. User variables take precedence over playbook variables if there's a conflict.

```bash
# User env vars are merged with playbook env vars
RUNVOY_USER_API_KEY=abc123 runvoy playbook run my-playbook
```

**Example Playbooks:**

**Terraform Plan:**
```yaml
description: Run Terraform plan
image: hashicorp/terraform:latest
git_repo: https://github.com/mycompany/infrastructure.git
git_ref: main
secrets:
  - aws-credentials
env:
  TF_VAR_environment: production
commands:
  - terraform init
  - terraform plan
```

**Ansible Playbook:**
```yaml
description: Run Ansible playbook
image: quay.io/ansible/ansible-runner:latest
git_repo: https://github.com/mycompany/ansible-playbooks.git
git_ref: main
git_path: playbooks
secrets:
  - ssh-key
  - vault-password
commands:
  - ansible-playbook site.yml -i inventory/production
```

**Node.js Tests:**
```yaml
description: Run Node.js test suite
image: node:20
git_repo: https://github.com/mycompany/myapp.git
git_ref: main
env:
  NODE_ENV: test
commands:
  - npm install
  - npm run test
```

**Playbook Discovery:**

Playbooks are discovered in the following order:
1. Current working directory `.runvoy/` folder
2. Home directory `.runvoy/` folder (fallback)

If the playbook directory doesn't exist, it's treated as empty (no playbooks available).

### Output Streams and Piping

runvoy follows Unix conventions by separating informational messages from data output, making it easy to pipe commands and script automation workflows:

- **stderr (standard error)**: Runtime messages, progress indicators, and logs
  - Informational messages (→, ✓, ⚠, ✗)
  - Progress spinners and status updates
  - Headers and UI formatting

- **stdout (standard output)**: Actual data from API responses
  - Tables, lists, and structured data
  - Raw output for piping to other tools

**Examples:**

```bash
# Hide informational messages, show only data
runvoy list 2>/dev/null

# Hide data, show only logs/status messages
runvoy list >/dev/null

# Pipe data to another command (jq, grep, etc.)
runvoy list 2>/dev/null | grep "RUNNING"

# Redirect logs and data to separate files
runvoy list 2>status.log >executions.txt

# Pipe between runvoy commands
runvoy command1 2>/dev/null | runvoy command2

# Use in scripts with proper error handling
if runvoy status $EXEC_ID 2>/dev/null | grep -q "SUCCEEDED"; then
  echo "Execution succeeded"
fi
```

This separation enables clean automation and integration with other Unix tools without mixing informational output with parseable data.

### Global Flags

All commands support the following global flags:

- `--timeout <duration>` - Timeout for command execution (default: `10m`, e.g., `30s`, `1h`, `600`)
- `--verbose` - Enable verbose output
- `--debug` - Enable debugging logs

Example:

```bash
runvoy --verbose --timeout 5m users create alice@example.com
```

### Environment Configuration

The `.env` file is automatically created when you run `just init` or `just local-dev-sync`. The `local-dev-sync` command syncs environment variables from the runvoy-orchestrator Lambda function to your local `.env` file for development.

## Development

### Prerequisites for Development

- Go 1.24 or later
- [just](https://github.com/casey/just) command runner
- AWS credentials configured in your shell environment

### Environment Setup

First-time setup for new developers:

```bash
# Install dependencies and development tools
just dev-setup

# Install pre-commit hook
just install-hook

# Sync Lambda environment variables to local .env file
just local-dev-sync
```

### Local Development Workflow

**Run the local development server:**

```bash
# Build and run local server (rebuilds on each restart)
just run-local

# Run local server with hot reloading (rebuilds automatically on file changes)
just local-dev-server
```

**Sync environment variables from AWS:**

```bash
# Fetch current environment variables from runvoy-orchestrator Lambda and save to .env
just local-dev-sync
```

**Run tests and quality checks:**

```bash
# Run all tests
just test

# Run tests with coverage report
just test-coverage

# Lint code
just lint

# Format code
just fmt

# Run both lint and tests
just check
```

**Build and deploy:**

```bash
# Build all binaries (CLI, orchestrator, event processor, local server)
just build

# Deploy all services
just deploy

# Deploy backend
just deploy-backend

# Deploy webapp
just deploy-webapp

# Or deploy individual services
just deploy-orchestrator
just deploy-event-processor
just deploy-webapp
```

**Infrastructure management:**

```bash
# Initialize complete backend infrastructure
just init

# Create/update backend infrastructure
just create-backend-infra

# Destroy backend infrastructure
just destroy-backend-infra
```

**Other useful commands:**

```bash
# Seed admin user in AWS DynamoDB
just seed-admin-user admin@example.com runvoy-backend

# Update README with latest CLI help output
just update-readme-help

# Clean build artifacts
just clean
```

For more information about the development workflow, see [Development with `just`](#development-with-just).

## Architecture

For detailed architecture information, see [ARCHITECTURE.md](docs/ARCHITECTURE.md).

### Development with `just`

The repository ships with a `justfile` to streamline common build, deploy, and QA flows. Run `just --list` to see all available commands. The default recipe (`just runvoy`) rebuilds the CLI before running any arguments you pass through, so you can quickly exercise commands locally:

```bash
# equivalent to: go build ./cmd/cli && ./bin/runvoy logs <id>
just runvoy logs <execution-id>
```

Key targets, grouped by workflow:

- **Build & run**: `just build` (all binaries), `just build-cli`, `just build-local`, `just run-local` (local HTTP server with freshly built binary)
- **Deploy artifacts**: `just deploy` (all), `just deploy-orchestrator`, `just deploy-event-processor`, `just deploy-webviewer`
- **Quality gates**: `just test`, `just test-coverage`, `just lint`, `just lint-fix`, `just fmt`, `just check`, `just clean`
- **Environment setup**: `just dev-setup`, `just install-hook`
- **Infrastructure helpers**: `just create-lambda-bucket`, `just update-backend-infra`, `just destroy-backend-infra`
- **Operational tooling**: `just seed-admin-user`, `just local-dev-server` (hot reloading)
- **Miscellaneous**: `just record-demo` (captures CLI demo as cast and GIF)

All commands honor the environment variables described in the `justfile`; AWS credentials and profiles must already be configured in your shell.
