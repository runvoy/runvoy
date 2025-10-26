# mycli

Remote command execution in isolated, ephemeral containers. Run any command in your own AWS infrastructure without local dependency conflicts.

## Features

- **One-command setup** - Deploy complete infrastructure with `mycli init`
- **Git-integrated** - Automatically clones your repository before execution
- **Flexible images** - Use any Docker image (terraform, python, node, etc.)
- **Self-hosted** - Runs in your AWS account, you control everything
- **Secure** - API key authentication, encrypted Git credentials
- **Audit trail** - Full execution logs in CloudWatch

## Quick Start

### 1. Install

```bash
go build -o mycli
```

### 2. Initialize Infrastructure

```bash
mycli init
```

This creates:
- ECS Fargate cluster for running tasks
- Lambda orchestrator with API Gateway
- VPC with public subnets
- CloudWatch Logs for execution output
- Optionally configures Git credentials for private repos

### 3. Execute Commands

**With a project configuration file:**

Create `.mycli.yaml` in your project directory:
```yaml
repo: https://github.com/mycompany/infrastructure
branch: main
image: hashicorp/terraform:1.6
env:
  TF_VAR_region: us-east-1
```

Then run:
```bash
mycli exec "terraform plan"
```

**Without configuration (explicit flags):**

```bash
mycli exec \
  --repo=https://github.com/user/infra \
  --image=hashicorp/terraform:1.6 \
  "terraform apply"
```

**Auto-detect from git repository:**

```bash
cd my-git-repo  # has git remote
mycli exec "make deploy"
```

**Run without Git cloning:**

```bash
mycli exec --skip-git --image=alpine:latest "echo hello world"
```

### 4. Monitor Execution

```bash
# Check status
mycli status <task-arn>

# View logs
mycli logs <execution-id>

# Follow logs in real-time
mycli logs -f <execution-id>
```

## How It Works

```
1. You run: mycli exec "terraform plan"
2. CLI calls your Lambda function via API Gateway
3. Lambda constructs a shell script that:
   - Installs git (if needed)
   - Clones your repository
   - Executes your command
4. Lambda starts ECS Fargate task with chosen Docker image
5. Container runs the script, logs output to CloudWatch
6. You monitor via: mycli status / mycli logs
```

## Configuration

### Global Config (~/.mycli/config.yaml)

Created automatically by `mycli init`:
```yaml
api_endpoint: https://....execute-api.us-east-1.amazonaws.com/prod/execute
api_key: sk_live_...
region: us-east-1
```

### Project Config (.mycli.yaml)

Optional file in your project directory:
```yaml
repo: https://github.com/company/project
branch: main
image: hashicorp/terraform:1.6
env:
  TF_VAR_environment: production
  AWS_REGION: us-east-1
timeout: 3600
```

### Configuration Priority

1. **Command-line flags** (highest priority)
2. **.mycli.yaml** in current directory
3. **Git remote auto-detection**
4. **Error** if no repo specified

## Commands

### `mycli init`
Deploy infrastructure to your AWS account

Options:
- `--stack-name` - CloudFormation stack name (default: "mycli")
- `--region` - AWS region (default: from AWS config or us-east-2)

### `mycli exec [flags] "command"`
Execute a command remotely

Flags:
- `--repo` - Git repository URL
- `--branch` - Git branch to checkout (default: main)
- `--image` - Docker image to use
- `--env KEY=VALUE` - Environment variables (repeatable)
- `--timeout` - Timeout in seconds (default: 1800)
- `--skip-git` - Skip git cloning, run command directly

### `mycli status <task-arn>`
Check execution status

### `mycli logs <execution-id>`
View execution logs

Flags:
- `-f, --follow` - Stream logs in real-time

### `mycli configure`
Manually configure CLI (for existing infrastructure)

### `mycli destroy`
Delete all infrastructure

Options:
- `--stack-name` - Stack to delete (default: "mycli")
- `--force` - Skip confirmation

## Use Cases

- **Infrastructure as Code** - Run Terraform, Pulumi, CloudFormation
- **Configuration Management** - Execute Ansible, Chef, Puppet playbooks
- **CI/CD Tasks** - Run build scripts, tests, deployments
- **Data Processing** - Execute Python scripts, data pipelines
- **One-off Commands** - Any task requiring isolation and audit trail

## Examples

**Terraform workflow:**
```bash
mycli exec "terraform init && terraform plan"
mycli exec "terraform apply -auto-approve"
```

**Python script:**
```bash
mycli exec --image=python:3.11 "python script.py"
```

**Multi-environment:**
```bash
mycli exec --branch=dev "terraform plan"
mycli exec --branch=prod "terraform apply"
```

**Custom environment variables:**
```bash
mycli exec --env TF_VAR_region=us-west-2 --env ENV=staging "terraform apply"
```

## Cost

Very low cost for typical usage:

**Per execution** (5 min, 0.25 vCPU, 0.5 GB):
- Fargate SPOT: ~$0.0003
- Lambda: ~$0.0000002
- API Gateway: ~$0.0000035
- **Total: ~$0.00033 per execution**

**Monthly estimates:**
- 100 executions: ~$0.03
- 1,000 executions: ~$0.33
- 10,000 executions: ~$3.30

## Security

- **API Authentication** - Bcrypt-hashed API keys
- **Git Credentials** - Encrypted in Lambda environment variables
- **Network Isolation** - VPC with security groups, egress-only
- **Audit Trail** - All executions logged to CloudWatch
- **IAM Roles** - Configurable task permissions

## Limitations

- Commands limited to ~30 minutes (configurable)
- No persistent artifact storage (use S3 from your commands)
- No real-time log streaming (poll-based only)
- Single command per execution (use scripts for multi-step workflows)

## Development

```bash
# Build CLI
go build -o mycli

# Build Lambda locally (optional, init does this automatically)
cd lambda/orchestrator
GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bootstrap main.go

# Deploy infrastructure
./mycli init --region us-east-1

# Test
./mycli exec --repo=https://github.com/hashicorp/terraform-guides "ls -la"

# Cleanup
./mycli destroy
```

## Architecture

See [ARCHITECTURE.md](./ARCHITECTURE.md) for detailed system design, implementation notes, and technical decisions.

## License

MIT
