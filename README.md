<h1 align="center">
    <p><strong>🚀 Runvoy</strong></p>
    <p>serverless command execution platform</p>
</h1>
<p align="center">
    <em>Run arbitrary commands on remote ephemeral containers.</em>
</p>
<p align="center">
  <a href="https://github.com/runvoy/runvoy/actions/workflows/tests-and-coverage.yml" target="_blank">
      <img src="https://github.com/runvoy/runvoy/actions/workflows/tests-and-coverage.yml/badge.svg?event=push&branch=main" alt="Tests">
  </a>
  <a href="https://github.com/runvoy/runvoy/actions/workflows/golangci-lint.yml" target="_blank">
      <img src="https://github.com/runvoy/runvoy/actions/workflows/golangci-lint.yml/badge.svg?event=push&branch=main" alt="Lint">
  </a>
  <a href="https://golang.org" target="_blank">
      <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go" alt="Go version">
  </a>
</p>

---

Deploy once, issue API keys, let your team run arbitrary (admin) applications safely from their terminals. Share playbooks to perform tasks consistently and reliably.

Workstations shouldn't need complex setups, let remote containers execute the actual commands in a secured and reproducible production grade environment.

No more snowflakes, _run envoys_.

## Use cases

- AWS CLI commands or any other application based on AWS SDKs (e.g. Terraform) in a remote container with the right permissions to access AWS resources (see [AWS CLI example](.runvoy/aws-cli-example.yml))
- one-off arbitrary commands like with `kubectl run` without the need for a Kubernetes cluster (or any other _always-running_ cluster, for that matter). Example: `runvoy run ping <my service ip>`
- resource-intensive tasks like e.g. test runners: select the proper instance type for the job, tail and/or share execution logs in real time like in GitHub Actions (see [Build Caddy example](.runvoy/build-caddy-example.yml))
- any commands that require a full audit trail
- ...

## Overview

Runvoy is composed of 3 main parts:

- a backend running on your AWS account (with plans to support other cloud providers in the future) which exposes the HTTP API endpoint and interacts with the cloud resources, to be deployed once by a cloud admin
- a CLI client (`runvoy`) for users to interact with the runvoy REST API
- a web app client (<https://runvoy.site>, or self hosted), currently supporting only the logs view, with plans to map 1:1 to the CLI commands

**Key Benefits:**

- **_Doesn't_ run on your computer**: The actual commands are executed in remote production-grade environments properly configured for access to secrets and other resources, team member's workstations don't need any special configuration, just `runvoy` CLI and its API key
- **Complete audit trail**: Every interaction with the backend is logged with user identification. All logs stored in read-only database for auditing purposes (currently only CloudWatch Logs is supported, but with plans to extend support to other cloud services in the future)
- **Self-hosted, no black magic**: The backend runs in your cloud provider account, you control everything, including the policies and permissions assigned to the containers
- **Serverless**: No always-running services, just pay for the compute your commands consume (essentially free for infrequent use)

## Features

- **API key authentication** - Secure access with hashed API keys (SHA-256)
- **Customizable container task and execution roles** - Register Docker images with custom task and execution roles to e.g. run Terraform with the right permissions to access AWS resources (currently only AWS ECS is supported)
- **Automatic git cloning** - Optionally clone a (public or private) Git repository into the container working directory
- **User access management** - Role based and ownership access control for the backend API. Runvoy admins define roles and permissions for users, non-admin users can only access secrets / select Docker images / see logs of executions they are allowed to
- **Native cloud provider logging integration** - Full execution logs and audit trails with request ID tracking
- **Reusable playbooks** - Store and reuse command execution configurations in YAML files, commit to a repository and share with your team to execute commands consistently (see [Terraform example](.runvoy/terraform-example.yml))
- **Secrets management** - Centralized encrypted secrets with full CRUD from the CLI
- **Real-time WebSocket streaming** - CLI and web viewer receive live logs over authenticated WebSocket connections
- **Unix-style output streams** - Separate CLI logs (stderr) from data (stdout) for easy piping and scripting
- **IaC deployment** - Deploy complete backend infrastructure with IaC templates (currently only AWS CloudFormation is supported, but with plans to extend support to other cloud providers in the future)

### Roadmap (NOT IMPLEMENTED YET!)

- **Multi-cloud support** - Backend support for other execution platforms, cloud providers (GCP, Azure...), Kubernetes, ...
- **Timeouts for command execution** - Send timed SIGTERM to the command execution if it doesn't complete within the timeout period
- **Lock management for concurrent command execution** - Prevent multiple users from executing the same command concurrently
- **Webapp - CLI command parity** - Allow users to perform all CLI commands from the webapp

## Quick Start

### CLI Users (requires Go 1.25 or later)

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

- Go 1.25 or later
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
runvoy - 0.2.0-20251118-c73328b
Isolated, repeatable execution environments for your commands

Usage:
  runvoy [command]

Available Commands:
  claim       Claim a user's API key
  completion  Generate the autocompletion script for the specified shell
  configure   Configure local environment with API key and endpoint URL
  health      Health and reconciliation commands
  help        Help about any command
  images      Docker images management commands
  kill        Kill a running command execution
  list        List command executions
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

<!-- CLI_HELP_END -->

See [CLI commands Documentation](docs/CLI.md) for more details.

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

### Web Viewer

The web viewer is a SvelteKit-based single-page application that provides:

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

**Development:**

To run the webapp locally with hot reloading:

```bash
just local-dev-webapp
```

The webapp will be available at <http://localhost:5173>

Alternatively, you can use npm commands directly from the `cmd/webapp` directory:

```bash
# Install dependencies
npm install

# Start dev server (with hot reload)
npm run dev

# Build for production (static files)
npm run build

# Preview production build
npm run preview
```

The build process creates a `dist/` directory optimized for static file hosting. The webapp is built with SvelteKit using the static adapter, and deployed via the `deploy-webapp` command in the justfile.

## Architecture

For detailed architecture information, see [ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Development

The repository ships with a `justfile` to streamline common build, deploy, and QA flows. Run `just --list` to see all available commands.

### Prerequisites for Development

- Go 1.25 or later
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
