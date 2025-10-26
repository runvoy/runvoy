# mycli - System Architecture & Implementation

Complete technical documentation for the mycli remote command execution platform.

## Table of Contents

1. [Overview & Design Philosophy](#overview--design-philosophy)
2. [Architecture](#architecture)
3. [Key Components](#key-components)
4. [Execution Flow](#execution-flow)
5. [Configuration System](#configuration-system)
6. [CLI Commands Reference](#cli-commands-reference)
7. [API Contract](#api-contract)
8. [Security Model](#security-model)
9. [Shell Command Construction](#shell-command-construction)
10. [Cost Analysis](#cost-analysis)
11. [Deployment & Testing](#deployment--testing)
12. [Limitations & Trade-offs](#limitations--trade-offs)
13. [Future Enhancements](#future-enhancements)
14. [Troubleshooting](#troubleshooting)

---

## Overview & Design Philosophy

### Project Vision

A CLI tool that provides isolated, repeatable execution environments for commands. Solves the problem of running infrastructure-as-code tools (Terraform, Ansible, etc.) and other CLI applications without local execution issues like race conditions, credential sharing, or dependency conflicts.

**Key principle:** General purpose remote execution, not tool-specific. Users can run any command in a containerized environment.

### Design Decisions

**Simplicity over custom solutions:**
- ✅ Use standard Docker images (no custom containers to maintain)
- ✅ Dynamic shell command construction (no custom entrypoints)
- ✅ Git as source of truth (no S3 code storage)
- ✅ Direct ECS/CloudWatch API usage (no DynamoDB for MVP)

**Target Use Cases:**
- Infrastructure as Code execution (Terraform, Ansible, Pulumi)
- CI/CD workflows
- Scripts requiring AWS credentials
- Any command needing isolation and audit trails

---

## Architecture

### System Diagram

```
┌─────────────┐
│  CLI User   │ mycli exec "terraform plan"
└──────┬──────┘
       │ HTTPS + API Key
       ▼
┌──────────────────┐
│  API Gateway     │ REST API endpoint
│  /execute        │
└──────┬───────────┘
       │
       ▼
┌─────────────────────────────────────────────────────┐
│  Lambda Orchestrator (Go)                           │
│  - Validates API key (bcrypt)                       │
│  - Constructs shell script:                         │
│    * Install git (if needed)                        │
│    * Configure git credentials                      │
│    * Clone repository                               │
│    * Execute user command                           │
│  - Starts ECS Fargate task with command override   │
└──────┬──────────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────────┐
│  ECS Fargate Task                                    │
│  - Uses any Docker image (terraform, python, etc.)  │
│  - Executes constructed shell command                │
│  - Logs output to CloudWatch                        │
│  - Exits with command's exit code                   │
└──────────────────────────────────────────────────────┘
       │
       ▼
┌──────────────────┐
│  CloudWatch Logs │
│  Execution logs  │
└──────────────────┘
```

### AWS Services Used

**Compute & Orchestration:**
- **API Gateway** - REST API endpoint for CLI requests
- **Lambda** - Orchestrator (validates, constructs commands, starts tasks)
- **ECS Fargate** - Serverless containers for command execution

**Storage:**
- **CloudWatch Logs** - Execution output (stdout/stderr)
- No S3 needed - Git is source of truth

**Networking:**
- **VPC** - Isolated network for Fargate tasks
- **Public Subnets** - For Git repository access (no NAT Gateway cost)
- **Security Groups** - Egress-only (no inbound traffic)

**Authentication:**
- **Lambda Environment Variables** - API key hash and Git credentials (encrypted at rest)

---

## Key Components

### 1. Lambda Orchestrator

**Location:** `lambda/orchestrator/`
**Language:** Go
**Runtime:** Custom runtime (provided.al2023, ARM64)

**Responsibilities:**
- API key authentication using bcrypt
- Request validation
- Shell command construction (git setup + clone + user command)
- Dynamic task definition registration (when custom image specified)
- ECS task orchestration with command override
- Task metadata management

**Key Files & Functions:**
- `main.go` - `handler()` entry point and routing
- `config.go` - AWS SDK clients and environment variable loading
- `auth.go` - `authenticate()` API key verification
- `handlers.go` - `handleExec()`, `handleStatus()`, `handleLogs()`
- `shell.go` - `buildShellCommand()`, `buildDirectCommand()`, `shellEscape()`
- `response.go` - `errorResponse()` helper
- `util.go` - `generateExecutionID()` - Short ID generation using timestamp + crypto/rand
- `types.go` - Shared API type aliases (`Request`, `Response`)

**Why Go?**
- Fast cold starts (~100ms)
- Small binary size (~10 MB)
- Strong typing for safety
- Excellent AWS SDK support

**Execution ID Generation:**

The system uses short, timestamp-based IDs for execution tracking:
- **Format:** `{timestamp_hex}{random_hex}` (12 characters, e.g., `67a1a8f4a3b5`)
- **Structure:** 8-character Unix timestamp (hex) + 4-character random suffix (hex)
- **Implementation:** Self-implemented using `crypto/rand` from Go standard library (no external dependencies)
- **Why this format?**
  - Short and URL-friendly: 12 chars vs 36 chars for UUID (66% shorter)
  - Collision-resistant: 2 random bytes provide ~65k combinations per second
  - Time-ordered: Timestamp prefix enables efficient date-based queries in future
  - Unpredictable: Cryptographically secure random generation
  - No external dependencies: ~15 lines of code using only standard library
  - Database-ready: Can delegate uniqueness to DynamoDB primary key when needed
- **Future Enhancement:** Entropy level will be configurable via `mycli init` command

### 2. CLI Application

**Language:** Go
**Framework:** Cobra (command structure)

**Key Files:**
- `cmd/init.go` - Infrastructure deployment, builds Lambda, configures Git credentials (cmd/init.go:61)
- `cmd/exec.go` - Command execution with config priority resolution (cmd/exec.go:65)
- `cmd/status.go` - Task status checking
- `cmd/logs.go` - Log viewing with follow mode
- `cmd/configure.go` - Manual configuration
- `cmd/destroy.go` - Infrastructure cleanup
- `internal/config/config.go` - Global config management (~/.mycli/config.yaml)
- `internal/project/config.go` - Project config parser (.mycli.yaml)
- `internal/git/detector.go` - Git remote URL auto-detection
- `internal/api/client.go` - API Gateway HTTP client

### 3. Container Execution Model

**NO custom Docker image required!**

Users can specify **any** Docker image:
- `hashicorp/terraform:1.6` - For Terraform
- `python:3.11` - For Python scripts
- `node:18` - For Node.js
- `ubuntu:22.04` - Generic fallback
- Any public or private Docker image

**How it works:**
1. Lambda constructs a shell script with git setup + clone + user command
2. If a custom image is specified via `--image` flag:
   - Lambda dynamically registers a new task definition with that image (or reuses existing one)
   - Task definition family name: Human-readable based on image name (e.g., `mycli-task-alpine-latest`, `mycli-task-terraform-1-6`)
   - Collision detection: If a task definition with that name exists but uses a different image, falls back to hash-based naming
   - Perfect caching: Same image always reuses the same task definition
3. ECS task runs with the appropriate task definition
4. Container executes the shell script which:
   - Installs git if needed (apt-get, apk, or yum)
   - Configures credentials
   - Clones repo to `/workspace/repo`
   - Runs user's command in that directory
   - Exits with the command's exit code

**Task Definition Naming Strategy:**

The Lambda orchestrator creates human-readable task definition family names for easy identification and caching:

**Naming Examples:**
- `alpine:latest` → `mycli-task-alpine-latest`
- `ubuntu:22.04` → `mycli-task-ubuntu-22-04`
- `hashicorp/terraform:1.6` → `mycli-task-hashicorp-terraform-1-6`
- `python:3.11-slim` → `mycli-task-python-3-11-slim`

**Sanitization Rules:**
- Replace `:`, `/`, `.`, `@`, `_` with `-`
- Convert to lowercase
- Truncate to 50 characters (ECS family name limit is 255)
- Trim trailing hyphens

**Collision Detection:**

When a task definition family already exists, the Lambda checks if it uses the same image:
1. **Match found**: Reuses existing task definition (perfect caching!)
2. **Collision detected**: Different image, same sanitized name
   - Falls back to hash-based naming: `mycli-task-{sanitized}-{8-char-hash}`
   - Example: If both `alpine:latest` and `alpine/latest` exist
   - First gets: `mycli-task-alpine-latest`
   - Second gets: `mycli-task-alpine-latest-a1b2c3d4`

**Benefits:**
- ✅ Easy to identify which images have cached task definitions
- ✅ Simple to search/filter in AWS Console: "mycli-task-terraform*"
- ✅ Perfect caching: same image always reuses same definition
- ✅ Helps with debugging and cost analysis
- ✅ No unnecessary task definition proliferation

### 4. CloudFormation Infrastructure

**Template:** `deploy/cloudformation.yaml`

**Resources Created:**
- **VPC** - 10.0.0.0/16 with DNS support
- **Internet Gateway** - For public subnet internet access
- **Public Subnets (2)** - Multi-AZ for high availability
- **Route Table** - Routes traffic to internet gateway
- **Security Group** - Egress-only for Fargate tasks
- **ECS Cluster** - Fargate capacity provider (FARGATE_SPOT default)
- **Task Definition** - Template task (image/command overridden at runtime)
- **CloudWatch Log Group** - /aws/mycli/{ProjectName}, 7-day retention
- **IAM Roles:**
  - Task Execution Role - Pull images, write logs
  - Task Role - Runtime permissions (minimal by default, user-configurable)
  - Lambda Execution Role - Start tasks, read logs, update function config
- **Lambda Function** - Created with placeholder code, updated by init command
- **API Gateway** - REST API with /execute resource, POST method, Lambda integration, and prod deployment

**Parameters:**
- `APIKeyHash` - Bcrypt hash of API key (NoEcho)
- `GitHubToken` - GitHub PAT (NoEcho, optional)
- `GitLabToken` - GitLab PAT (NoEcho, optional)
- `SSHPrivateKey` - Base64-encoded SSH key (NoEcho, optional)
- `DefaultImage` - Template image (default: ubuntu:22.04)

---

## Execution Flow

### Standard Git-based Execution

```
1. User runs: mycli exec --repo=https://github.com/user/infra "terraform apply"

2. CLI (cmd/exec.go:65):
   - Loads global config from ~/.mycli/config.yaml
   - Builds execution config (flags > .mycli.yaml > git auto-detect)
   - Validates configuration
   - Sends POST request to API Gateway

3. API Gateway:
   - Routes to Lambda function
   - Passes request body and headers

4. Lambda (`lambda/orchestrator/main.go`):
   - Authenticates API key via bcrypt
   - Validates request (repo, command required)
   - Constructs shell script (`lambda/orchestrator/shell.go`):
     * Install git if not present
     * Configure GitHub/GitLab/SSH credentials
     * Clone repo: git clone --depth 1 --branch main <repo> /workspace/repo
     * cd /workspace/repo
     * Execute user command
     * Cleanup credentials
   - If custom image specified (`lambda/orchestrator/handlers.go:getOrCreateTaskDefinition()`):
     * Creates human-readable family name by sanitizing image: `mycli-task-{sanitized-image}`
       - Example: `alpine:latest` → `mycli-task-alpine-latest`
       - Example: `hashicorp/terraform:1.6` → `mycli-task-hashicorp-terraform-1-6`
     * Checks if task definition already exists with this family name
     * If exists, verifies the image matches (perfect caching)
     * If collision detected (same name, different image), falls back to: `mycli-task-{sanitized-image}-{hash}`
     * If not exists, registers new task definition with custom image
     * Uses base task definition as template (CPU, memory, roles, etc.)
   - Starts ECS Fargate task with:
     * Task Definition: Custom or base task definition
     * Command: ["/bin/sh", "-c", "<script>"]
     * Environment: User-provided env vars
     * Tags: ExecutionID, Repo
   - Returns task ARN and execution ID

5. Fargate Task:
   - Downloads specified Docker image
   - Executes shell script
   - Logs all output to CloudWatch
   - Exits with command's exit code

6. User monitors:
   - mycli status <task-arn> - Check task status
   - mycli logs <execution-id> - View logs
   - mycli logs -f <execution-id> - Follow logs
```

### Direct Execution (--skip-git mode)

```
1. User runs: mycli exec --skip-git --image=alpine:latest "echo hello"

2. Lambda (`lambda/orchestrator/main.go`):
   - Skips git credential setup
   - Constructs simpler script (`lambda/orchestrator/shell.go`):
     * Echo execution header
     * Execute user command directly
     * Report exit code
   - Starts ECS task without git cloning

3. Fargate Task:
   - Runs command in container root directory
   - No repository cloning
   - Faster startup
```

---

## Configuration System

### Three-Level Priority System

When executing `mycli exec "command"`, configuration is resolved in this order:

1. **Command-line flags** (highest priority)
   - `--repo`, `--branch`, `--image`, `--env`, `--timeout`, `--skip-git`

2. **`.mycli.yaml`** in current directory
   - Project-specific settings
   - Should be committed to version control

3. **Git remote auto-detection** (convenience)
   - Runs: `git remote get-url origin`
   - Detects current branch

4. **Error** if no repo specified (unless --skip-git)

### Global Configuration

**Location:** `~/.mycli/config.yaml`
**Permissions:** 0600 (read/write for owner only)
**Created by:** `mycli init` command

**Format:**
```yaml
api_endpoint: https://abc123.execute-api.us-east-1.amazonaws.com/prod/execute
api_key: sk_live_a1b2c3d4e5f6...
region: us-east-1
```

**Purpose:** User-level mycli configuration (API credentials, endpoint)

### Project Configuration

**Location:** `.mycli.yaml` in project directory
**Permissions:** Standard file (should be committed to git)
**Created by:** User (manually or via future `mycli init-project` command)

**Format:**
```yaml
# Repository to clone (required if not using --repo flag)
repo: https://github.com/mycompany/infrastructure

# Branch to checkout (optional, default: main)
branch: main

# Docker image to use (optional, default: ubuntu:22.04)
image: hashicorp/terraform:1.6

# Environment variables (optional)
env:
  TF_VAR_environment: production
  TF_VAR_region: us-east-1
  AWS_REGION: us-east-1

# Timeout in seconds (optional, default: 1800)
timeout: 3600
```

**Examples:**

Terraform project:
```yaml
repo: https://github.com/company/aws-infrastructure
image: hashicorp/terraform:1.6
env:
  TF_VAR_region: us-east-1
  TF_VAR_environment: production
timeout: 3600
```

Ansible project:
```yaml
repo: https://github.com/company/ansible-playbooks
image: ansible/ansible:latest
env:
  ANSIBLE_HOST_KEY_CHECKING: "False"
```

Multi-environment pattern:
```yaml
# .mycli.yaml - base config
repo: https://github.com/company/infra
# Use --branch flag for different environments
```

Usage:
```bash
mycli exec --branch=dev "terraform apply"   # dev environment
mycli exec --branch=prod "terraform apply"  # prod environment
```

---

## CLI Commands Reference

### `mycli init`

**Purpose:** One-command infrastructure setup

**What it does:**
1. Loads AWS config (region from flag or AWS config or default us-east-2)
2. Shows confirmation prompt with region and resources to be created (unless --force)
3. Generates API key: `sk_live_{64_hex_chars}`
4. Hashes key with bcrypt (cost 10)
5. Prompts for Git credentials (optional, interactive)
6. Builds Lambda function (Go cross-compile for linux/arm64)
7. Creates CloudFormation stack with all resources (including Lambda with placeholder code and API Gateway)
8. Waits for stack creation (~5 minutes)
9. Updates Lambda function code with built zip
10. Saves config to ~/.mycli/config.yaml
11. Displays API key (shown once, also saved to config)

**Flags:**
- `--stack-name string` - CloudFormation stack name (default: "mycli")
- `--region string` - AWS region (default: from AWS config or us-east-2)
- `--force` - Skip confirmation prompt

**Git Credential Setup (interactive):**
```
→ Git Credential Configuration
  For private repositories, you can configure Git authentication.
  This is optional - you can skip this and only use public repos.

Configure Git credentials? [y/N]: y

Choose authentication method:
  1) GitHub Personal Access Token (recommended)
  2) GitLab Personal Access Token
  3) SSH Private Key (for any Git provider)
  4) Skip

Selection [1-4]: 1
Enter GitHub token (ghp_...): ghp_xxxxx
  ✓ GitHub token configured
```

**Output:**
```
🚀 Initializing mycli infrastructure...
   Stack name: mycli
   Region: us-east-1

⚠️  This will create AWS infrastructure in your account:
   Stack Name: mycli
   Region:     us-east-1

Resources to be created:
   - VPC with subnets and internet gateway
   - ECS Fargate cluster and task definitions
   - Lambda function and API Gateway
   - CloudWatch log groups
   - IAM roles and security groups

Type 'yes' to confirm: yes

→ Generating API key...
→ Building Lambda function...
→ Creating CloudFormation stack...
  Waiting for stack creation (this may take a few minutes)...
✓ Stack created successfully
→ Updating Lambda function code...
✓ Lambda function code updated
→ Saving configuration...

✅ Setup complete!
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Configuration saved to ~/.mycli/config.yaml
  API Endpoint: https://abc123.execute-api.us-east-1.amazonaws.com/prod/execute
  Region:       us-east-1
  GitHub Auth:  ✓ Configured
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔑 Your API key: sk_live_a1b2c3d4e5f6...
   (Also saved to config file)

Next steps:
  1. Test it: mycli exec --repo=https://github.com/user/repo "echo hello"
```

### `mycli exec [flags] "command"`

**Purpose:** Execute command remotely

**Flags:**
- `--repo string` - Git repository URL (overrides .mycli.yaml and git remote)
- `--branch string` - Git branch to checkout (overrides .mycli.yaml)
- `--image string` - Docker image to use (overrides .mycli.yaml)
- `--env stringArray` - Environment variables KEY=VALUE (merges with .mycli.yaml, repeatable)
- `--timeout int` - Timeout in seconds (overrides .mycli.yaml)
- `--skip-git` - Skip git cloning and run command directly in container

**Configuration Priority:** CLI flags > .mycli.yaml > Git auto-detect > Error

**Examples:**

With .mycli.yaml (simplest):
```bash
cd my-terraform-project  # has .mycli.yaml
mycli exec "terraform plan"
# Uses repo, branch, image from .mycli.yaml
```

Override specific settings:
```bash
mycli exec --branch=dev "terraform plan"
mycli exec --env TF_VAR_region=us-west-2 "terraform apply"
```

Without .mycli.yaml (explicit):
```bash
mycli exec --repo=https://github.com/user/infra "terraform apply"
mycli exec --repo=https://github.com/user/infra --branch=dev "terraform plan"
```

Auto-detect from git remote:
```bash
cd my-git-repo  # no .mycli.yaml, but has git remote
mycli exec "make deploy"
# Automatically uses: git remote get-url origin
```

Skip git cloning:
```bash
mycli exec --skip-git --image=alpine:latest "echo hello world"
# No repository cloning, runs command directly
```

**Output:**
```bash
$ mycli exec "terraform apply"
→ Loaded configuration from .mycli.yaml
→ Repository: https://github.com/user/infra
→ Branch: main
→ Image: hashicorp/terraform:1.6
→ Command: terraform apply

→ Starting execution...
✓ Execution started

Execution Details:
  Execution ID: 67a1a8f4a3b5
  Task ARN:     arn:aws:ecs:us-east-1:123456789:task/mycli-cluster/abc123def456
  Log Stream:   task/executor/abc123def456

Monitor execution:
  mycli status arn:aws:ecs:us-east-1:123456789:task/mycli-cluster/abc123def456
  mycli logs arn:aws:ecs:us-east-1:123456789:task/mycli-cluster/abc123def456
  mycli logs -f arn:aws:ecs:us-east-1:123456789:task/mycli-cluster/abc123def456  # Follow logs in real-time
```

### `mycli status <task-arn>`

**Purpose:** Check execution status

**What it does:**
1. Sends API request with action="status"
2. Lambda queries ECS DescribeTasks API
3. Returns task status, desired status, timestamps

**Output:**
```
Status:         RUNNING
Desired Status: RUNNING
Created At:     2025-01-26T14:32:10Z
```

Possible statuses:
- `PROVISIONING` - Container being provisioned
- `PENDING` - Waiting for resources
- `RUNNING` - Command executing
- `DEPROVISIONING` - Task shutting down
- `STOPPED` - Task completed (check logs for exit code)

### `mycli logs <task-arn>`

**Purpose:** View execution logs

**Flags:**
- `-f, --follow` - Stream logs in real-time (polls every 2 seconds)

**What it does:**
1. Sends API request with action="logs" and task ARN
2. Lambda extracts task ID from the ARN
3. Constructs log stream name: `task/executor/{task-id}`
4. Queries CloudWatch Logs for the specific log stream
5. Returns log events with timestamps in format: `YYYY-MM-DD HH:MM:SS UTC | message`

**Design Note:** Uses task ARN directly instead of execution ID for simplicity. This avoids the need to list all ECS tasks and search through tags, or maintain a DynamoDB table for ExecutionID → TaskARN mapping. The task ARN is provided in the output of `mycli exec` command.

**Output:**
```bash
$ mycli logs arn:aws:ecs:us-east-1:123456789:task/mycli-cluster/abc123def456

Logs for task: arn:aws:ecs:us-east-1:123456789:task/mycli-cluster/abc123def456
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
2025-10-26 14:32:10 UTC | ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
2025-10-26 14:32:10 UTC | mycli Remote Execution
2025-10-26 14:32:10 UTC | ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
2025-10-26 14:32:11 UTC | → Configuring GitHub authentication...
2025-10-26 14:32:11 UTC | → Repository: https://github.com/user/infra
2025-10-26 14:32:11 UTC | → Branch: main
2025-10-26 14:32:11 UTC | → Cloning repository...
2025-10-26 14:32:15 UTC | ✓ Repository cloned
2025-10-26 14:32:15 UTC |
2025-10-26 14:32:15 UTC | ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
2025-10-26 14:32:15 UTC | Executing command...
2025-10-26 14:32:15 UTC | ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
2025-10-26 14:32:15 UTC |
2025-10-26 14:32:16 UTC | [terraform output here...]
2025-10-26 14:35:42 UTC |
2025-10-26 14:35:42 UTC | ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
2025-10-26 14:35:42 UTC | ✓ Command completed successfully
2025-10-26 14:35:42 UTC | ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

**Timestamp Format:** Each log line is prefixed with a timestamp showing when the event occurred in UTC timezone.

Follow mode:
```bash
$ mycli logs -f arn:aws:ecs:us-east-1:123456789:task/mycli-cluster/abc123def456
# Streams logs in real-time until task completes
# Each line includes timestamp for accurate event tracking
```

### `mycli configure`

**Purpose:** Manual configuration or reconfiguration

**What it does:**
1. Discovers existing CloudFormation stack
2. Extracts outputs (API endpoint, region)
3. Prompts for API key (not in stack outputs)
4. Saves to ~/.mycli/config.yaml

**Use cases:**
- Lost config file
- Multiple environments
- Deployed infrastructure manually

### `mycli destroy`

**Purpose:** Clean up all infrastructure

**Flags:**
- `--stack-name string` - Stack to delete (default: "mycli")
- `--region string` - AWS region (default: from config or AWS profile)
- `--force` - Skip confirmation prompt
- `--keep-config` - Keep local config file after destruction

**What it does:**
1. Confirms with user (unless --force)
2. Empties S3 bucket (required before CloudFormation deletion)
3. Deletes all ECS task definitions with mycli prefix (both active and inactive, dynamically created)
   - First deregisters all ACTIVE task definitions
   - Then deletes all INACTIVE task definitions (including newly deregistered ones)
4. Deletes Lambda function (created outside CloudFormation)
5. Deletes CloudFormation stack (cascades to all resources)
6. Waits for deletion to complete
7. Removes local config file (unless --keep-config)

**Example:**
```bash
$ mycli destroy
⚠️  This will delete all mycli infrastructure in your AWS account.
   Stack: mycli
   Region: us-east-1

Continue? [y/N]: y

→ Emptying S3 bucket...
✓ Bucket emptied
→ Deleting ECS task definitions...
  Collecting task definitions...
  Found 3 active and 2 inactive mycli task definitions across 5 families
  Deregistering active task definitions...
  Deregistered: arn:aws:ecs:us-east-1:123456789:task-definition/mycli-task:3
  Deregistered: arn:aws:ecs:us-east-1:123456789:task-definition/mycli-task:2
  Deleting inactive task definitions...
  Deleted: arn:aws:ecs:us-east-1:123456789:task-definition/mycli-task:1
✓ Deleted 3 task definitions
→ Deleting Lambda function...
✓ Lambda function deleted
→ Deleting CloudFormation stack...
  Waiting for stack deletion...
✓ Stack deleted successfully
→ Removing local configuration...
✓ Config removed

✅ Destruction complete!
   All AWS resources have been removed.
```

---

## API Contract

### Unified API Design

All actions use POST to `/execute` endpoint with `action` field in request body.

### Authentication

**Header:** `X-API-Key: sk_live_...`
**Validation:** Bcrypt comparison in Lambda (`lambda/orchestrator/auth.go`)

### POST /execute - Exec Action

**Request:**
```json
{
  "action": "exec",
  "repo": "https://github.com/user/infrastructure",
  "branch": "main",
  "command": "terraform apply -auto-approve",
  "image": "hashicorp/terraform:1.6",
  "env": {
    "TF_VAR_region": "us-east-1",
    "AWS_REGION": "us-east-1"
  },
  "timeout_seconds": 1800,
  "skip_git": false
}
```

**Response:**
```json
{
  "execution_id": "67a1a8f4a3b5",
  "task_arn": "arn:aws:ecs:us-east-1:123456789:task/mycli-cluster/abc123",
  "status": "starting",
  "log_stream": "task/abc123",
  "created_at": "2025-01-26T14:32:10Z"
}
```

**Validation:**
- `command` required
- `repo` required unless `skip_git` is true
- `branch` defaults to "main"
- `timeout_seconds` defaults to 1800
- `env` optional, merged with task environment

### POST /execute - Status Action

**Request:**
```json
{
  "action": "status",
  "task_arn": "arn:aws:ecs:us-east-1:123456789:task/mycli-cluster/abc123"
}
```

**Response:**
```json
{
  "status": "RUNNING",
  "desired_status": "RUNNING",
  "created_at": "2025-01-26T14:32:10Z"
}
```

### POST /execute - Logs Action

**Request:**
```json
{
  "action": "logs",
  "task_arn": "arn:aws:ecs:us-east-1:123456789:task/mycli-cluster/abc123def456"
}
```

**Response:**
```json
{
  "logs": "2025-10-26 14:32:10 UTC | mycli Remote Execution\n2025-10-26 14:32:11 UTC | → Cloning repository...\n..."
}
```

**Implementation Details:**
- Accepts task ARN directly (no ExecutionID lookup needed)
- Extracts the task ID from the task ARN (last 36 characters)
- Constructs log stream name: `task/executor/{task-id}` (includes container name)
- Checks if log stream exists first using DescribeLogStreams
- Queries CloudWatch Logs FilterLogEvents for the specific log stream
- Each log line includes a timestamp prefix: `YYYY-MM-DD HH:MM:SS UTC | message`
- Returns up to 1000 log events (sorted chronologically)

**Design Decision:** Using task ARN instead of execution ID simplifies implementation. Alternative approaches would require either:
1. Listing all ECS tasks and searching through tags (inefficient at scale)
2. Maintaining a DynamoDB table for ExecutionID → TaskARN mapping (additional infrastructure)
For MVP simplicity, we use the task ARN directly, which is already provided in the `exec` command output.

### Error Responses

**401 Unauthorized:**
```json
{
  "error": "unauthorized"
}
```

**400 Bad Request:**
```json
{
  "error": "invalid request: repo required"
}
```

**500 Internal Server Error:**
```json
{
  "error": "failed to run task: [error details]"
}
```

---

## Security Model

### API Authentication

**Key Format:** `sk_live_{64_hex_chars}`
**Generation:** 32 random bytes, hex-encoded, prefixed (cmd/init.go:99)
**Storage:**
- Lambda: bcrypt hash (cost 10) in environment variable `API_KEY_HASH`
- CLI: plaintext in ~/.mycli/config.yaml (permissions 0600)

**Validation Flow:**
1. CLI sends API key in `X-API-Key` header
2. Lambda reads hash from environment
3. bcrypt.CompareHashAndPassword(hash, key)
4. Returns 401 if invalid

### Git Credentials

**Supported Methods:**

1. **GitHub Personal Access Token**
   - Scope: `repo` (full control of private repositories)
   - Format: `ghp_xxxxxxxxxxxx`
   - Storage: Lambda environment variable `GITHUB_TOKEN`
   - Usage: `https://{token}:x-oauth-basic@github.com`

2. **GitLab Personal Access Token**
   - Scope: `read_repository`, `write_repository`
   - Storage: Lambda environment variable `GITLAB_TOKEN`
   - Usage: `https://oauth2:{token}@gitlab.com`

3. **SSH Private Key**
   - Base64-encoded for environment variable storage
   - Storage: Lambda environment variable `SSH_PRIVATE_KEY`
   - Usage: Decoded to ~/.ssh/id_rsa in container

**Security Properties:**
- Encrypted at rest by AWS (Lambda environment variables)
- Never logged to CloudWatch
- Never exposed in API responses or CloudFormation outputs
- Written to container filesystem with 0600 permissions
- Deleted before container exits (`lambda/orchestrator/shell.go`)
- Transmitted only to container via shell script

**Rotation:**
1. Update CloudFormation stack parameters
2. Or update Lambda environment variables directly
3. No CLI changes needed

### Network Isolation

**VPC Configuration:**
- CIDR: 10.0.0.0/16
- Public subnets: 10.0.1.0/24, 10.0.2.0/24 (multi-AZ)
- Internet Gateway for Git repository access
- No NAT Gateway (cost optimization)

**Security Group:**
- Egress: All traffic allowed (0.0.0.0/0) - needed for Git clone, package downloads
- Ingress: None - tasks don't accept inbound connections

**Container Isolation:**
- Each task runs in isolated Fargate container
- No shared filesystem or network
- Ephemeral - destroyed after execution
- Public IP assigned (for internet access) but no inbound traffic allowed

### IAM Roles

**Lambda Execution Role (mycli-lambda-role):**
```yaml
Permissions:
  - AWSLambdaBasicExecutionRole (managed policy)
  - ecs:RunTask
  - ecs:DescribeTasks
  - ecs:DescribeTaskDefinition
  - ecs:RegisterTaskDefinition
  - ecs:ListTasks
  - ecs:TagResource
  - iam:PassRole (for TaskExecutionRole and TaskRole)
  - logs:GetLogEvents
  - logs:FilterLogEvents
  - logs:DescribeLogStreams
```

**ECS Task Execution Role (mycli-task-execution-role):**
```yaml
Permissions:
  - AmazonECSTaskExecutionRolePolicy (managed policy)
    - ecr:GetAuthorizationToken
    - ecr:BatchCheckLayerAvailability
    - ecr:GetDownloadUrlForLayer
    - ecr:BatchGetImage
    - logs:CreateLogStream
    - logs:PutLogEvents
```

**ECS Task Role (mycli-task-role):**
```yaml
Permissions:
  - logs:CreateLogStream
  - logs:PutLogEvents (to execution log group)

# Users can attach additional policies for AWS operations
# Example: AdministratorAccess for Terraform
# Production: Use least-privilege policies
```

### Audit Trail

**What's Logged:**
- All command executions (stdout/stderr) to CloudWatch
- Task start/stop events
- API Gateway access logs (can be enabled)
- Lambda invocation logs

**What's NOT Logged:**
- API keys (only hashed values)
- Git credentials (only presence indicated)
- Environment variable values (keys logged, not values)

**Retention:**
- CloudWatch Logs: 7 days default (configurable in CloudFormation)
- Can archive to S3 for long-term retention

---

## Shell Command Construction

### Design Decision: Bash Script Approach

**Current Implementation:** Dynamic bash script construction in Lambda (`lambda/orchestrator/shell.go`)

**Why Bash?**
- ✅ Works with any image that has `/bin/sh` (universal)
- ✅ Simple to understand and debug
- ✅ No compilation or build step
- ✅ Easy to modify in Lambda code
- ✅ Human-readable in CloudWatch logs
- ✅ No binary injection complexity

**Security:** Proper shell escaping via `shellEscape()` function (`lambda/orchestrator/shell.go`)
- Single quotes wrap all user input
- Embedded single quotes escaped as `'\''`
- Prevents command injection

### Standard Git-based Script

**Generated by:** `buildShellCommand()` (`lambda/orchestrator/shell.go`)

**Script Structure:**
```bash
#!/bin/sh
set -e  # Exit on error

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "mycli Remote Execution"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# 1. Install git if not present
if ! command -v git &> /dev/null; then
  echo "→ Installing git..."
  if command -v apk &> /dev/null; then
    apk add --no-cache git openssh-client
  elif command -v apt-get &> /dev/null; then
    apt-get update && apt-get install -y git openssh-client
  elif command -v yum &> /dev/null; then
    yum install -y git openssh-clients
  else
    echo "ERROR: Cannot install git - unsupported package manager"
    exit 1
  fi
fi

# 2. Configure git credentials (GitHub example)
echo "→ Configuring GitHub authentication..."
git config --global credential.helper store
echo "https://'<token>':x-oauth-basic@github.com" > ~/.git-credentials
chmod 600 ~/.git-credentials

# 3. Clone repository
echo "→ Repository: <repo>"
echo "→ Branch: <branch>"
echo "→ Cloning repository..."
git clone --depth 1 --branch '<branch>' '<repo>' /workspace/repo || {
  echo "ERROR: Failed to clone repository"
  exit 1
}
cd /workspace/repo
echo "✓ Repository cloned"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Executing command..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 4. Execute user command (with proper escaping)
eval '<user-command>'

# 5. Capture exit code and cleanup
EXIT_CODE=$?
echo ""
if [ $EXIT_CODE -eq 0 ]; then
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "✓ Command completed successfully"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
else
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "✗ Command failed with exit code: $EXIT_CODE"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
fi
rm -f ~/.git-credentials ~/.ssh/id_rsa
exit $EXIT_CODE
```

### Direct Execution Script (--skip-git)

**Generated by:** `buildDirectCommand()` (`lambda/orchestrator/shell.go`)

**Script Structure:**
```bash
set -e
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "mycli Remote Execution (No Git)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "→ Mode: Direct command execution (git cloning skipped)"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Executing command..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

eval '<user-command>'

EXIT_CODE=$?
echo ""
if [ $EXIT_CODE -eq 0 ]; then
  echo "✓ Command completed successfully"
else
  echo "✗ Command failed with exit code: $EXIT_CODE"
fi
exit $EXIT_CODE
```

### Credential Configuration Variants

**GitHub Token:**
```bash
git config --global credential.helper store
echo "https://'$TOKEN':x-oauth-basic@github.com" > ~/.git-credentials
chmod 600 ~/.git-credentials
```

**GitLab Token:**
```bash
git config --global credential.helper store
echo "https://oauth2:'$TOKEN'@gitlab.com" > ~/.git-credentials
chmod 600 ~/.git-credentials
```

**SSH Key:**
```bash
mkdir -p ~/.ssh
echo '$SSH_PRIVATE_KEY' | base64 -d > ~/.ssh/id_rsa
chmod 600 ~/.ssh/id_rsa
ssh-keyscan github.com >> ~/.ssh/known_hosts 2>/dev/null
ssh-keyscan gitlab.com >> ~/.ssh/known_hosts 2>/dev/null
```

### Future Considerations

**Current approach is adequate for MVP**, but potential improvements:

**Option 1: Compiled Go Binary**
- Small Go binary injected into container
- Better error handling and type safety
- Faster (no git installation)
- More complex (need multi-arch compilation)

**Option 2: Python Script**
- Better structure than bash
- Most images have Python
- Not as universal as bash

**Recommendation:** Stick with bash unless we encounter serious limitations.

---

## Cost Analysis

### Per-Execution Cost Breakdown

**Assumptions:**
- 5-minute execution
- 0.25 vCPU, 0.5 GB memory
- FARGATE_SPOT pricing (us-east-1)
- Shallow git clone (~10 MB transfer)

**Costs:**

**Fargate SPOT:**
- vCPU: $0.01242124 per vCPU-hour
- Memory: $0.00136308 per GB-hour
- Duration: 5 min = 0.0833 hours
- Compute: (0.25 × $0.01242124 + 0.5 × $0.00136308) × 0.0833 = **$0.000315**

**Lambda:**
- Invocation: $0.0000002
- Duration: ~100ms, 128MB
- Negligible cost: **~$0.0000002**

**API Gateway:**
- Request: $0.0000035

**Data Transfer:**
- Git clone: ~10 MB × $0.09/GB = **$0.00001**

**Total per execution: ~$0.00033**

### Monthly Cost Estimates

| Executions | Compute | Fixed Costs | Total    |
|-----------|---------|-------------|----------|
| 100       | $0.03   | $0.00       | ~$0.03   |
| 1,000     | $0.33   | $0.00       | ~$0.33   |
| 10,000    | $3.30   | $0.00       | ~$3.30   |

**Fixed Costs:**
- CloudWatch Logs: $0.50/GB stored (minimal for 7-day retention)
- API Gateway: No fixed cost (pay per request)
- Lambda: No fixed cost (pay per invocation)
- ECS Cluster: No cost (Fargate is serverless)

### Cost Optimization Tips

1. **Use FARGATE_SPOT (default)** - 70% savings over Fargate on-demand
2. **Right-size tasks** - 0.25 vCPU adequate for most commands
3. **Short log retention** - 7 days default (adjust based on needs)
4. **Shallow clones** - `--depth 1` minimizes data transfer
5. **ARM64 architecture** - 20% cheaper than x86_64 (Lambda only, not ECS images)
6. **Public subnets** - No NAT Gateway costs ($0.045/hour saved)

### Cost Comparison

| Service         | mycli  | GitHub Actions | AWS CodeBuild |
|----------------|--------|----------------|---------------|
| Per execution  | $0.0003| Free*          | $0.025        |
| 1000/month     | $0.33  | Free*          | $25           |
| Infrastructure | $0     | $0             | $0            |
| Control        | Full   | Limited        | AWS-managed   |

*GitHub Actions: 2000 minutes/month free for private repos

---

## Deployment & Testing

### Prerequisites

- AWS account with admin access
- AWS CLI configured (`aws configure`)
- Go 1.21+ installed
- Git installed

### Initial Deployment

**1. Build CLI:**
```bash
cd /path/to/mycli
go build -o mycli
```

**2. Deploy infrastructure:**
```bash
./mycli init --region us-east-1
```

This will:
- Generate API key
- Prompt for Git credentials (optional)
- Build Lambda function
- Create CloudFormation stack (~5 min)
- Create Lambda function
- Configure API Gateway
- Save config to ~/.mycli/config.yaml

**3. Verify deployment:**
```bash
# Check config
cat ~/.mycli/config.yaml

# Check CloudFormation stack
aws cloudformation describe-stacks --stack-name mycli --region us-east-1

# Check Lambda function
aws lambda get-function --function-name mycli-orchestrator --region us-east-1
```

### Testing

**Test 1: Public repository**
```bash
mycli exec \
  --repo=https://github.com/hashicorp/terraform-guides \
  --image=hashicorp/terraform:1.6 \
  "ls -la"
```

**Test 2: Command execution**
```bash
mycli exec \
  --skip-git \
  --image=alpine:latest \
  "echo 'Hello from mycli!'"
```

**Test 3: With .mycli.yaml**
```bash
# Create test config
cat > .mycli.yaml << EOF
repo: https://github.com/hashicorp/terraform-guides
image: hashicorp/terraform:1.6
EOF

# Execute
mycli exec "terraform version"
```

**Test 4: Status and logs**
```bash
# Start execution
mycli exec --skip-git --image=alpine:latest "sleep 30 && echo done"

# Get task ARN from output, then:
mycli status <task-arn>
mycli logs <execution-id>
mycli logs -f <execution-id>  # Follow
```

**Test 5: Private repository (if Git credentials configured)**
```bash
mycli exec \
  --repo=https://github.com/your-company/private-repo \
  "ls -la"
```

### Integration Testing

**Error scenarios to test:**
- Invalid repository URL → Check error message
- Non-existent branch → Check git clone failure
- Command failure (exit code != 0) → Verify exit code propagated
- Missing Git credentials for private repo → Check auth failure
- Invalid API key → Verify 401 response
- Invalid Docker image → Check ECS task failure

### Cleanup

```bash
./mycli destroy
rm ~/.mycli/config.yaml  # Optional
```

---

## Limitations & Trade-offs

### What Works

- ✅ Public Git repositories (GitHub, GitLab, Bitbucket)
- ✅ Private repos with token/SSH authentication
- ✅ Any Docker image from Docker Hub, ECR, or private registries
- ✅ Custom image per execution via `--image` flag or `.mycli.yaml`
- ✅ Commands up to ~30 minutes (configurable)
- ✅ Environment variable passing
- ✅ Exit code propagation
- ✅ Multi-line commands and scripts
- ✅ Concurrent executions (limited by AWS quotas)

### Current Limitations

| Limitation | Workaround |
|-----------|------------|
| No artifact storage | Use S3 from your commands: `aws s3 cp output.txt s3://bucket/` |
| No multi-step workflows | Create script in repo: `./run.sh` |
| No real-time log streaming | Use `mycli logs -f` (polls every 2s) |
| No scheduled executions | Use EventBridge to invoke API |
| No Git submodules | Clone with `--recurse-submodules` in script |
| Working directory is repo root | `cd subdirectory && command` in script |
| Logs require task ARN (not execution ID) | Task ARN is provided in exec output, copy/paste it |

### Design Trade-offs

| Decision | Pro | Con |
|----------|-----|-----|
| No custom Docker image | Simple, flexible, no maintenance | May need to install git at runtime |
| Bash script construction | Universal, easy to debug | Less robust than compiled code |
| Public subnets | Cheaper (no NAT Gateway) | Tasks have public IPs |
| No S3 for code | Simpler, cheaper | Can't store artifacts |
| Shallow git clone | Faster, less data transfer | No git history |
| No DynamoDB | Simpler, cheaper | No queryable execution history |
| Logs use task ARN directly | Simple, no tag lookup needed | Longer ARN to copy vs short execution ID |

### Not Supported (Yet)

- ❌ Repositories > 1 GB (shallow clone helps but large repos still slow)
- ❌ Execution history queries (no DynamoDB table)
- ❌ Scheduled/cron executions
- ❌ Multi-step workflows with dependencies
- ❌ Artifact storage and retrieval
- ❌ Custom task role per execution
- ❌ VPC endpoint for private ECR (uses public ECR/Docker Hub)

---

## Future Enhancements

### Near-term (1-2 months)

**Execution History & Metadata**
- Add DynamoDB table for execution records
- Store: execution ID, repo, command, status, duration, cost, tags
- Benefits: Queryable history, audit trail, cost analysis
- `mycli list` - Show recent executions
- `mycli list --repo=...` - Filter by repository

**API Keys Management**
- Multiple API keys per deployment
- Key rotation without redeployment
- Scoped permissions (read-only vs execute)
- Per-key rate limiting

**Enhanced Error Handling**
- Detailed error messages from Lambda
- Retry logic for transient failures
- Better git clone error diagnostics

**Custom Task Roles**
- `--task-role-arn` flag for per-execution IAM role
- Least-privilege execution

### Medium-term (3-6 months)

**~~Dynamic Task Definition Registration~~ ✓ IMPLEMENTED**
- ✓ Register task definition per image on-the-fly
- ✓ True custom image support per execution
- ✓ Cache task definitions to avoid duplicates (checks if family exists before registering)
- ✓ Human-readable task definition names based on image name
- ✓ Collision detection for edge cases (same sanitized name, different image)

**S3 Artifact Storage**
- Optional S3 bucket for command outputs
- `mycli artifacts <execution-id>` - Download outputs
- Auto-upload files from /workspace/artifacts

**Web Dashboard**
- CloudFront + S3 static site
- View execution history
- Real-time log streaming (WebSocket)
- Cost tracking graphs

**Scheduled Executions**
- EventBridge integration
- Cron syntax: `mycli schedule "0 0 * * *" "terraform plan"`
- Manage scheduled tasks

### Long-term (6-12 months)

**Multi-step Workflows**
- YAML-based workflow definitions
- Dependencies between steps
- Conditional execution
- Parallel steps

**Team Management**
- User accounts and authentication
- Team workspaces
- Shared execution history
- Access control

**SaaS Offering**
- Hosted infrastructure (we manage AWS)
- User signup and billing
- Usage metering
- Multi-tenancy

**Multi-cloud Support**
- Google Cloud Run backend
- Azure Container Instances
- Unified CLI for all clouds

### Shell Command Improvements

**Current: Bash (adequate for MVP)**

**Potential upgrades:**
1. **Go binary approach** - Compile orchestrator.go, inject into container
   - Pros: Better error handling, type safety, no git install needed
   - Cons: Multi-arch compilation, larger payload, more complex

2. **Python script approach** - Self-contained Python orchestrator
   - Pros: Better than bash, most images have Python
   - Cons: Not as universal

3. **Hybrid approach** - Bash for simple, Go for complex

**Recommendation:** Revisit if we encounter serious bash limitations.

---

## Troubleshooting

### Common Issues

**Issue: "Failed to clone repository"**

Symptoms:
```
ERROR: Failed to clone repository
Please verify:
  - Repository URL is correct
  - Branch exists
  - Git credentials are configured (for private repos)
```

Solutions:
1. Verify repository URL: `git ls-remote <repo-url>`
2. Check branch exists: `git ls-remote --heads <repo-url> <branch>`
3. For private repos, ensure Git credentials configured during `mycli init`
4. Test credentials locally: `git clone <repo-url>`
5. Check Lambda environment variables have GITHUB_TOKEN or SSH_PRIVATE_KEY

**Issue: "Command not found"**

Symptoms:
```
/bin/sh: terraform: not found
```

Solutions:
1. Use image with tool pre-installed: `--image=hashicorp/terraform:1.6`
2. Or install in command: `mycli exec "apk add terraform && terraform plan"`
3. Check image documentation for available tools

**Issue: Task takes too long to start**

Symptoms:
- Task shows PROVISIONING for 2-3 minutes
- Eventually times out

Solutions:
1. Large Docker images take time to pull (~1 min per GB)
2. Use smaller base images: `alpine` instead of `ubuntu`
3. Use image tags to get cached images: `python:3.11` not `python:latest`
4. First execution always slower (image not cached)

**Issue: "No logs available"**

Symptoms:
```
mycli logs <execution-id>
No logs available yet. The task may still be starting.
```

Solutions:
1. Logs take 5-10 seconds to appear in CloudWatch
2. Wait and retry: `mycli logs <execution-id>`
3. Or use follow mode: `mycli logs -f <execution-id>`
4. Check task started: `mycli status <task-arn>`
5. If task STOPPED immediately, check task definition for errors

**Issue: "API key authentication fails"**

Symptoms:
```
Error: unauthorized
```

Solutions:
1. Check ~/.mycli/config.yaml has correct API key
2. Verify Lambda environment variable API_KEY_HASH is set:
   ```bash
   aws lambda get-function-configuration \
     --function-name mycli-orchestrator \
     --query 'Environment.Variables.API_KEY_HASH'
   ```
3. Re-run `mycli init` to regenerate key and hash
4. Ensure API key in header: `X-API-Key: sk_live_...`

**Issue: "User is not authorized to perform: ecs:TagResource"**

Symptoms:
```
AccessDeniedException: User: arn:aws:sts::...:assumed-role/mycli-lambda-role/mycli-orchestrator
is not authorized to perform: ecs:TagResource
```

Solutions:
1. **FIXED** - Update CloudFormation template to add `ecs:TagResource` permission (deploy/cloudformation.yaml:248)
2. Update stack:
   ```bash
   aws cloudformation update-stack \
     --stack-name mycli \
     --template-body file://deploy/cloudformation.yaml \
     --capabilities CAPABILITY_NAMED_IAM \
     --parameters \
       ParameterKey=APIKeyHash,UsePreviousValue=true \
       ParameterKey=GitHubToken,UsePreviousValue=true \
       ParameterKey=GitLabToken,UsePreviousValue=true \
       ParameterKey=SSHPrivateKey,UsePreviousValue=true
   ```

**Issue: Stack creation fails**

Symptoms:
```
CloudFormation stack creation failed
```

Solutions:
1. Check CloudFormation events:
   ```bash
   aws cloudformation describe-stack-events --stack-name mycli
   ```
2. Common causes:
   - IAM permission issues (need admin access)
   - Resource limits (VPC limit, EIP limit)
   - Region not supported
3. Delete failed stack and retry:
   ```bash
   aws cloudformation delete-stack --stack-name mycli
   # Wait for deletion, then:
   mycli init
   ```

### Debugging Tips

**View Lambda logs:**
```bash
aws logs tail /aws/lambda/mycli-orchestrator --follow
```

**Check ECS task details:**
```bash
aws ecs describe-tasks \
  --cluster mycli-cluster \
  --tasks <task-id>
```

**View CloudWatch logs directly:**
```bash
aws logs filter-log-events \
  --log-group-name /aws/mycli/mycli \
  --log-stream-name-prefix task/
```

**Test API Gateway directly:**
```bash
curl -X POST \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"action":"exec","repo":"https://github.com/user/repo","command":"echo test","skip_git":true}' \
  $API_ENDPOINT
```

---

## Appendix: Project Structure

```
mycli/
├── cmd/
│   ├── root.go                # Cobra root command
│   ├── init.go                # Infrastructure deployment (cmd/init.go:61)
│   ├── configure.go           # Manual configuration
│   ├── exec.go                # Execute commands (cmd/exec.go:65)
│   ├── status.go              # Check execution status
│   ├── logs.go                # View execution logs
│   └── destroy.go             # Cleanup infrastructure
├── deploy/
│   └── cloudformation.yaml    # Infrastructure template
├── internal/
│   ├── config/                # Global config management
│   │   └── config.go          # ~/.mycli/config.yaml
│   ├── project/               # Project config management
│   │   └── config.go          # .mycli.yaml parser
│   ├── git/                   # Git utilities
│   │   └── detector.go        # Remote URL auto-detection
│   └── api/                   # API client
│       └── client.go          # HTTP client for API Gateway
├── lambda/
│   └── orchestrator/
│       ├── main.go            # Lambda handler entry
│       ├── config.go          # AWS clients and env vars
│       ├── auth.go            # API key auth
│       ├── handlers.go        # exec/status/logs handlers
│       ├── shell.go           # shell command builders
│       ├── response.go        # response helpers
│       ├── util.go            # misc utilities
│       └── types.go           # shared types aliases
├── pkg/
│   └── api/
│       └── types.go           # Shared API types (Request/Response)
├── scripts/
│   ├── README.md              # Development scripts documentation
│   └── update-lambda.sh       # Lambda update helper
├── main.go                    # CLI entry point
├── go.mod                     # Go dependencies
├── go.sum                     # Dependency checksums
├── README.md                  # User documentation
└── ARCHITECTURE.md            # This file (technical documentation)
```

---

## Appendix: AWS Resource Summary

| Resource Type | Name/ID | Purpose |
|--------------|---------|---------|
| VPC | mycli-vpc | Network isolation |
| Subnets | mycli-public-1, mycli-public-2 | Multi-AZ public subnets |
| Internet Gateway | mycli-igw | Internet access |
| Security Group | mycli-fargate-sg | Egress-only for tasks |
| ECS Cluster | mycli-cluster | Fargate task orchestration |
| Task Definition | mycli-task | Task template |
| Log Group | /aws/mycli/mycli | Execution logs |
| IAM Roles | mycli-lambda-role, mycli-task-execution-role, mycli-task-role | Permissions |
| Lambda Function | mycli-orchestrator | API request handler |
| API Gateway REST API | mycli-api | REST API endpoint |
| API Gateway Method | POST /execute | API method configuration |
| Lambda Permission | AllowAPIGatewayInvoke | API Gateway invoke permission |
| API Gateway Deployment | prod | API deployment to prod stage |

**Total Resources:** ~18 (all managed by CloudFormation)

---

**Last Updated:** 2025-01-26
**Version:** 1.0 (Working MVP)
