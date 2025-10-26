# mycli - Remote Command Execution Platform

## Project Vision

A CLI tool that provides isolated, repeatable execution environments for commands. Solves the problem of running infrastructure-as-code tools (Terraform, Ansible, etc.) and other CLI applications without local execution issues like race conditions, credential sharing, or dependency conflicts.

**Key principle:** General purpose remote execution, not tool-specific. Users can run any command in a containerized environment.

## Target Use Cases

- Infrastructure as Code execution (Terraform, Ansible, Pulumi)
- CI/CD workflows
- Scripts requiring AWS credentials
- Any command needing isolation and audit trails

## Deployment Models

1. **Self-hosted (MVP focus):** Users deploy infrastructure to their own AWS account
2. **SaaS (future):** We host the infrastructure, users just use the CLI

## Architecture (Ultra-MVP)

### AWS Services Used

**Compute & Orchestration:**
- **API Gateway:** REST API endpoint for CLI to trigger executions
- **Lambda:** Orchestrator that receives requests and starts Fargate tasks
- **ECS Fargate:** Serverless containers that execute the actual commands

**Storage:**
- **CloudWatch Logs:** Execution output (stdout/stderr)
- ~~S3: Dropped for MVP - Git is source of truth~~

**Auth:**
- **Lambda Environment Variable:** Stores bcrypt-hashed API key
- **Lambda Environment Variable:** Stores Git credentials (GitHub token, SSH key, etc.)
- No DynamoDB (removed for simplicity)

**Security:**
- API key authentication (bcrypt hashed, stored in Lambda env var)
- Git credentials (stored in Lambda env var or AWS Secrets Manager)
- IAM roles for proper AWS service permissions

### Execution Flow

```
1. User: mycli exec "terraform apply" (in project directory with .mycli.yaml)
2. CLI reads config from .mycli.yaml or --repo flag
3. CLI calls API Gateway: POST /executions
   {
     "repo": "https://github.com/user/infra",
     "branch": "main",
     "command": "terraform apply",
     "image": "hashicorp/terraform:1.6",
     "env": {...}
   }
4. API Gateway invokes Lambda (validates API key)
5. Lambda starts ECS Fargate task with:
   - Docker image (default or user-specified)
   - Git repo URL and branch
   - Command to execute
   - Environment variables
   - Git credentials (from Lambda env var)
6. Fargate task:
   - Clones repo: git clone --depth 1 <repo>
   - Checks out branch
   - Runs the command in repo directory
   - Streams output to CloudWatch Logs
   - Exits
7. CLI polls task status via ECS API
8. CLI fetches logs from CloudWatch
```

### Data Flow

**No persistent storage for code:**
- Git is the source of truth
- Container clones repo at runtime
- No S3 upload/download needed
- Shallow clone (--depth 1) for speed

**No execution metadata storage in MVP:**
- CLI talks directly to ECS API for status
- CLI talks directly to CloudWatch Logs API for logs
- Stateless design - simpler, fewer components

## CLI Commands

### `mycli init`
**Purpose:** One-command infrastructure setup

**What it does:**
1. Deploys CloudFormation stack to user's AWS account
2. Generates random API key (format: `sk_live_{64_hex_chars}`)
3. Hashes key with bcrypt
4. Updates Lambda environment variable with hash
5. Prompts for Git credentials (GitHub token, SSH key) - optional
6. Stores Git credentials in Lambda environment variable
7. Saves config to `~/.mycli/config.yaml`
8. Shows API key to user (once)

**Flags:**
- `--stack-name` (default: "mycli")
- `--region` (default: from AWS config)

**Git Credential Setup (optional):**
```
→ Optional: Configure Git access
  Do you want to configure GitHub access? [y/N]: y
  
  Choose authentication method:
  1) GitHub Personal Access Token (recommended)
  2) SSH Private Key
  3) Skip (public repos only)
  
  Selection: 1
  Enter GitHub token: ghp_xxxxx
  ✓ Token stored securely in Lambda environment
```

**Output:**
```
🚀 Initializing mycli infrastructure...
→ Creating CloudFormation stack...
→ Generating API key...
→ Storing API key...
→ Configuring Git access...
→ Saving configuration...
✅ Setup complete!

Configuration saved to ~/.mycli/config.yaml
  API Endpoint: https://abc123.execute-api.us-east-1.amazonaws.com/prod
  Region:       us-east-1

🔑 Your API key: sk_live_a1b2c3d4...
   (Also saved to config file)

Test it with: mycli exec "echo hello"
```

### `mycli configure`
**Purpose:** Manual configuration or reconfiguration

**What it does:**
1. Discovers existing CloudFormation stack
2. Extracts outputs (API endpoint, code bucket)
3. Prompts for API key (not in stack outputs)
4. Saves to config file

**Use cases:**
- Lost config file
- Multiple environments
- Deployed infrastructure manually

### `mycli exec "command"`
**Purpose:** Execute command remotely

**Flags:**
- `--repo`: Git repository URL (overrides .mycli.yaml and git remote)
- `--branch`: Git branch to checkout (default: main, overrides .mycli.yaml)
- `--image`: Docker image to use (default: mycli default image, overrides .mycli.yaml)
- `--env`: Environment variables (repeatable: `--env KEY=VALUE`, merges with .mycli.yaml)
- `--timeout`: Max execution time in seconds (default: 1800, overrides .mycli.yaml)

**Configuration Priority (highest to lowest):**
1. Command-line flags
2. `.mycli.yaml` in current directory
3. Git remote URL from current directory (auto-detected)
4. Error if no repo specified

**What it does:**
1. Loads config from `~/.mycli/config.yaml`
2. Reads project config from `.mycli.yaml` (if exists)
3. Auto-detects git remote if no config (optional fallback)
4. Generates execution ID: `exec_{timestamp}_{random}`
5. Calls API: `POST /executions` with repo, branch, command, etc.
6. Returns execution ID and task ARN
7. Shows how to check status/logs

**Examples:**

**With .mycli.yaml (simplest):**
```bash
cd my-terraform-project  # has .mycli.yaml
mycli exec "terraform plan"
# Uses repo, branch, image from .mycli.yaml
```

**Override specific settings:**
```bash
mycli exec --branch=dev "terraform plan"
mycli exec --env TF_VAR_region=us-west-2 "terraform apply"
```

**Without .mycli.yaml (explicit):**
```bash
mycli exec --repo=https://github.com/user/infra "terraform apply"
mycli exec --repo=https://github.com/user/infra --branch=dev "terraform plan"
```

**Auto-detect from git remote:**
```bash
cd my-git-repo  # no .mycli.yaml, but has git remote
mycli exec "make deploy"
# Automatically uses: git remote get-url origin
```

**Output:**
```bash
$ mycli exec "terraform apply"
→ Repository: https://github.com/user/infra (from .mycli.yaml)
→ Branch: main
→ Starting execution...
✓ Execution started: exec_abc123

Run 'mycli status exec_abc123' to check status
Run 'mycli logs exec_abc123' to view logs
```

### `mycli status <execution-id>`
**Purpose:** Check execution status

**What it does:**
1. Queries ECS API for task status using task ARN
2. Shows: status, command, start time, duration

**Output:**
```
Execution ID: exec_abc123
Status: running
Command: terraform apply -auto-approve
Started: 2025-10-25 14:32:10
Duration: 45s
```

### `mycli logs <execution-id>`
**Purpose:** View execution logs

**Flags:**
- `-f, --follow`: Stream logs in real-time

**What it does:**
1. Determines CloudWatch log stream from task ARN
2. Fetches logs from CloudWatch Logs API
3. Displays to stdout
4. If `--follow`, polls for new log lines

### `mycli init-project`
**Purpose:** Initialize project-level configuration (optional convenience command)

**What it does:**
1. Detects git remote URL (if in git repo)
2. Prompts for configuration
3. Creates `.mycli.yaml` in current directory

**Example:**
```bash
$ cd my-terraform-project
$ mycli init-project

? Repository URL: (https://github.com/user/infra) 
? Branch: (main) 
? Docker image: (hashicorp/terraform:1.6) 
? Timeout (seconds): (1800) 
? Add environment variables? [y/N]: y
  KEY: TF_VAR_region
  VALUE: us-east-1
  Add another? [y/N]: n

✓ Created .mycli.yaml

You can now run: mycli exec "terraform plan"
```

**Note:** This is optional - users can create `.mycli.yaml` manually.
**Purpose:** Clean up all infrastructure

**Flags:**
- `--stack-name` (default: "mycli")
- `--region` (default: from AWS config or saved config)
- `--force`: Skip confirmation prompt

**What it does:**
1. Confirms with user (unless `--force`)
2. Empties S3 bucket (required before deletion)
3. Deletes CloudFormation stack
4. Waits for deletion to complete
5. Optionally removes local config file

**Example:**
```bash
$ mycli destroy
⚠️  This will delete all mycli infrastructure in your AWS account.
   Stack: mycli
   Region: us-east-1
   
Continue? [y/N]: y

→ Emptying S3 bucket...
→ Deleting CloudFormation stack...
  Waiting for stack deletion...
✓ Infrastructure destroyed

Local config (~/.mycli/config.yaml) preserved.
Run 'rm ~/.mycli/config.yaml' to remove it.
```

## Project Configuration File

### `.mycli.yaml`

**Location:** Project root directory (same directory as your Terraform/Ansible files)

**Purpose:** Define project-specific execution configuration

**Should be committed:** Yes - this is project config, not secrets

**Format:**
```yaml
# Repository to clone (required if not using --repo flag)
repo: https://github.com/mycompany/infrastructure

# Branch to checkout (optional, default: main)
branch: main

# Docker image to use (optional, default: mycli default image)
image: hashicorp/terraform:1.6

# Environment variables (optional)
env:
  TF_VAR_environment: production
  TF_VAR_region: us-east-1
  AWS_REGION: us-east-1

# Timeout in seconds (optional, default: 1800)
timeout: 3600
```

**Example for Terraform project:**
```yaml
# .mycli.yaml
repo: https://github.com/mycompany/aws-infrastructure
branch: main
image: hashicorp/terraform:1.6
env:
  TF_VAR_region: us-east-1
  TF_VAR_environment: production
timeout: 3600
```

**Example for Ansible project:**
```yaml
# .mycli.yaml
repo: https://github.com/mycompany/ansible-playbooks
image: ansible/ansible:latest
env:
  ANSIBLE_HOST_KEY_CHECKING: "False"
```

**Minimal config (rely on defaults):**
```yaml
# .mycli.yaml
repo: https://github.com/mycompany/scripts
```

**Multi-environment pattern:**
```yaml
# .mycli.yaml - base config
repo: https://github.com/mycompany/infra
# Use --branch flag for different environments
```

```bash
mycli exec --branch=dev "terraform apply"   # dev environment
mycli exec --branch=prod "terraform apply"  # prod environment
```

### Configuration Priority

When executing, config is resolved in this order (highest priority first):

1. **Command-line flags** (e.g., `--repo`, `--branch`, `--env`)
2. **`.mycli.yaml`** in current directory
3. **Git remote URL** (auto-detected via `git remote get-url origin`)
4. **Error** if no repo specified

**Example:**
```yaml
# .mycli.yaml
repo: https://github.com/user/infra
branch: main
```

```bash
# Uses branch from .mycli.yaml
$ mycli exec "terraform plan"
→ Branch: main

# Overrides branch with flag
$ mycli exec --branch=dev "terraform plan"
→ Branch: dev
```

### Auto-detection

If no `.mycli.yaml` exists and no `--repo` flag provided:

```bash
$ cd my-git-repo
$ mycli exec "make deploy"
→ Repository: https://github.com/user/repo (auto-detected from git remote)
→ Branch: main
→ Starting execution...
```

## Global Configuration File

**Location:** `~/.mycli/config.yaml`

**Purpose:** User-level mycli configuration

**Contents:**
```yaml
api_endpoint: https://abc123.execute-api.us-east-1.amazonaws.com/prod
api_key: sk_live_a1b2c3d4e5f6...
region: us-east-1
```

**Permissions:** 0600 (read/write for owner only)

**Created by:** `mycli init` command

**Note:** This is user config, NOT project config. Project config lives in `.mycli.yaml`.

## CloudFormation Resources (To Be Created)

### ~~S3 Bucket~~ (Removed - Git is source of truth)

### API Gateway
- Type: REST API
- Authorization: AWS_IAM (Signature V4)
- Endpoints:
  - `POST /executions` - Create execution
  - `GET /executions/{id}` - Get execution status (future)

### Lambda Function

- Runtime: Go (provided.al2023 runtime, ARM64)
- Purpose: Orchestrator
- Environment variables:
  - `API_KEY_HASH`: bcrypt hash of API key
  - `GITHUB_TOKEN`: GitHub Personal Access Token (optional)
  - `GITLAB_TOKEN`: GitLab token (optional)
  - `SSH_PRIVATE_KEY`: Base64-encoded SSH key (optional)
  - `ECS_CLUSTER`: ECS cluster name
  - `ECS_TASK_DEFINITION`: Task definition ARN
- Permissions:
  - Start ECS tasks
  - Read environment variables
  - Validate incoming requests

### ECS Cluster
- Type: Fargate
- Name: `mycli-cluster`

### ECS Task Definition
- Launch type: Fargate
- CPU: 0.25 vCPU (configurable via parameter)
- Memory: 0.5 GB (configurable via parameter)
- Container:
  - Image: Public ECR image with common tools (Terraform, Git, AWS CLI, etc.)
  - Entry point: Custom orchestrator script
  - Logs: CloudWatch Logs
  - Environment variables passed from Lambda (repo, branch, command, Git credentials)
- Task role: Permissions for actual AWS operations (user-configurable)

### CloudWatch Log Group
- Name: `/mycli/executions`
- Retention: 7 days (configurable)
- Log streams: One per task (automatic via ECS)

### IAM Roles

**Lambda Execution Role:**
- Permissions:
  - Write CloudWatch Logs
  - Start ECS tasks
  - Read environment variables
  - Update own function configuration (for `mycli init`)

**ECS Task Execution Role:**
- Permissions:
  - Pull Docker images from ECR
  - Write CloudWatch Logs

**ECS Task Role:**
- Permissions: User-configurable (parameter)
- Default: AdministratorAccess (for MVP)
- Production: User provides policy ARN

## Authentication Flow

### API Key Generation (in `mycli init`)
```go
// 1. Generate random key
randomBytes := make([]byte, 32)
rand.Read(randomBytes)
apiKey := fmt.Sprintf("sk_live_%s", hex.EncodeToString(randomBytes))

// 2. Hash with bcrypt
hash, _ := bcrypt.GenerateFromPassword([]byte(apiKey), bcrypt.DefaultCost)

// 3. Store hash in Lambda env var
lambdaClient.UpdateFunctionConfiguration(ctx, &lambda.UpdateFunctionConfigurationInput{
    FunctionName: "mycli-orchestrator",
    Environment: &types.Environment{
        Variables: map[string]string{
            "API_KEY_HASH": string(hash),
        },
    },
})

// 4. Save plaintext key to config
config.Save(&Config{APIKey: apiKey, ...})
```

### Request Validation (in Lambda)
```python
import bcrypt
import os

def validate_api_key(incoming_key):
    stored_hash = os.environ['API_KEY_HASH']
    return bcrypt.checkpw(
        incoming_key.encode('utf-8'),
        stored_hash.encode('utf-8')
    )

def handler(event, context):
    api_key = event['headers'].get('X-API-Key', '')
    
    if not validate_api_key(api_key):
        return {
            'statusCode': 401,
            'body': json.dumps({'error': 'Invalid API key'})
        }
    
    # Continue with execution logic...
```

### CLI Request (using API key)
```go
req, _ := http.NewRequest("POST", apiEndpoint, body)
req.Header.Set("X-API-Key", cfg.APIKey)
req.Header.Set("Content-Type", "application/json")
resp, _ := http.DefaultClient.Do(req)
```

## Docker Image (Default Container)

**Base:** Ubuntu or Alpine
**Pre-installed tools:**
- Git (for cloning repos)
- Terraform (latest stable)
- AWS CLI
- Python 3
- curl, jq, other common utilities

**Entry point:** Custom orchestrator script that:
1. Reads environment variables (REPO_URL, BRANCH, COMMAND, GIT_TOKEN)
2. Clones repo: `git clone --depth 1 --branch $BRANCH $REPO_URL /workspace`
3. Changes to workspace: `cd /workspace`
4. Executes user's command
5. Streams output to stdout (→ CloudWatch)
6. Exits with command's exit code

**Git Authentication:**
- If `GITHUB_TOKEN` env var exists: `git clone https://$GITHUB_TOKEN@github.com/user/repo`
- If `SSH_PRIVATE_KEY` env var exists: Setup SSH key, use SSH URL
- If neither: Public repos only

**Future:** Allow users to specify custom images via `--image` flag

## API Contract

### POST /executions

**Request:**
```json
{
  "execution_id": "exec_abc123",
  "repo": "https://github.com/user/infrastructure",
  "branch": "main",
  "command": "terraform apply -auto-approve",
  "image": "hashicorp/terraform:1.6",
  "env": {
    "TF_VAR_region": "us-east-1",
    "AWS_REGION": "us-east-1"
  },
  "timeout_seconds": 1800
}
```

**Response:**
```json
{
  "execution_id": "exec_abc123",
  "task_arn": "arn:aws:ecs:us-east-1:123456789:task/mycli-cluster/abc123",
  "status": "starting",
  "log_stream": "exec/exec_abc123/abc123",
  "created_at": "2025-10-25T14:32:10Z"
}
```

## Technology Stack

**CLI:**
- Language: Go
- Framework: Cobra (commands)
- AWS SDK: aws-sdk-go-v2
- Config: YAML (gopkg.in/yaml.v3)
- Crypto: bcrypt (golang.org/x/crypto/bcrypt)
- Git: Auto-detect remotes, parse .mycli.yaml

**Backend:**
- Lambda: Python 3.11 or Go
- Container: Docker (Ubuntu/Alpine base with Git)
- IaC: CloudFormation

**AWS Services:**
- API Gateway, Lambda, ECS Fargate, CloudWatch Logs

## Future Enhancements (Post-MVP)

### Execution History Table (DynamoDB)
Currently: CLI queries ECS/CloudWatch directly (stateless)
Future: Store execution metadata for:
- Queryable history
- Tags/labels
- Cost tracking
- Usage analytics

### Multi-tenancy
Currently: Single API key per deployment
Future:
- Multiple API keys
- User/team isolation
- Per-key rate limiting
- Key rotation

### Enhanced Features
- Log archival to S3 (long-term retention)
- Execution templates (saved configs)
- Scheduled executions
- Webhooks on completion
- Cost estimation
- Multi-cloud support (GCP, Azure)

### SaaS Mode
- Hosted API endpoint
- User signup/billing
- Web dashboard
- Team collaboration
- Usage metering

## Open Questions / Decisions Needed

1. **Lambda runtime:** Python 3.11 or Go? (Python simpler for inline, Go for performance)
2. **Container orchestrator:** Inline Python script or separate Go binary?
3. **Default task permissions:** Full admin or require user to specify?
4. **Error handling:** How detailed should error messages be?
5. **Timeouts:** Default 30min reasonable? Max timeout?

## Development Workflow

### Local Development
```bash
# Setup
./setup-mycli.sh
cd mycli
go mod tidy

# Build
go build -o mycli

# Test
./mycli --help
```

### Deployment Testing
```bash
# Deploy to AWS
./mycli init --region us-east-1

# Test execution
./mycli exec "echo 'Hello, world!'"
./mycli status exec_abc123
./mycli logs exec_abc123

# Cleanup
./mycli destroy
```

## Project Structure
```
mycli/
├── cmd/
│   ├── root.go          # Cobra root command
│   ├── init.go          # Infrastructure deployment + Git credential setup
│   ├── init_project.go  # Create .mycli.yaml (optional helper)
│   ├── configure.go     # Manual configuration
│   ├── exec.go          # Execute commands (reads .mycli.yaml)
│   ├── status.go        # Check execution status
│   ├── logs.go          # View execution logs
│   └── destroy.go       # Cleanup infrastructure
├── internal/
│   ├── config/          # Config file management (~/.mycli/config.yaml)
│   ├── project/         # Project config (.mycli.yaml parser)
│   ├── api/             # API client (HTTP requests)
│   └── git/             # Git remote detection
├── deploy/
│   └── cloudformation.yaml  # Infrastructure template (no S3 bucket)
├── .mycli.yaml          # Example project config (in docs)
├── main.go              # Entry point
├── go.mod               # Go dependencies
├── README.md            # User documentation
└── CONTEXT.md           # Architecture & design (this file)
```

## Next Steps

1. ✅ CLI skeleton created
2. ✅ `init` command designed
3. ✅ Config management implemented
4. ✅ Git-first architecture decided
5. ✅ Project config (.mycli.yaml) designed
6. ⏭️ **CloudFormation template** (current focus - no S3 bucket)
7. ⏭️ Lambda orchestrator implementation
8. ⏭️ Container orchestrator script (Git clone logic)
9. ⏭️ Project config parser (.mycli.yaml)
10. ⏭️ Git remote auto-detection
11. ⏭️ API client implementation
12. ⏭️ Integration testing
13. ⏭️ Documentation & examples