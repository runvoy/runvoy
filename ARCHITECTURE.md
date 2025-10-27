# runvoy Architecture

## Overview

runvoy is a centralized execution platform that allows teams to run infrastructure commands without sharing AWS credentials. An AWS admin deploys runvoy once to the company's AWS account, then issues API keys to team members who can execute commands safely with full audit trails.

## Design Principles

1. **Centralized Execution, Distributed Access**: One deployment per company, multiple users with API keys
2. **No Credential Sharing**: Team members never see AWS credentials
3. **Complete Audit Trail**: Every execution logged with user identification
4. **Safe Stateful Operations**: Automatic locking prevents concurrent operations on shared resources
5. **Self-Service**: Team members don't wait for admins to run commands
6. **Extensible Authorization**: Architecture supports fine-grained permissions (to be added later)

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         AWS Account                              │
│                                                                  │
│  ┌──────────────┐                                               │
│  │ Lambda       │◄─────── HTTPS Function URL with X-API-Key    │
│  │ Function URL │     header                                  │
│  └──────┬───────┘                                               │
│         │                                                        │
│  ┌──────▼───────────┐                                          │
│  │ Lambda           │                                           │
│  │ (Orchestrator)   │                                           │
│  │                  │                                           │
│  │ - Validate API   │         ┌──────────────┐                │
│  │   key            │────────►│ DynamoDB     │                │
│  │ - Check lock     │         │ - API Keys   │                │
│  │ - Start ECS task │         │ - Locks      │                │
│  │ - Record exec    │         │ - Executions │                │
│  └──────┬───────────┘         └──────────────┘                │
│         │                                                        │
│  ┌──────▼───────────┐                                          │
│  │ ECS Fargate      │                                           │
│  │                  │                                           │
│  │ Container:       │         ┌──────────────┐                │
│  │ - Clone git repo │────────►│ S3 Bucket    │                │
│  │   (optional)     │         │ - Code       │                │
│  │ - Run command    │         │   uploads    │                │
│  │ - Stream logs    │         └──────────────┘                │
│  │                  │                                           │
│  │ Task Role:       │                                           │
│  │ - AWS perms for  │         ┌──────────────┐                │
│  │   actual work    │────────►│ CloudWatch   │                │
│  └──────────────────┘         │ Logs         │                │
│                                └──────────────┘                │
│                                                                  │
│  ┌──────────────────────────────────────────┐                 │
│  │ Web UI (S3 + CloudFront)                 │                 │
│  │ - Static site for viewing logs           │                 │
│  │ - Token-based access (no login)          │                 │
│  │ - Real-time log streaming                │                 │
│  └──────────────────────────────────────────┘                 │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────┐
│ Users           │
│                 │
│ - CLI with API  │
│   key (no AWS   │
│   credentials)  │
│                 │
│ - Web browser   │
│   for viewing   │
│   logs          │
└─────────────────┘
```

## Components

### 1. CLI (Go)

**Purpose**: User-facing interface for executing commands and managing the platform

**Architecture**: The CLI is built with a modular provider architecture to support multiple cloud providers. The `init` command uses a provider abstraction layer that allows for easy extension to additional cloud platforms (AWS, GCP, Azure, etc.).

**Provider Architecture**:
- `internal/provider/` - Provider abstraction layer
  - `interface.go` - Defines the `Provider` interface with methods for infrastructure lifecycle management
  - `types.go` - Shared types (`InfrastructureOutput`, `Config`, `ValidationError`)
  - `aws.go` - AWS-specific implementation (CloudFormation, DynamoDB, S3, Lambda)
  - Factory pattern for provider registration and selection
  
**Provider Interface**:
```go
type Provider interface {
    InitializeInfrastructure(ctx, cfg) (*InfrastructureOutput, error)
    UpdateInfrastructure(ctx, cfg) error
    DestroyInfrastructure(ctx, cfg) error
    GetEndpoint(ctx, cfg) (string, error)
    ValidateConfig(cfg) error
    GetName() string
}
```

**Provider Selection**:
```bash
# Use AWS (default)
runvoy init --provider aws

# Future: Use GCP
runvoy init --provider gcp

# Future: Use Azure
runvoy init --provider azure
```

Each provider implementation handles cloud-specific operations:
- AWS: CloudFormation stacks, DynamoDB, S3, Lambda, ECS Fargate
- GCP (planned): Deployment Manager, Firestore, Cloud Storage, Cloud Functions, Cloud Run
- Azure (planned): ARM templates, Cosmos DB, Blob Storage, Azure Functions, Container Instances

**Key Commands**:
- `runvoy init` - Deploy infrastructure (admin only, requires AWS credentials)
- `runvoy destroy` - Remove all infrastructure (deletes CloudFormation stacks, S3 bucket)
- `runvoy admin add-user <email>` - Generate API key for new user
- `runvoy admin revoke-user <email>` - Disable user's API key
- `runvoy admin list-users` - Show all users and their status
- `runvoy configure` - Set up CLI with API key
- `runvoy exec "command"` - Execute command remotely
- `runvoy status <exec-id>` - Check execution status
- `runvoy logs <exec-id>` - View execution logs
- `runvoy list` - Show recent executions
- `runvoy locks list` - Show active locks
- `runvoy version` - Show version and embedded assets information

**Embedded Assets Architecture**:

The CLI is designed to be distributed as a self-contained binary with no external dependencies. CloudFormation templates are embedded at build time using Go's `embed` package.

**Structure** (`internal/assets/`):
- `templates.go` - Provides access to embedded CloudFormation templates via `GetCloudFormation*Template()` functions
- `aws/` - AWS CloudFormation templates (organized by cloud provider for future multi-cloud support)
  - `cloudformation-backend.yaml` - Main infrastructure template
  - `cloudformation-lambda-bucket.yaml` - Lambda code bucket template
- `README.md` - Documentation on embedded assets and multi-cloud organization

**How It Works**:
1. CloudFormation templates are maintained in `internal/assets/aws/` (organized by cloud provider)
2. At build time, `//go:embed aws/*.yaml` directive embeds the templates into the binary
3. The AWS provider calls `assets.GetCloudFormation*Template()` instead of reading from disk
4. No file system access required at runtime - binary is fully self-contained
5. Multi-cloud ready: Templates organized by provider (aws/, gcp/, azure/, etc.)

**Keeping Templates in Sync**:
When updating CloudFormation templates:
```bash
# Edit template directly in assets directory
vim internal/assets/aws/cloudformation-backend.yaml

# Rebuild to embed changes
go build
```

**Debugging Embedded Templates**:
```bash
# View embedded templates
runvoy version --show-templates

# View specific template
runvoy version --show-templates --template=backend
runvoy version --show-templates --template=bucket

# Verify templates in binary
strings runvoy | grep "AWSTemplateFormatVersion"
```

**Benefits**:
- ✅ Single binary distribution (no external template files needed)
- ✅ Version consistency (templates guaranteed to match CLI version)
- ✅ Simplified deployment (no asset management required)
- ✅ Reduced errors (no missing or mismatched template files)

**Note**: Templates are edited directly in `internal/assets/aws/` and embedded at build time. The `update-cloudformation.sh` script reads from this directory to update deployed CloudFormation stacks.

**Configuration**:
```yaml
# ~/.runvoy/config.yaml
api_endpoint: https://api.runvoy.company.com
api_key: sk_live_abc123...
region: us-east-2
stack_name: runvoy-backend
bucket_stack_name: runvoy-backend-lambda-bucket
# Note: No AWS credentials stored
```

### 2. Lambda Function URL

**Purpose**: HTTP entry point for CLI requests

Provides a direct HTTPS endpoint for the Lambda function, eliminating the need for API Gateway and simplifying the architecture.

**Benefits over API Gateway**:
- Simpler setup (no API Gateway resources needed)
- Lower cost ($0.60 vs $3.50 per million requests)
- Direct Lambda invocation
- Reduced latency (one less hop)

### 3. Network Infrastructure

**Purpose**: Isolated VPC and networking for ECS Fargate tasks

The CloudFormation template creates a dedicated VPC for runvoy to ensure isolation from other AWS resources and avoid network conflicts with existing infrastructure.

**VPC Configuration**:
- **CIDR Block**: `172.20.0.0/16` (instead of the common `10.0.0.0/16` to avoid conflicts)
- **DNS Support**: Enabled for hostname resolution
- **DNS Hostnames**: Enabled for public IP hostnames
- **Internet Gateway**: Attached for outbound internet access
- **Resource Tagging**: All resources are tagged with the following tags for easy identification and cost tracking:
  - **Name**: `{ProjectName}-{resource-type}` - Human-readable resource name
  - **Application**: `runvoy` - Identifies the application
  - **ManagedBy**: `cloudformation` - Infrastructure management tool

**Subnets** (2 public subnets for high availability):
- `172.20.1.0/24` - Public subnet in AZ 1
- `172.20.2.0/24` - Public subnet in AZ 2
- **Purpose**: Allow ECS tasks to access internet for git clone, AWS API calls, package downloads

**Security Group**:
- **Fargate Security Group**: Allows all outbound traffic (`0.0.0.0/0`)
- **Rationale**: Tasks need internet access for:
  - Git operations (clone repository)
  - AWS API calls (Terraform, AWS CLI operations)
  - Package downloads (apt, pip, npm, etc.)
  - Docker image pulls

**Why `172.20.0.0/16`?**:
- `10.0.0.0/16` is the most commonly used private network range
- Choosing `172.20.0.0/16` reduces likelihood of conflicts with existing infrastructure
- `/16` provides plenty of IP space (65,534 addresses) for future expansion
- If conflicts still occur, this will be made configurable via `runvoy init --vpc-cidr`

**Note**: All subnets are public (not private with NAT) for simplicity and cost optimization in the MVP. ECS tasks get public IPs and can access internet directly via Internet Gateway. No NAT Gateway costs.

### 4. Lambda Orchestrator (Go)

**Purpose**: Validate requests and orchestrate ECS task execution

**Resource Tagging**: Lambda function, execution role, and CloudWatch log groups are tagged with Name, Application, and ManagedBy tags for easy identification and cost tracking.

**Responsibilities**:
1. **Validate API Key**: Check against DynamoDB, ensure not revoked
2. **Check Lock**: Acquire lock if requested, fail if held by another execution
3. **Start ECS Task**: Launch Fargate task with user's command
4. **Record Execution**: Store metadata in DynamoDB
5. **Return Response**: Execution ID, task ARN, log viewer URL

**Environment Variables**:
- `API_KEYS_TABLE` - DynamoDB table name
- `EXECUTIONS_TABLE` - DynamoDB table name
- `LOCKS_TABLE` - DynamoDB table name
- `ECS_CLUSTER` - ECS cluster name
- `ECS_TASK_DEFINITION` - Task definition ARN
- `JWT_SECRET` - Secret for signing log viewer tokens
- `WEB_UI_URL` - Base URL for log viewer

**Flow**:
```python
def handler(event, context):
    # 1. Validate API key
    api_key = event['headers']['X-API-Key']
    user = validate_api_key(api_key)
    if not user:
        return {'statusCode': 401, 'body': 'Invalid API key'}

    # 2. Parse request
    body = json.loads(event['body'])
    command = body['command']
    lock_name = body.get('lock')

    # 3. Acquire lock if requested
    if lock_name:
        acquired = try_acquire_lock(lock_name, user['email'])
        if not acquired:
            holder = get_lock_holder(lock_name)
            return {
                'statusCode': 409,
                'body': f'Lock held by {holder["email"]} since {holder["started"]}'
            }

    # 4. Generate execution ID
    execution_id = generate_execution_id()

    # 5. Start ECS task
    task_arn = start_ecs_task(
        command=command,
        execution_id=execution_id,
        user_email=user['email']
    )

    # 6. Record execution
    record_execution(
        execution_id=execution_id,
        user_email=user['email'],
        command=command,
        task_arn=task_arn,
        lock_name=lock_name
    )

    # 7. Generate log viewer token
    token = generate_log_token(execution_id)
    log_url = f"{WEB_UI_URL}/{execution_id}?token={token}"

    return {
        'statusCode': 200,
        'body': json.dumps({
            'execution_id': execution_id,
            'task_arn': task_arn,
            'log_url': log_url,
            'status': 'starting'
        })
    }
```

### 5. DynamoDB Tables

**Resource Tagging**: All DynamoDB tables are tagged with Name, Application, and ManagedBy tags for easy identification and cost tracking.

#### API Keys Table
```
Partition Key: api_key_hash (string)

Attributes:
- api_key_hash: SHA256 hash of the API key (used for lookup)
- user_email: string
- created_at: timestamp
- revoked: boolean
- last_used: timestamp (updated on each request)

Future attributes (not implemented yet):
- execution_role_arn: IAM role for this user's executions
- allowed_commands: list of command patterns
- allowed_images: list of allowed Docker images
- allowed_locks: list of lock patterns
- groups: list of group names
```

#### Executions Table
```
Partition Key: execution_id (string)
Sort Key: started_at (timestamp) - for time-based queries

Attributes:
- execution_id: string (exec_abc123)
- user_email: string (who ran it)
- command: string (what was executed)
- lock_name: string (if locked)
- task_arn: string (ECS task identifier)
- started_at: timestamp
- completed_at: timestamp
- status: string (starting, running, completed, failed)
- exit_code: number
- duration_seconds: number
- log_stream_name: string
- cost_usd: number (calculated)

GSI: user_email-started_at (for per-user queries)
GSI: status-started_at (for filtering by status)
```

#### Locks Table
```
Partition Key: lock_name (string)

Attributes:
- lock_name: string
- execution_id: string (who holds it)
- user_email: string
- acquired_at: timestamp
- ttl: number (auto-expire after execution timeout)

Note: Lock is automatically released when execution completes
```

### 6. ECS Fargate

**Resource Tagging**: ECS cluster, task definitions, and IAM roles are tagged with Name, Application, and ManagedBy tags for easy identification and cost tracking.

**Task Definition**:
```yaml
Family: runvoy-executor
LaunchType: FARGATE
CPU: 256 (0.25 vCPU) - configurable
Memory: 512 (0.5 GB) - configurable
NetworkMode: awsvpc

ExecutionRole: (for pulling images, writing logs)
  - ecr:GetAuthorizationToken
  - ecr:BatchCheckLayerAvailability
  - ecr:GetDownloadUrlForLayer
  - ecr:BatchGetImage
  - logs:CreateLogStream
  - logs:PutLogEvents

TaskRole: (for actual AWS operations)
  - Initially: AdministratorAccess (MVP)
  - Future: Configurable per-user/group

Container:
  Image: public.ecr.aws/runvoy/executor:latest
  Command: ["/entrypoint.sh"]
  Environment:
    - EXECUTION_ID: (from Lambda)
    - COMMAND: (user's command)
    - USER_EMAIL: (for audit)
    - LOCK_NAME: (if applicable)

  LogConfiguration:
    LogDriver: awslogs
    Options:
      awslogs-group: /runvoy/executions
      awslogs-region: us-east-1
      awslogs-stream-prefix: exec
```

**Container Entrypoint**:
```bash
#!/bin/bash
set -e

# Log execution start
echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] Execution started"
echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] User: $USER_EMAIL"
echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] Command: $COMMAND"

# Set up working directory
mkdir -p /workspace
cd /workspace

# Execute the command (shell script prepared by Lambda)
echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] Running command..."
eval "$COMMAND"
EXIT_CODE=$?

# Log completion
echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] Command completed with exit code: $EXIT_CODE"

# Release lock (Lambda will also handle this via task state change)
if [ -n "$LOCK_NAME" ]; then
    # Call Lambda to release lock
    aws lambda invoke \
        --function-name runvoy-release-lock \
        --payload "{\"lock_name\":\"$LOCK_NAME\",\"execution_id\":\"$EXECUTION_ID\"}" \
        /tmp/response.json
fi

exit $EXIT_CODE
```

**Default Images** (Future):
- `runvoy/executor:terraform` - Terraform + AWS CLI
- `runvoy/executor:ansible` - Ansible + AWS CLI
- `runvoy/executor:python` - Python 3.11 + common tools
- `runvoy/executor:node` - Node.js + common tools
- Custom images via `--image` flag

### 7. CloudWatch Logs

**Log Group**: `/runvoy/executions`

**Log Streams**: One per execution
- Format: `exec/{execution_id}/{task_id}`
- Retention: 7 days (configurable)

**Benefits**:
- Centralized logging
- Searchable
- Integrated with AWS ecosystem
- No additional storage setup

### 8. Web UI (Log Viewer)

**Hosting**: S3 static website + CloudFront (optional)

**Tech Stack**: Single HTML file with embedded JavaScript
- No framework needed (keep it simple)
- Vanilla JS + minimal CSS
- Mobile-responsive

**Features**:
- Real-time log streaming (polling)
- ANSI color support
- Line number linking
- Search/filter
- Copy to clipboard
- Download logs

**Authentication**: JWT token in URL
```
https://runvoy.company.com/{execution_id}?token=eyJ...

Token payload:
{
  "execution_id": "exec_abc123",
  "exp": 1730000000,  // 48 hours from creation
  "aud": "web-viewer"
}
```

**API Endpoints** (separate from main API):
```
GET /api/logs/{execution_id}
  Headers: Authorization: Bearer {token}
  Query: ?since={timestamp} (for polling new logs)

  Response:
  {
    "execution_id": "exec_abc123",
    "status": "running",
    "logs": "...",
    "last_timestamp": "2025-10-26T14:42:45Z",
    "completed": false
  }
```

## Data Flow

### Execution Flow (Detailed)

```
1. User runs command
   $ runvoy exec "terraform apply" --lock infra-prod

2. CLI sends request to Lambda Function URL
   POST /executions
   Headers: X-API-Key: sk_live_abc123...
   Body: {
     "command": "terraform apply",
     "lock": "infra-prod",
     "image": "hashicorp/terraform:1.6",
     "env": {"TF_VAR_region": "us-east-1"},
     "timeout": 1800
   }

3. Lambda validates API key
   - Query DynamoDB: api_key_hash = SHA256(sk_live_abc123...)
   - Check revoked = false
   - Update last_used timestamp
   - Get user_email

4. Lambda attempts to acquire lock
   - Try to write to Locks table with condition:
     attribute_not_exists(lock_name)
   - If fails, query who holds it and return 409
   - If succeeds, continue

5. Lambda generates execution ID
   - Format: exec_{timestamp}_{random}
   - Example: exec_20251026143210_a1b2c3

6. Lambda starts ECS task
   - Task definition: runvoy-executor
   - Override command: ["/entrypoint.sh"]
   - Environment variables:
     * EXECUTION_ID=exec_abc123
     * COMMAND=terraform apply
     * USER_EMAIL=alice@acme.com
     * LOCK_NAME=infra-prod
   - Network: Public subnet with NAT (or private with VPC endpoints)

7. Lambda records execution in DynamoDB
   - execution_id, user_email, command, task_arn
   - started_at, status=starting, lock_name

8. Lambda generates log viewer token
   - JWT signed with secret
   - Expires in 48 hours
   - Contains execution_id

9. Lambda returns response
   {
     "execution_id": "exec_abc123",
     "task_arn": "arn:aws:ecs:...",
     "log_url": "https://runvoy.company.com/exec_abc123?token=...",
     "status": "starting"
   }

10. CLI displays to user
    ✓ Execution started: exec_abc123
    🔗 View logs: https://runvoy.company.com/exec_abc123?token=...
    → Running...

11. ECS task starts
    - Container pulls image
    - Entrypoint script runs
    - Logs to CloudWatch: /runvoy/executions/exec/exec_abc123/{task-id}

12. User opens log viewer URL
    - Static HTML page loads from S3
    - JavaScript extracts execution_id and token from URL
    - Polls API: GET /api/logs/exec_abc123?token=...
    - Displays logs with ANSI colors
    - Polls every 2 seconds while status != completed

13. Task completes
    - Exit code captured
    - CloudWatch receives final logs
    - Task state change event triggers Lambda (optional)

14. Lambda updates execution record
    - status=completed/failed
    - completed_at, exit_code, duration_seconds
    - Releases lock (deletes from Locks table)

15. User sees completion in web UI
    ✓ Completed in 10m 35s
    Exit code: 0
    [Logs with final output]
```

### Lock Acquisition Flow

```
Request with lock:
  POST /executions
  Body: {"command": "...", "lock": "infra-prod"}

Lambda tries to acquire:
  DynamoDB PutItem with condition expression:
    attribute_not_exists(lock_name)

  Item: {
    lock_name: "infra-prod",
    execution_id: "exec_abc123",
    user_email: "alice@acme.com",
    acquired_at: "2025-10-26T14:32:10Z",
    ttl: 1730000000  // Auto-expire after timeout
  }

Success → Continue with execution

Failure (ConditionalCheckFailedException):
  Query lock to see who holds it:
    GetItem(lock_name="infra-prod")

  Return 409 Conflict:
    {
      "error": "Lock held",
      "lock_name": "infra-prod",
      "held_by": "alice@acme.com",
      "since": "2025-10-26T14:32:10Z",
      "execution_id": "exec_abc123",
      "log_url": "https://..."
    }

On completion:
  DeleteItem(lock_name="infra-prod")

  Or rely on TTL to auto-expire if task crashes
```

## Security Model

### Authentication Layers

1. **CLI to Lambda Function URL**: API key in header (`X-API-Key`)
2. **Lambda execution**: AWS IAM role (Lambda invokes Lambda directly)
3. **Web UI to Log API**: JWT token in URL/header
4. **ECS Task to AWS**: IAM Task Role

### Secrets Management

**What's stored where**:
- API keys: DynamoDB (SHA256 hashed for lookup)
- JWT signing secret: Lambda environment variable (or Secrets Manager)
- AWS credentials: Never stored (IAM roles everywhere)

**User never sees**:
- AWS access keys
- AWS secret keys
- Other users' API keys (only their own)

### Network Security

**ECS Tasks**:
- Run in VPC
- Option 1: Public subnet with NAT gateway (internet access)
- Option 2: Private subnet with VPC endpoints (no internet)
- Security group: Egress only (no ingress needed)

**Lambda Function URL**:
- Public endpoint (HTTPS only)
- API key validation in Lambda handler
- CORS configured for web access

### Audit Trail

Every execution records:
- Who (`user_email`)
- What (`command`)
- When (`started_at`, `completed_at`)
- Where (`task_arn`, `log_stream_name`)
- Result (`exit_code`, `status`)
- Cost (`cost_usd`)

This satisfies compliance requirements:
- SOC 2: Access logging
- HIPAA: Audit trails
- PCI DSS: User activity tracking

## Resource Tagging Strategy

### Tagging Standard

All resources in the CloudFormation stack are tagged with consistent tags for easy identification, cost tracking, and resource management:

- **Name**: `{ProjectName}-{resource-type}` - Human-readable resource identifier for easy identification in AWS Console
  - Examples: `runvoy-api-keys`, `runvoy-vpc`, `runvoy-orchestrator`
  
- **Application**: `runvoy` - Identifies all resources belonging to the runvoy application
  - Enables filtering and cost allocation by application in AWS Cost Explorer
  
- **ManagedBy**: `cloudformation` - Infrastructure management tool
  - Indicates resources are managed by CloudFormation/Infrastructure as Code
  - Helps distinguish between manually created and IaC-managed resources

### Tagged Resources

All the following resource types receive these tags:
- **Compute**: ECS Cluster, Task Definitions, Lambda Functions
- **Storage**: DynamoDB Tables, S3 Buckets
- **Networking**: VPC, Subnets, Route Tables, Internet Gateway, Security Groups
- **IAM**: Task Execution Role, Task Role, Lambda Execution Role
- **Monitoring**: CloudWatch Log Groups

### Benefits

**Cost Management**:
- Use AWS Cost Explorer to filter costs by `Application = runvoy`
- Track spending for the entire runvoy deployment in one view
- Enable cost allocation tags for detailed reporting

**Resource Management**:
- Easily identify all runvoy resources in AWS Console using the `Application` tag
- Filter resources across all AWS services using the tag filter
- Quickly distinguish runvoy resources from other infrastructure

**Governance & Compliance**:
- Use AWS Tag Policies to enforce consistent tagging
- Audit compliance with organizational tagging standards
- Track resource ownership and management tooling

### Example Usage

```bash
# Find all runvoy resources using AWS CLI
aws resourcegroupstaggingapi get-resources --tag-filters Key=Application,Values=runvoy

# Filter costs in AWS Cost Explorer
# Use tag filter: Application = runvoy

# Check resource management
aws resourcegroupstaggingapi get-resources \
  --tag-filters Key=ManagedBy,Values=cloudformation
```

## Deployment Model

### Single Tenant per Company

Each company gets one runvoy deployment:
```
Company "Acme Corp" → AWS Account 123456789
  └─ runvoy CloudFormation stack
     ├─ Lambda Function URL: https://xxx.lambda-url.region.on.aws/
     ├─ Lambda orchestrator
     ├─ DynamoDB tables
     ├─ ECS cluster
     └─ S3 bucket

  Users:
  ├─ alice@acme.com → API key sk_live_abc...
  ├─ bob@acme.com → API key sk_live_def...
  └─ carol@acme.com → API key sk_live_ghi...
```

### Deployment Steps

1. **Admin deploys infrastructure**:
   ```bash
   $ aws configure  # Uses admin AWS credentials
   $ runvoy init --provider aws --stack-name runvoy-backend --region us-east-2
   → Generating API key...
   → Building Lambda function...
   → Creating S3 bucket stack for Lambda code (Stack 1)...
   ✓ Lambda bucket stack created
   → Uploading Lambda code to S3...
   ✓ Lambda code uploaded
   → Creating main CloudFormation stack (Stack 2)...
   ✓ Main stack created successfully
   → Configuring API key...
   ✓ API key configured
   → Saving configuration...
   ✓ Setup complete!

   🔑 Your API key: sk_live_abc123...
   ```

   **What `runvoy init` does**:
   - Generates a random API key (sk_live_...)
   - Computes SHA256 hash of the API key
   - Builds the Lambda orchestrator binary (Go → ARM64 Linux)
   - Creates a temporary S3 bucket stack via CloudFormation (for Lambda code)
   - Uploads the Lambda ZIP to S3
   - Creates the main CloudFormation stack with all infrastructure:
     - VPC, subnets, internet gateway, security groups
     - ECS Fargate cluster and task definitions
     - Lambda function (loaded from S3)
     - Lambda Function URL
     - DynamoDB tables (API Keys, Executions, Locks)
     - IAM roles (Lambda execution, ECS task, ECS task execution)
     - CloudWatch log groups
   - Inserts the generated API key into DynamoDB (SHA256 hashed)
   - Saves the configuration to `~/.runvoy/config.yaml` (includes API endpoint, API key, region, main stack name, and bucket stack name)

   **Note**: Git credentials (GitHub/GitLab tokens, SSH keys) are NOT currently supported.
   The Lambda orchestrator does not implement git cloning yet. This is a planned feature.

   **What `runvoy destroy` does** (`cmd/destroy.go`, `internal/provider/aws.go`):
   ```bash
   $ runvoy destroy --region us-east-2
   🗑️  Destroying runvoy infrastructure...
   
   → Deleting main CloudFormation stack...
   → Emptying S3 bucket...
   → Deleting bucket stack...
   ✓ Destruction complete!
   ```
   
   Steps:
   1. Deletes the main CloudFormation stack (Lambda, API Gateway, DynamoDB, ECS, VPC, etc.)
   2. Empties the S3 bucket by deleting all object versions and delete markers
   3. Deletes the Lambda bucket CloudFormation stack
   4. Removes the local configuration file (`~/.runvoy/config.yaml`) unless `--keep-config` is used
   
   Implementation (`internal/provider/aws.go:129-188`):
   - `DestroyInfrastructure()` orchestrates the entire teardown process
   - `deleteStack()` deletes a CloudFormation stack and waits for completion (up to 15 minutes)
   - `emptyBucket()` uses ListObjectVersions to delete all versions of all objects
   - `getBucketNameFromStack()` retrieves the bucket name from stack outputs, or constructs it if the stack is already deleted
   - Bucket name format: `{stackname}-lambda-code-{accountId}-{region}` (`internal/assets/aws/cloudformation-lambda-bucket.yaml:16`)
   
   **Note**: The destroy process requires confirmation by typing "delete" unless `--force` is used.

2. **Admin generates API keys for other users**:
   ```bash
   $ runvoy admin add-user alice@acme.com
   ✓ API key: sk_live_abc123...
     Share this with alice@acme.com
   ```

3. **Users configure CLI**:
   ```bash
   $ runvoy configure
   API Endpoint: https://api.runvoy.acme.internal
   API Key: sk_live_abc123...
   ✓ Configuration saved
   ```

4. **Users execute commands**:
   ```bash
   $ runvoy exec "terraform apply"
   ✓ Running in Acme's AWS account
   ```

### Multi-Environment (Optional)

Companies can deploy multiple instances:
```bash
# Production instance
$ runvoy init --company acme --environment prod

# Staging instance (separate AWS account or separate stack)
$ runvoy init --company acme --environment staging

# Users configure for different environments
$ runvoy configure --profile acme-prod
$ runvoy configure --profile acme-staging

$ runvoy exec "terraform apply" --profile acme-prod
```

## Scalability

### Current Design (MVP)

- **Concurrency**: Fargate scales automatically (up to AWS service limits)
- **Cost**: Pay-per-execution (no idle costs except DynamoDB and small Lambda)
- **Limits**:
  - Lambda Function URL: 1000 RPS per URL (default)
  - Lambda: 1,000 concurrent executions (default)
  - Fargate: 1,000 tasks per cluster (default, can increase)
  - DynamoDB: On-demand scaling (no hard limit)

### Bottlenecks & Solutions

**If many users execute simultaneously**:
- Problem: DynamoDB hot partitions (lock table)
- Solution: Use consistent hashing for lock names, or shard lock table

**If log viewer gets popular**:
- Problem: S3 request rate limits
- Solution: Add CloudFront CDN in front of S3

**If executions are very long**:
- Problem: ECS task limit (1000 concurrent)
- Solution: Request limit increase from AWS, or queue executions

**If audit table grows large**:
- Problem: DynamoDB scan operations slow
- Solution: Use GSIs for common queries, archive old data to S3

## Monitoring & Observability

### Metrics to Track

**Operational**:
- Executions per hour
- Success rate (completed vs failed)
- Average execution duration
- Lock contention (failed lock acquisitions)
- API key usage per user

**Performance**:
- Lambda cold start time
- Lambda Function URL latency
- ECS task start time
- Log fetch latency

**Cost** (future):
- Fargate compute cost
- CloudWatch Logs cost
- S3 storage cost
- Total cost per execution
- Cost per user

### Alerts

**Critical**:
- Lambda execution errors > 5% in 5 minutes
- ECS task failure rate > 10% in 5 minutes
- Lambda Function URL 5xx errors > 1% in 5 minutes

**Warning**:
- High lock contention (many 409 responses)
- Unusual execution duration (>2x average)
- DynamoDB throttling

### Logs

**CloudWatch Log Groups**:
- `/aws/lambda/runvoy-orchestrator` - Lambda logs
- `/runvoy/executions` - Execution output logs

**Log Retention**:
- Lambda logs: 30 days
- Execution logs: 7 days (configurable)

## Future Architecture Enhancements

### Phase 1: Role-Based Authorization
- Multiple IAM roles (read-only, admin, custom)
- Assign role per user/group
- Lambda selects appropriate role for ECS task

### Phase 2: Advanced Permissions
- Command filtering (allow/deny patterns)
- Image restrictions (approved images only)
- Lock-based restrictions (prod access only for some users)

### Phase 3: Multi-Region
- Deploy to multiple regions
- Users specify region with `--region` flag
- Reduces latency for global teams

### Phase 4: SaaS Mode
- Anthropic hosts the infrastructure
- Companies sign up, get isolated environments
- Billing per execution or per user
- No AWS account needed

### Phase 5: Advanced Features
- Approval workflows (require approval before execution)
- Scheduled executions (cron-like)
- Execution templates/runbooks
- Integration with CI/CD (GitHub Actions, GitLab CI)
- Multi-cloud support (GCP, Azure)

## Technology Choices

### Why Go for CLI?
- Single binary distribution (no dependencies)
- Cross-platform (Linux, macOS, Windows)
- Fast execution
- Great AWS SDK support
- Cobra for CLI framework

### Why Python for Lambda?
- Fast development
- Excellent AWS SDK (boto3)
- Easy to read and maintain
- Could switch to Go later for performance

### Why DynamoDB?
- Serverless (no management)
- Scales automatically
- Perfect for key-value lookups
- Atomic operations for locking
- Pay-per-request pricing

### Why Fargate?
- Serverless compute (no EC2 management)
- Scales automatically
- Isolated environments
- Pay-per-second pricing
- Easy to use different images

### Why CloudWatch Logs?
- Native AWS integration
- No additional setup
- Searchable
- Long-term retention
- Integrates with other AWS services

## Cost Estimation

### Small Team (10 users, 50 executions/day)

**Monthly costs**:
- Fargate: 50 exec/day × 10 min avg × 0.25 vCPU × $0.04048/vCPU-hour
  - = 50 × (10/60) × 0.25 × 0.04048 × 30 = **$0.25**
- Lambda: 50 exec/day × 1 sec × 128MB × 30 days
  - = Nearly free (within free tier)
- DynamoDB: Minimal reads/writes
  - = **$0.50** (on-demand)
- CloudWatch Logs: 50 exec × 5MB × 30 days × $0.50/GB
  - = **$3.75**
- S3: Negligible
- Lambda Function URL: 1,500 requests/month
  - = **$0.001**

**Total: ~$5/month**

### Medium Team (50 users, 500 executions/day)

**Monthly costs**:
- Fargate: **$2.50**
- Lambda: **$0.10**
- DynamoDB: **$5.00**
- CloudWatch Logs: **$37.50**
- S3: **$1.00**
- Lambda Function URL: **$0.01**

**Total: ~$46/month**

### Large Team (200 users, 2000 executions/day)

**Monthly costs**:
- Fargate: **$10.00**
- Lambda: **$0.40**
- DynamoDB: **$20.00**
- CloudWatch Logs: **$150.00**
- S3: **$5.00**
- Lambda Function URL: **$0.05**

**Total: ~$186/month**

**Note**: CloudWatch Logs dominates cost at scale. Consider:
- Shorter retention period (3 days instead of 7)
- Archive to S3 after 1 day (cheaper storage)
- Stream to external log service

## Comparison to Alternatives

| Solution | Setup | Cost (50 users) | Pros | Cons |
|----------|-------|-----------------|------|------|
| **runvoy** | 5 min | $46/mo | Self-hosted, full control, audit trails | Requires AWS knowledge |
| **Terraform Cloud** | 10 min | $1000/mo | Terraform-specific features | Expensive, vendor lock-in |
| **Jenkins** | 2 hours | $100/mo | Very flexible | Complex, requires maintenance |
| **GitHub Actions** | 5 min | $200/mo | Integrated with git | Git-based only, no ad-hoc |
| **AWS CodeBuild** | 30 min | $50/mo | Native AWS | Complex setup, build-focused |
| **Shared credentials** | 1 min | $0 | Simple | Insecure, no audit, conflicts |

runvoy wins on: simplicity, cost, audit trails, and general-purpose execution.

---

This architecture balances simplicity (for MVP) with extensibility (for future features). The core design supports authorization, multi-tenancy, and advanced features without major refactoring.
