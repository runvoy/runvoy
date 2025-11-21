<h1 align="center">
    <p><strong>🚀 Runvoy</strong></p>
    <p>self-hosted serverless command runner</p>
</h1>
<p align="center">
    <em>Run arbitrary commands on remote ephemeral containers — no complex setup required.</em>
</p>
<p align="center">
  <a href="https://github.com/runvoy/runvoy/actions/workflows/tests-and-coverage-go.yml" target="_blank">
      <img src="https://github.com/runvoy/runvoy/actions/workflows/tests-and-coverage-go.yml/badge.svg?event=push&branch=main" alt="Tests (Go)">
  </a>
  <a href="https://github.com/runvoy/runvoy/actions/workflows/tests-and-coverage-svelte.yml" target="_blank">
      <img src="https://github.com/runvoy/runvoy/actions/workflows/tests-and-coverage-svelte.yml/badge.svg?event=push&branch=main" alt="Tests (Svelte)">
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

**Deploy once, issue API keys, and let your team run arbitrary (admin) applications safely from their terminals.** Share playbooks to perform common tasks consistently and reliably.

Workstations shouldn't need complex setups. Let remote containers execute commands in a secured and reproducible production-grade environment.

**No more snowflakes, _run envoys_.** ✨

## 🎯 Use cases

- ☁️ **Cloud CLI operations** — AWS CLI, Terraform, or any SDK-based tools in remote containers with proper permissions ([AWS CLI example](.runvoy/aws-cli-example.yml))
- ⚙️ **One-off commands** — Run arbitrary commands like `kubectl run` without maintaining an always-running cluster. Example: `runvoy run ping <my service ip>`
- 🏗️ **Resource-intensive tasks** — Builds, test runners and any other heavy workload which require a specific instance type. Tail and share logs in real-time like GitHub Actions ([Build Caddy example](.runvoy/build-caddy-example.yml))
- 📝 **Audit-required operations** — Any command that needs a complete audit trail with user identification
- 🔐 **Secure operations** — Execute commands with secrets without exposing them to local workstations

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

## 💡 What is Runvoy?

**Runvoy is composed of 3 main parts** (see [Architecture](#%EF%B8%8F-architecture) for details):

- 🖥️ **Backend** — Runs on your AWS account (multi-cloud support planned), exposes the HTTP API, and orchestrates cloud resources. Deploy once as a cloud admin.
- ⌨️ **CLI client** — The `runvoy` command-line tool for interacting with the REST API
- 🌐 **Web app** — Visit [runvoy.site](https://runvoy.site) or self-host. Currently supports log viewing with full CLI parity coming soon.

## ✨ Key Benefits

- 🖥️ **_Doesn't_ run on your computer** — Commands execute in remote production-grade environments with proper secrets access. Team members only need the `runvoy` CLI and an API key — no complex workstation setup required.

- 📊 **Complete audit trail** — Every backend interaction is logged with user identification. All logs stored in append-only database for compliance (CloudWatch Logs supported, more providers coming).

- 🔓 **Self-hosted, no black magic** — The backend runs in _your_ cloud account. You control everything: policies, permissions, and data.

- 💰 **Serverless** — No always-running services. Pay only for the compute your commands consume (essentially free for infrequent use).

## 🎨 Features

- 🔑 **API key authentication** — Secure access with SHA-256 hashed API keys
- 👥 **User access management** — Role-based and ownership access control. Admins define permissions; users access only what they're allowed to
- 🐳 **Customizable container roles** — Register Docker images with custom IAM roles for proper AWS resource access (ECS support, more coming soon)
- 📋 **Native cloud logging** — Full execution logs and audit trails with request ID tracking
- 📖 **Reusable playbooks** — Store command configs in YAML, commit them, and share with your team for consistent execution ([Terraform example](.runvoy/terraform-example.yml))
- 🔐 **Secrets management** — Centralized encrypted secrets with full CRUD operations from the CLI
- ⚡️ **Real-time WebSocket streaming** — Live logs delivered to CLI and web viewer via authenticated WebSocket connections
- 🔗 **Automatic Git cloning** — Clone private Git repos directly into container working directory ([Build Caddy example](.runvoy/build-caddy-example.yml))
- 🔧 **Unix-style output streams** — Separate CLI logs (stderr) from data (stdout) for easy piping and scripting
- 🏗️ **IaC deployment** — Deploy complete backend infrastructure with CloudFormation (multi-cloud support coming)
- 📦 **Single binary** — Download one ~6MB binary and run it. No dependencies, no installation hassle.

### 🚧 Roadmap

- 🌍 **Multi-cloud support** — GCP, Azure...
- 📡 **Robust log streaming** — More reliable streaming mechanism (current implementation is lossy)
- ⏱️ **Execution timeouts** — Automatic SIGTERM for commands exceeding timeout
- 🔒 **Lock management** — Prevent concurrent execution conflicts
- 🌐 **Full webapp parity** — All CLI commands available in the web interface
- 🍺 **Homebrew support** — Native installation via Homebrew package manager

## ⚡️ Quick Start

Download the latest release from the [releases page](https://github.com/runvoy/runvoy/releases):

<!-- VERSION_EXAMPLES_START -->
- **Linux example:**

```bash
curl -L -o runvoy-cli-linux-arm64.tar.gz https://github.com/runvoy/runvoy/releases/download/v0.2.0/runvoy_linux_amd64.tar.gz
tar -xzf runvoy_linux_amd64.tar.gz
sudo mv runvoy_linux_amd64/runvoy /usr/local/bin/runvoy
```

- **macOS example:**

```bash
curl -L -o runvoy_linux_amd64.tar.gz https://github.com/runvoy/runvoy/releases/download/v0.2.0/runvoy_darwin_arm64.tar.gz
tar -xzf runvoy_darwin_arm64.tar.gz
xattr -dr com.apple.quarantine runvoy_darwin_arm64/runvoy
codesign -s - --deep --force runvoy_darwin_arm64/runvoy
sudo mv runvoy_darwin_arm64/runvoy /usr/local/bin/runvoy
```

<!-- VERSION_EXAMPLES_END -->

### 🏗️ Deploying the backend infrastructure

**Requirements:**

- AWS credentials configured in your shell environment ([AWS credentials docs](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html))

Bootstrap the backend infrastructure and seed the admin user:

```bash
runvoy infra apply --configure --region eu-west-1 --seed-admin-user admin@example.com
```

#### 👤 Creating a new user

The admin API key and endpoint are automatically configured in `~/.runvoy/config.yaml` after deployment. Start using runvoy immediately:

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

## 📖 Usage

<!-- CLI_HELP_START -->
### Available Commands

To see all available commands and their descriptions:

```bash
runvoy --help
```

```text
runvoy - v0.2.0-20251121-03ca77f
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

### 🔧 Output Streams and Piping

Runvoy follows Unix conventions by separating informational messages from data output, making it easy to pipe commands and script automation workflows:

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

### 🌐 Web Viewer

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

## 🏛️ Architecture

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

## 💭 The Story Behind Runvoy

As a software engineer, I frequently found myself needing to run admin commands — and more importantly, **enabling my team to run those same commands without becoming a bottleneck**. 🚧

I've always been fascinated by "thin clients" — delegating heavy lifting and security concerns to the server while keeping the client simple. Just "log in" and run the commands you're authorized to run. The server handles the rest. ✨

The final piece fell into place with AWS Lambda and the _serverless_ revolution: **"you only pay for what you use"** is now a commodity in cloud computing. 💰

**Full disclosure:** I love building applications in Go, and this felt like the perfect opportunity to create something genuinely useful — not just for me and my colleagues, but for teams everywhere. 🛠️

## 🤝 Development

For development setup, workflow, and contributing guidelines, see [CONTRIBUTING](CONTRIBUTING.md) and [CODE OF CONDUCT](CODE_OF_CONDUCT.md).

Contributions welcome! 🎉
