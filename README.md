<h1 align="center">
    <p><strong>🚀 Runvoy</strong></p>
    <p>serverless command runner</p>
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
  <a href="https://codecov.io/github/runvoy/runvoy" >
      <img src="https://codecov.io/github/runvoy/runvoy/graph/badge.svg?token=Q673GMB33N"/>
  </a>
  <a href="https://golang.org" target="_blank">
      <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go" alt="Go version">
  </a>
  <a href="https://runvoy.github.io" target="_blank">
      <img src="https://img.shields.io/badge/%F0%9F%9A%80%20Docs-orange" alt="Runvoy Docs">
  </a>
  <a href="https://runvoy.site" target="_blank">
      <img src="https://img.shields.io/badge/%F0%9F%9A%80%20Webapp-blue" alt="Runvoy Webapp">
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

## Example

```text
runvoy run "uname -a"

🚀 runvoy run
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
→ Running command: uname -a
✓ Command execution started successfully
  Execution ID: 010adbfb34374116b47c8d0faab2befa
  Status: STARTING
  Image ID: amazonlinux/amazonlinux:latest-d7ba6332
→ Execution status: STARTING
→ Execution is starting (logs usually ready after ~30 seconds)...
→ Connecting to log stream...
✓ Connected to log stream. Press Ctrl+C to exit.

Line  Timestamp (UTC)      Message
────  ───────────────────  ─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
1     2025-11-19 17:30:54  ### runvoy runner execution started by requestID => f59378e3-01ed-4b44-a315-618951e048aa
2     2025-11-19 17:30:54  ### Image ID => amazonlinux/amazonlinux:latest-d7ba6332
3     2025-11-19 17:30:54  ### runvoy command => uname -a
4     2025-11-19 17:30:54  Linux ip-172-20-1-130.us-east-2.compute.internal 5.10.245-241.978.amzn2.aarch64 #1 SMP Fri Oct 31 17:59:47 UTC 2025 aarch64 aarch64 aarch64 GNU/Linux

→ Execution completed. Closing connection...
→ WebSocket connection closed
→ View logs in web viewer: https://runvoy.site/?execution_id=010adbfb34374116b47c8d0faab2befa
```

## Overview

Runvoy is composed of 3 main parts (see [#architecture](#architecture) for more details):

- a backend running on your AWS account (with plans to support other cloud providers in the future) which exposes the HTTP API endpoint and interacts with the cloud resources, to be deployed once by a cloud admin
- a CLI client (`runvoy`) for users to interact with the runvoy REST API
- a web app client (<https://runvoy.site>, or self hosted), currently supporting only the logs view, with plans to map 1:1 to the CLI commands

**Key Benefits:**

- **_Doesn't_ run on your computer**: The actual commands are executed in remote production-grade environments properly configured for access to secrets and other resources, team member's workstations don't need any special configuration, just `runvoy` CLI and its API key
- **Complete audit trail**: Every interaction with the backend is logged with user identification. All logs stored in append-only database for auditing purposes (currently only CloudWatch Logs is supported, but with plans to extend support to other cloud services in the future)
- **Self-hosted, no black magic**: The backend runs in your cloud provider account, you control everything, including the policies and permissions assigned to the containers
- **Serverless**: No always-running services, just pay for the compute your commands consume (essentially free for infrequent use)

## Features

- **API key authentication** - Secure access with hashed API keys (SHA-256)
- **User access management** - Role based and ownership access control for the backend API. Runvoy admins define roles and permissions for users, non-admin users can only access secrets / select Docker images / see logs of executions they are allowed to
- **Customizable container task and execution roles** - Register Docker images with custom task and execution roles to e.g. run Terraform with the right permissions to access AWS resources (currently only AWS ECS is supported)
- **Native cloud provider logging integration** - Full execution logs and audit trails with request ID tracking
- **Reusable playbooks** - Store and reuse command execution configurations in YAML files, commit to a repository and share with your team to execute commands consistently (see [Terraform example](.runvoy/terraform-example.yml))
- **Secrets management** - Centralized encrypted secrets with full CRUD from the CLI
- **Real-time WebSocket streaming** - CLI and web viewer receive live logs over authenticated WebSocket connections
- **Automatic Git cloning** - Optionally clone a (private) Git repository into the container working directory (see [Build Caddy example](.runvoy/build-caddy-example.yml))
- **Unix-style output streams** - Separate CLI logs (stderr) from data (stdout) for easy piping and scripting
- **IaC deployment** - Deploy complete backend infrastructure with IaC templates (currently only AWS CloudFormation is supported, but with plans to extend support to other cloud providers in the future)
- **Single multi-platform binary deployment** - Download a single binary (tarball is ~ 6MB) and run it directly from the command line, no need to install any other dependencies

### Roadmap (NOT IMPLEMENTED YET!)

- **Multi-cloud support** - Backend support for other cloud providers (GCP, Azure...) or even compute platforms like Kubernetes if it makes sense
- **Robust log streaming** - Right now log streaming is lossy, more robust streaming mechanism on top of CloudWatch Logs is needed.
- **Timeouts for command execution** - Send timed SIGTERM to the command execution if it doesn't complete within the timeout period
- **Lock management for concurrent command execution** - Prevent multiple users from executing the same command concurrently
- **Webapp - CLI command parity** - Allow users to perform all CLI commands from the webapp
- **Homebrew package manager support** - Add Homebrew installation support for the CLI

## Quick Start

Download the latest release from the [releases page](https://github.com/runvoy/runvoy/releases), e.g.

<!-- VERSION_EXAMPLES_START -->
For Linux:

```bash
curl -L -o runvoy-cli-linux-arm64.tar.gz https://github.com/runvoy/runvoy/releases/download/v0.2.0/runvoy_linux_amd64.tar.gz
tar -xzf runvoy_linux_amd64.tar.gz
sudo mv runvoy_linux_amd64/runvoy /usr/local/bin/runvoy
```

For macOS:

```bash
curl -L -o runvoy_linux_amd64.tar.gz https://github.com/runvoy/runvoy/releases/download/v0.2.0/runvoy_darwin_arm64.tar.gz
tar -xzf runvoy_darwin_arm64.tar.gz
xattr -dr com.apple.quarantine runvoy_darwin_arm64/runvoy
codesign -s - --deep --force runvoy_darwin_arm64/runvoy
sudo mv runvoy_darwin_arm64/runvoy /usr/local/bin/runvoy
```

<!-- VERSION_EXAMPLES_END -->

### Deploying the backend infrastructure

Requirements:

- AWS credentials configured in your shell environment (see [AWS credentials configuration](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html))

This will bootstrap the backend infrastructure and seed the admin user:

```bash
runvoy infra apply --configure --region eu-west-1 --seed-admin-user admin@example.com
```

#### Creating a new user

The admin API key and endpoint are automatically configured in `~/.runvoy/config.yaml` after running the above command. You can start using runvoy immediately:

```bash
runvoy images register public.ecr.aws/docker/library/alpine:latest
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
runvoy - v0.2.0-20251121-3d5bcf0
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
  infra       Infrastructure management commands
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
just dev-webapp
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

```text
┌─────────────────┐          ┌─────────────────┐
│   CLI Client    │          │   Web Viewer    │
│   (runvoy)      │          │  (browser)      │
└────────┬────────┘          └────────┬────────┘
         │                            │
         │  HTTPS (REST API)          │  HTTPS (REST API)
         │  WebSocket (logs)          │  WebSocket (logs)
         │                            │
         └────────────┬───────────────┘
                      │
         ┌────────────▼─────────────────────────┐
         │     AWS Backend                      │
         │                                      │
         │  • Lambda Functions (Orchestrator,   │
         │    Event Processor)                  │
         │  • DynamoDB (metadata storage)       │
         │  • ECS Fargate (command execution)   │
         │  • CloudWatch Logs (execution logs)  │
         │  • EventBridge (event routing)       │
         │  • API Gateway WebSocket (real-time) │
         │  • SSM Parameter Store (secrets)     │
         └──────────────────────────────────────┘
```

For detailed architecture information, see [ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Why Runvoy?

For almost all my carer as a software engineer I frequently found myself in situations where I was an admin in need of a way to run commands and more importantly to allow my team members to run some of the same commands without me being in the way.

I've also always been fascinated by the idea of "thin clients" and delegating the heavy lifting and security concerns to the server while keeping the client simple and easy to use and setup: just "log in" and run the commands you're allowed to run, the server takes care of the rest.

The final piece of the puzzle was probably AWS launching Lambda and with that the _serverless_ concept of "you only pay for what you use" which meanwhile has become a commodity in the (cloud) computing world.

Full disclosure: I love to build things in Go and I thought this would be a great opportunity to build something that not only I would find useful.

## Development

For development setup, workflow, and contributing guidelines, see [CONTRIBUTING](CONTRIBUTING.md) and [CODE OF CONDUCT](CODE_OF_CONDUCT.md).
