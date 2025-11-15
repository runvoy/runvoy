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

Workstations shouldn't be snowflakes that need complex setups, let remote containers (_run envoys..._) execute the actual commands in a privileged, production grade environment.

## Use cases

- one-off arbitrary commands in remote containers like with `kubectl run` without the need for a Kubernetes cluster (or any other _always-running_ cluster, for that matter)
- compute-intensive tasks like e.g. test suites: select the proper instance type for the job, tail and/or share execution logs in real time like in GitHub Actions
- dev team that runs Terraform from a shared repository without the need for a Terraform Cloud account (and monthly bill...)
- any commands which execution require full audit trail
- ...

## Overview

Runvoy is composed of 3 main parts:

- a CLI client (`runvoy`) to interact with the runvoy API
- a web app client (<https://runvoy.site>, or self hosted), currently supporting only the logs view, with plans to map 1:1 with the CLI commands
- a backend running on your AWS account (with plans to support other cloud providers in the future) which exposes the HTTP API endpoint and interacts with the cloud resources

**Key Benefits:**

- **_Doesn't_ run on your computer**: The actual commands are executed in remote production-grade environments properly configured for access to secrets and other resources, team member's workstations don't need any special configuration, just `runvoy` CLI and its API key
- **Complete audit trail**: Every interaction with the backend is logged with user identification. All logs stored in read-only database for auditing purposes (currently only CloudWatch Logs is supported, but with plans to extend support to other cloud services in the future)
- **Self-hosted, no black magic**: The backend runs in your cloud provider account, you control everything, including the policies and permissions assigned to the containers
- **Serverless**: No always-running services, just pay for the compute your commands consume (essentially free for infrequent use)

## Features

- **API key authentication** - Secure access with hashed API keys (SHA-256)
- **Customizable container task and execution roles** - Register Docker images with custom task and execution roles to e.g. run Terraform with the right permissions to access AWS resources (currently only AWS ECS is supported)
- **Automatic git cloning** - Optionally clone a (public or private) Git repository into the container working directory
- **Native cloud provider logging integration** - Full execution logs and audit trails with request ID tracking
- **Reusable playbooks** - Store and reuse command execution configurations in YAML files, share with your team to execute commands consistently
- **Secrets management** - Centralized encrypted secrets with full CRUD from the CLI
- **Real-time WebSocket streaming** - CLI and web viewer receive live logs over authenticated WebSocket connections
- **Unix-style output streams** - Separate CLI logs (stderr) from data (stdout) for easy piping and scripting
- **IaC deployment** - Deploy complete backend infrastructure with IaC templates (currently only AWS CloudFormation is supported, but with plans to extend support to other cloud providers in the future)

### Roadmap (NOT IMPLEMENTED YET!)

- **RBAC** - Role based access control for the backend API. Runvoy admins define roles and permissions for users, non-admin users can only execute commands / access secrets/ select Docker images they are allowed to
- **Multi-cloud support** - Backend support for other execution platforms, cloud providers (GCP, Azure...), Kubernetes, ...

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
runvoy - 0.1.0-20251115-1293191
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

### CLI commands

See [CLI Documentation](docs/CLI.md) for more details.

### Web Viewer

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

## Development

### Prerequisites for Development

- Go 1.24 or later
- [just](https://github.com/casey/just) command runner
- AWS credentials configured in your shell environment

### Environment Configuration

The `.env` file is automatically created when you run `just init` or `just local-dev-sync`. The `local-dev-sync` command syncs environment variables from the runvoy-orchestrator Lambda function to your local `.env` file for development.

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
