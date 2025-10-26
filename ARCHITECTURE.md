# mycli Architecture

## Design Philosophy

**Simplicity over custom solutions** - Use standard Docker images and tools rather than maintaining custom containers.

## Architecture Overview

```
┌─────────────┐
│  CLI User   │
└──────┬──────┘
       │ mycli exec "terraform plan"
       │
       ▼
┌──────────────────┐
│  API Gateway     │
│  REST API        │
└──────┬───────────┘
       │
       ▼
┌─────────────────────────────────────────────────────┐
│  Lambda Orchestrator                                │
│  - Validates API key                                │
│  - Constructs shell command:                        │
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

## Key Components

### 1. Lambda Orchestrator

**Language:** Go  
**Runtime:** Custom runtime (provided.al2023, ARM64)

**Responsibilities:**
- API key authentication (bcrypt)
- Request validation
- **Shell command construction** - Builds a shell script that:
  - Installs git if not present
  - Configures git credentials (GitHub/GitLab token or SSH key)
  - Clones the repository
  - Changes to repo directory
  - Executes user's command
  - Cleans up credentials
- ECS task orchestration with command override
- Task metadata management

**Why Go?**
- Fast cold starts
- Small binary size
- Strong typing for safety
- Good AWS SDK support

### 2. Container Execution Model

**NO custom Docker image required!** Users can specify any image:
- `hashicorp/terraform:1.6` - For Terraform
- `python:3.11` - For Python scripts
- `node:18` - For Node.js
- `ubuntu:22.04` - Generic fallback
- Any other public/private Docker image

**How it works:**
1. Lambda constructs a shell script with git clone + user command
2. ECS task override sets: `Command: ["/bin/sh", "-c", "<constructed_script>"]`
3. Container executes the script which:
   - Installs git if needed (apt-get, apk, or yum)
   - Configures credentials
   - Clones repo to `/workspace/repo`
   - Runs user's command in that directory
   - Exits with the command's exit code

### 3. Git Credentials

Stored in Lambda environment variables (encrypted at rest by AWS):
- `GITHUB_TOKEN` - GitHub Personal Access Token
- `GITLAB_TOKEN` - GitLab Personal Access Token  
- `SSH_PRIVATE_KEY` - Base64-encoded SSH private key

Passed to container via shell script, cleaned up after execution.

### 4. Configuration Priority

When executing `mycli exec "command"`:

1. **Command-line flags** (highest priority)
   - `--repo`, `--branch`, `--image`, `--env`, `--timeout`
2. **`.mycli.yaml`** in current directory
3. **Git remote auto-detection** (`git remote get-url origin`)
4. **Error** if no repo specified

Example:
```bash
# Uses config from .mycli.yaml
mycli exec "terraform plan"

# Overrides branch from .mycli.yaml
mycli exec --branch=dev "terraform plan"

# Overrides image
mycli exec --image=hashicorp/terraform:1.7 "terraform plan"

# Auto-detect git repo
cd my-git-repo && mycli exec "make deploy"
```

## Data Flow

### Execution Flow

```
1. User runs: mycli exec --repo=https://github.com/user/infra "terraform apply"

2. CLI sends to API Gateway:
   POST /execute
   {
     "action": "exec",
     "repo": "https://github.com/user/infra",
     "branch": "main",
     "command": "terraform apply",
     "image": "hashicorp/terraform:1.6",
     "env": {"TF_VAR_region": "us-east-1"}
   }

3. Lambda constructs shell script:
   #!/bin/sh
   set -e
   # Install git if needed
   apt-get update && apt-get install -y git
   # Configure GitHub auth
   git config --global credential.helper store
   echo "https://TOKEN@github.com" > ~/.git-credentials
   # Clone repo
   git clone --depth 1 --branch main https://github.com/user/infra /workspace/repo
   cd /workspace/repo
   # Execute user command
   terraform apply
   # Cleanup
   rm -f ~/.git-credentials
   exit $?

4. Lambda starts ECS task:
   - Image: hashicorp/terraform:1.6
   - Command: ["/bin/sh", "-c", "<script above>"]
   - Environment: {EXECUTION_ID: "exec_...", TF_VAR_region: "us-east-1"}

5. Fargate executes:
   - Downloads hashicorp/terraform:1.6 image
   - Runs the shell script
   - Logs output to CloudWatch
   - Exits with terraform's exit code

6. User checks status and logs:
   mycli status <task-arn>
   mycli logs <execution-id>
```

### No Persistent Storage

- ✅ Git is the source of truth
- ✅ No S3 bucket needed
- ✅ No code upload/download
- ✅ Shallow clone for speed (`--depth 1`)
- ✅ Container is ephemeral

## Security Model

### API Authentication
- API key format: `sk_live_{64_hex_chars}`
- Stored in Lambda as bcrypt hash
- CLI stores plaintext in `~/.mycli/config.yaml` (0600 permissions)

### Git Credentials
- Stored in Lambda environment variables (encrypted at rest)
- Passed to container as part of shell script
- Written to files with restricted permissions (0600)
- Deleted before container exits

### Network Isolation
- VPC with public subnets (for Git access)
- Security group: egress-only (no ingress)
- Each task runs in isolated container

### IAM Roles
- **Lambda Execution Role:** Start ECS tasks, write logs
- **Task Execution Role:** Pull images, write logs
- **Task Role:** Minimal (logs only), users can add more permissions

## Shell Command Construction

**Current Implementation:** Bash script (pragmatic MVP solution)

**Advantages:**
- ✅ Simple and works everywhere
- ✅ No compilation needed
- ✅ Easy to understand and debug
- ✅ Works with any base image that has `/bin/sh`

**Future Considerations:**

The bash approach is fine for MVP, but we may want to consider more robust solutions:

1. **Go binary approach:**
   - Compile small Go binary
   - Inject into container as command
   - Better error handling and logging
   - More portable

2. **Python script approach:**
   - Self-contained Python script
   - Better for complex logic
   - Most images have Python

3. **Hybrid approach:**
   - Keep bash for simple cases
   - Switch to Go/Python for complex scenarios

**Note:** For now, bash is acceptable and gets the job done. We'll revisit if we encounter issues.

## Cost Optimization

### Compute
- **FARGATE_SPOT:** Default capacity provider (70% cost savings)
- **ARM64:** Cheaper than x86_64 (20% savings)
- **Small tasks:** 0.25 vCPU, 0.5 GB memory (configurable)

### Network
- **Public subnets:** No NAT Gateway cost
- **Shallow clones:** `--depth 1` minimizes transfer

### Storage
- **No S3:** Zero storage costs
- **Short log retention:** 7 days default

### API
- **API Gateway:** Pay per request
- **Lambda:** Pay per invocation (milliseconds)

**Example cost** (approximate):
- 100 executions/month @ 5 min each
- FARGATE_SPOT: ~$2/month
- Lambda: ~$0.20/month
- API Gateway: ~$0.03/month
- **Total: ~$2.23/month**

## Limitations & Trade-offs

### Current Limitations
1. **Bash dependency** - Requires `/bin/sh` in container
2. **Git installation** - Some images need git installed at runtime
3. **Single command** - No multi-step workflows (can use scripts)
4. **No artifacts** - Output not persisted (only in logs)

### Design Trade-offs
| Decision | Pro | Con |
|----------|-----|-----|
| No custom image | Simple, flexible | May need to install git |
| Shell command construction | Works everywhere | Less robust than Go |
| Public subnets | Cheaper (no NAT) | Tasks have public IPs |
| No S3 | Simpler, cheaper | Can't store artifacts |
| Shallow clone | Faster | No git history |

### Not Supported (Yet)
- ❌ Artifact storage (use S3 from your commands)
- ❌ Multi-step workflows (use a script in your repo)
- ❌ Scheduled executions (use EventBridge)
- ❌ Execution history (use CloudWatch queries)
- ❌ Private Git servers (GitHub/GitLab/SSH only)

## Scalability

### Current Limits
- **Lambda:** 1000 concurrent executions (AWS default)
- **ECS:** 10,000 tasks per cluster (soft limit)
- **API Gateway:** 10,000 req/sec (default)

### Bottlenecks
1. **ECS task quotas** - Request limit increase if needed
2. **Subnet IPs** - Use larger CIDR blocks for more tasks
3. **CloudWatch Logs** - No practical limit

### Scaling Strategy
1. Start with defaults (good for 100s of executions)
2. Monitor ECS cluster utilization
3. Request AWS quota increases as needed
4. Consider multiple clusters for isolation

## Monitoring

### CloudWatch Logs
- Log group: `/aws/mycli/<stack-name>`
- Log streams: `task/<execution-id>`
- Retention: 7 days (configurable)
- Query with CloudWatch Insights

### Metrics (future)
- Execution count
- Success/failure rate
- Execution duration
- Cost per execution

## Future Architecture Improvements

### Near-term
1. **Replace bash with Go binary** for robustness
2. **DynamoDB execution history** for queryability
3. **S3 artifact storage** for command outputs
4. **Execution templates** for common workflows

### Long-term
1. **Web dashboard** for monitoring
2. **Multi-step workflows** with dependencies
3. **Scheduled executions** with cron syntax
4. **Team management** and access control
5. **SaaS offering** with hosted infrastructure

## Comparison to Alternatives

| Feature | mycli | GitHub Actions | AWS CodeBuild |
|---------|-------|----------------|---------------|
| Setup | Quick (mycli init) | In repo | Console config |
| Cost | $2-5/month | Free (2000 min) | $0.005/min |
| Flexibility | Any image | Limited runners | Any image |
| Git integration | Built-in | Built-in | Built-in |
| Self-hosted | Yes | Optional | No |
| Trigger | CLI | Push/PR/Manual | Event-driven |

**mycli sweet spot:** Ad-hoc command execution from CLI with flexible image support and self-hosted infrastructure.
