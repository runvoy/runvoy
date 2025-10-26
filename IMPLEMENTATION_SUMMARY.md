# mycli Git-based Backend Implementation Summary

## Overview

Successfully implemented the **Git-based remote execution backend** for mycli, eliminating the need for S3 code storage and enabling direct execution from Git repositories.

## Key Architecture Decision

✅ **No Go wrapper needed in the container!** 

We use a simple, elegant bash script (`entrypoint.sh`) that:
- Clones Git repositories
- Configures authentication
- Executes user commands
- Handles errors gracefully

This is much simpler and more maintainable than a Go wrapper.

## Implementation Details

### 1. Lambda Orchestrator (`lambda/orchestrator/main.go`)

**Changes:**
- Accepts Git parameters: `repo`, `branch`, `command`, `image`, `env`, `timeout_seconds`
- Passes Git credentials (GitHub token, GitLab token, SSH key) to containers
- Supports custom Docker images per execution
- Generates unique execution IDs
- Tags ECS tasks with execution metadata

**API Contract:**
```json
POST /execute
{
  "action": "exec",
  "repo": "https://github.com/user/infrastructure",
  "branch": "main",
  "command": "terraform apply",
  "image": "hashicorp/terraform:1.6",
  "env": {
    "TF_VAR_region": "us-east-1"
  },
  "timeout_seconds": 1800
}
```

### 2. Docker Executor Container (`docker/`)

**Components:**

#### `entrypoint.sh` (Simple Bash Script)
- Validates required environment variables (`REPO_URL`, `USER_COMMAND`)
- Configures Git authentication (GitHub token, GitLab token, or SSH key)
- Clones repository with `git clone --depth 1 --branch $REPO_BRANCH`
- Executes user command with proper error handling
- Cleans up credentials after execution
- Exit code reflects command success/failure

#### `Dockerfile`
Pre-installed tools:
- **Core:** Git, SSH client, curl, wget, jq
- **Cloud:** AWS CLI v2
- **IaC:** Terraform, Ansible
- **Kubernetes:** kubectl, Helm
- **Languages:** Python 3, pip
- **Libraries:** boto3, PyYAML, requests

**Image size optimization:**
- Based on Ubuntu 22.04 LTS
- Multi-stage build support
- ~800MB with all tools

#### `Makefile`
- `make build` - Build locally
- `make build-multi` - Build for multiple architectures
- `make push-ecr` - Push to AWS ECR Public
- `make test` - Test with public repo

### 3. CloudFormation Template (`cmd/cloudformation.yaml`)

**Changes:**
- **Removed:** S3 bucket dependency
- **Added:** Git credential parameters (optional):
  - `GitHubToken`
  - `GitLabToken`
  - `SSHPrivateKey` (base64-encoded)
- **Simplified:** Task role (no S3 permissions needed)
- **Container:** Uses placeholder image (overridden at runtime)

**Resources:**
- VPC with public subnets (for internet access to Git)
- ECS Fargate cluster with FARGATE_SPOT for cost savings
- Lambda execution role with ECS permissions
- API Gateway REST API
- CloudWatch Log Group

### 4. CLI Implementation

#### Project Config Parser (`internal/project/config.go`)
- Reads `.mycli.yaml` from project root
- Supports config merging (CLI flags override file values)
- Validates required fields
- Environment variable merging

#### Git Auto-detection (`internal/git/detector.go`)
- Detects Git remote URL: `git remote get-url origin`
- Detects current branch: `git rev-parse --abbrev-ref HEAD`
- Checks if directory is a Git repository
- Provides complete repository info

#### Updated Exec Command (`cmd/exec.go`)
**Configuration Priority:**
1. Command-line flags (`--repo`, `--branch`, `--image`, `--env`)
2. `.mycli.yaml` in current directory
3. Git remote URL (auto-detected)
4. Error if no repo specified

**Example usage:**
```bash
# With .mycli.yaml
cd my-terraform-project
mycli exec "terraform plan"

# Override branch
mycli exec --branch=dev "terraform plan"

# Explicit repo
mycli exec --repo=https://github.com/user/infra "terraform apply"

# Auto-detect from git remote
cd my-git-repo
mycli exec "make deploy"
```

#### Updated Init Command (`cmd/init.go`)
**Changes:**
- **Removed:** S3 bucket creation
- **Added:** Interactive Git credential configuration
  - GitHub Personal Access Token
  - GitLab Personal Access Token
  - SSH Private Key (with base64 encoding)
- Stores credentials in Lambda environment variables
- Passes credentials via CloudFormation parameters

### 5. API Client (`internal/api/client.go`)
- Updated `ExecRequest` to include Git parameters
- Sends complete execution configuration to Lambda
- Handles new response fields (`log_stream`, `created_at`)

## Configuration Files

### `.mycli.yaml` (Project Config)
```yaml
repo: https://github.com/mycompany/infrastructure
branch: main
image: hashicorp/terraform:1.6
env:
  TF_VAR_environment: production
  TF_VAR_region: us-east-1
timeout: 3600
```

### `~/.mycli/config.yaml` (User Config)
```yaml
api_endpoint: https://xxx.execute-api.us-east-1.amazonaws.com/prod/execute
api_key: sk_live_xxx
region: us-east-1
```

## Security Considerations

### Git Credentials
1. **Storage:** Stored in Lambda environment variables (encrypted at rest)
2. **Transmission:** Passed to containers via ECS task environment
3. **Cleanup:** Automatically cleaned up after each execution
4. **SSH Keys:** Base64-encoded for safe environment variable transmission

### API Authentication
- Bcrypt-hashed API key
- Key stored in Lambda environment
- CLI stores plaintext key in `~/.mycli/config.yaml` (600 permissions)

### Container Isolation
- Fresh container per execution
- No persistent storage
- Credentials cleaned up after execution
- Exit code propagation for proper error handling

## Testing Checklist

### Docker Container
- [ ] Build: `cd docker && make build`
- [ ] Test public repo: `make test`
- [ ] Test private repo (GitHub): `REPO_URL=... GITHUB_TOKEN=... make test-custom`
- [ ] Verify Git authentication works
- [ ] Check logs for credential cleanup

### Lambda Function
- [ ] Build: Lambda builds automatically during `mycli init`
- [ ] Test locally with SAM (optional)
- [ ] Deploy and test via API Gateway

### CLI Commands
- [ ] `mycli init` - Full deployment with Git credentials
- [ ] `mycli exec` with `.mycli.yaml`
- [ ] `mycli exec` with CLI flags
- [ ] `mycli exec` with git auto-detect
- [ ] `mycli status <task-arn>`
- [ ] `mycli logs <execution-id>`
- [ ] `mycli logs -f <execution-id>` (follow mode)

### Integration Test Scenarios
1. **Public repo:** `mycli exec --repo=https://github.com/hashicorp/terraform-guides "ls -la"`
2. **Private repo (GitHub):** Configure GitHub token, execute
3. **Private repo (SSH):** Configure SSH key, execute
4. **With .mycli.yaml:** Create config, execute
5. **Git auto-detect:** Run from git repo without config
6. **Custom image:** `mycli exec --image=python:3.11 "python --version"`
7. **Environment variables:** `mycli exec --env FOO=bar "env | grep FOO"`

## Deployment Instructions

### 1. Build and Push Executor Image

```bash
cd docker

# Build for ARM64 (Fargate)
docker buildx build --platform linux/arm64 -t mycli/executor:latest .

# Push to ECR Public
aws ecr-public get-login-password --region us-east-1 | \
  docker login --username AWS --password-stdin public.ecr.aws

# Tag and push
docker tag mycli/executor:latest public.ecr.aws/YOUR_ALIAS/mycli-executor:latest
docker push public.ecr.aws/YOUR_ALIAS/mycli-executor:latest
```

### 2. Deploy Infrastructure

```bash
# From project root
go build -o mycli

# Initialize (deploys to AWS)
./mycli init --region us-east-1

# Follow prompts to configure Git credentials
```

### 3. Test Execution

```bash
# Test with public repo
./mycli exec --repo=https://github.com/hashicorp/terraform-guides "ls -la && cat README.md"

# Check status
./mycli status <task-arn>

# View logs
./mycli logs <execution-id>
```

## Benefits of This Implementation

### Simplicity
- ✅ No S3 bucket management
- ✅ No code upload/download overhead
- ✅ Simple bash entrypoint (no Go wrapper)
- ✅ Git as the single source of truth

### Flexibility
- ✅ Support for any Docker image
- ✅ Multiple Git authentication methods
- ✅ Per-execution customization
- ✅ Environment variable passing

### Developer Experience
- ✅ `.mycli.yaml` for project config
- ✅ Git auto-detection
- ✅ Clear configuration priority
- ✅ Helpful error messages

### Cost Efficiency
- ✅ No S3 storage costs
- ✅ FARGATE_SPOT for compute
- ✅ Shallow git clones (`--depth 1`)
- ✅ Pay only for execution time

## Future Enhancements

### Short-term
- [ ] Build and publish official mycli executor image to ECR Public
- [ ] Add support for more Git providers (Bitbucket, etc.)
- [ ] Support for Git submodules
- [ ] Support for monorepos (working directory override)

### Medium-term
- [ ] Execution history in DynamoDB
- [ ] Web dashboard for monitoring
- [ ] Slack/Discord notifications
- [ ] Scheduled executions (cron-like)

### Long-term
- [ ] Multi-tenancy support
- [ ] SaaS offering
- [ ] Execution templates/workflows
- [ ] Cost tracking and optimization

## Migration from S3-based Architecture

If you have an existing S3-based deployment:

1. **Data:** No migration needed (Git is source of truth)
2. **Infrastructure:** Destroy old stack, deploy new one
3. **CLI:** Update to new version
4. **Config:** Update `.mycli.yaml` format (add `repo` field)

## Questions & Answers

**Q: Why not use S3 for code storage?**
A: Git is already the source of truth for code. Using S3 adds complexity, cost, and latency without significant benefits.

**Q: Why bash instead of Go for the container entrypoint?**
A: Bash is perfect for this use case:
- Simple git clone and command execution
- No compilation needed
- Easy to read and modify
- Reduces container image size
- No need for complex error handling (exit codes are sufficient)

**Q: How are secrets handled?**
A: Git credentials are stored in Lambda environment variables (encrypted at rest by AWS), passed to containers, and cleaned up after execution. User commands should use AWS Secrets Manager or similar for application secrets.

**Q: Can I use private Docker images?**
A: Yes, configure ECS task execution role with ECR permissions and provide the private image URL via `--image` flag or `.mycli.yaml`.

**Q: What about large repositories?**
A: Use shallow clones (`--depth 1`) to minimize clone time. For monorepos, consider using git sparse-checkout (future enhancement).

## Conclusion

The Git-based architecture is **simpler, faster, and more maintainable** than the S3-based approach. By leveraging Git as the source of truth and using a simple bash entrypoint script, we've created an elegant solution that's easy to understand, extend, and operate.

**Status:** ✅ Ready for testing and deployment
