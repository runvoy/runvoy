# mycli Git-based Backend Implementation Summary

## Overview

Successfully implemented a **simple, flexible Git-based remote execution backend** for mycli that uses standard Docker images without custom containers.

## Key Architecture Decision

✅ **NO custom Docker image needed!**  
✅ **NO custom entrypoint script needed!**

Instead, we:
- Use **any standard Docker image** (terraform, python, node, etc.)
- **Dynamically construct shell commands** in Lambda
- **Override ECS task command** at runtime

This approach is:
- **Simpler** - No image to build or maintain
- **Flexible** - Users choose their own images and tools
- **Standard** - Uses official Docker Hub images

## How It Works

```
┌─────────────┐
│ mycli exec  │──► "terraform plan"
└──────┬──────┘
       │
       ▼
┌──────────────────────────────────────────┐
│ Lambda Orchestrator                      │
│  • Constructs shell script:              │
│    - Install git (if needed)             │
│    - Configure credentials               │
│    - Clone repo                          │
│    - cd /workspace/repo                  │
│    - terraform plan                      │
└──────┬───────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────┐
│ ECS Fargate Task                         │
│  • Image: hashicorp/terraform:1.6       │
│  • Command: ["/bin/sh", "-c", script]   │
│  • Executes git clone + user command    │
└──────────────────────────────────────────┘
```

## Implementation Details

### 1. Lambda Orchestrator (`lambda/orchestrator/main.go`)

**Key Function:** `buildShellCommand(repo, branch, userCommand, credentials...)`

Constructs a shell script that:
1. Installs git if not present (apt-get, apk, or yum)
2. Configures git credentials (GitHub/GitLab token or SSH key)
3. Clones repository: `git clone --depth 1 --branch <branch> <repo> /workspace/repo`
4. Changes directory: `cd /workspace/repo`
5. Executes user command
6. Cleans up credentials
7. Exits with command's exit code

**ECS Task Override:**
```go
containerOverride := ecsTypes.ContainerOverride{
    Name:    "executor",
    Image:   "hashicorp/terraform:1.6", // User-specified
    Command: []string{"/bin/sh", "-c", shellCommand},
    Environment: userEnvVars,
}
```

### 2. CloudFormation Template (`cmd/cloudformation.yaml`)

**Changes from custom image approach:**
- **Removed:** Custom image parameters
- **Added:** Generic `DefaultImage` parameter (ubuntu:22.04)
- **Task Definition:** Uses template image, overridden at runtime

**Key insight:** Task definition is just a template. Real image and command are set via task override.

### 3. CLI Implementation

#### Configuration Sources
1. **Command-line flags** (highest priority)
   ```bash
   mycli exec --repo=... --branch=... --image=... "command"
   ```

2. **`.mycli.yaml`** in current directory
   ```yaml
   repo: https://github.com/company/infrastructure
   branch: main
   image: hashicorp/terraform:1.6
   env:
     TF_VAR_region: us-east-1
   ```

3. **Git auto-detection** (convenience)
   ```bash
   cd my-git-repo
   mycli exec "make deploy"  # Auto-detects: git remote get-url origin
   ```

#### Updated Commands
- **`cmd/exec.go`** - Project config parser, git auto-detect, config merging
- **`cmd/init.go`** - Interactive Git credential setup
- **`internal/project/config.go`** - .mycli.yaml parser
- **`internal/git/detector.go`** - Git remote URL detection

### 4. Example Usage

**With .mycli.yaml (simplest):**
```bash
cd my-terraform-project  # has .mycli.yaml
mycli exec "terraform plan"
```

**With CLI flags (explicit):**
```bash
mycli exec \
  --repo=https://github.com/user/infra \
  --branch=main \
  --image=hashicorp/terraform:1.6 \
  "terraform apply"
```

**With git auto-detect:**
```bash
cd my-git-repo  # no .mycli.yaml, but has remote
mycli exec "make deploy"
```

**Different images for different tasks:**
```bash
# Terraform
mycli exec --image=hashicorp/terraform:1.6 "terraform plan"

# Python
mycli exec --image=python:3.11 "python script.py"

# Node.js
mycli exec --image=node:18 "npm test"

# Generic Ubuntu
mycli exec "apt-get update && apt-get install -y jq && ./script.sh"
```

## Benefits of This Approach

### Simplicity
- ✅ No custom Docker image to build, maintain, or publish
- ✅ No entrypoint script in containers
- ✅ Uses standard, official Docker images
- ✅ Shell command construction in Lambda (easy to modify)

### Flexibility  
- ✅ Use **any** Docker image (public or private)
- ✅ Different image per execution
- ✅ Users control their own tool versions
- ✅ No lock-in to our tool choices

### Performance
- ✅ Smaller images = faster task starts
- ✅ Images often already cached in AWS
- ✅ No custom image pull from ECR

### Maintenance
- ✅ Zero maintenance for container image
- ✅ Users update their own images
- ✅ No security patching burden
- ✅ No build/publish pipeline needed

## Shell Command Construction

**Current Implementation:** Bash script (pragmatic, works everywhere)

**Why bash is OK for now:**
- Works with any image that has `/bin/sh` (universal)
- Simple to understand and debug
- Easy to modify in Lambda code
- No compilation needed
- Human-readable logs

**Future considerations:** Could move to Go binary or Python script if we need more robust error handling, but bash is perfectly acceptable for MVP and likely beyond.

See `IMPLEMENTATION_NOTES.md` for detailed discussion of future improvements.

## Files Added/Modified

### Added (3 files)
- `internal/project/config.go` - .mycli.yaml parser
- `internal/git/detector.go` - Git remote auto-detection  
- `ARCHITECTURE.md` - Complete architecture documentation
- `IMPLEMENTATION_NOTES.md` - Implementation details and decisions

### Modified (6 files)
- `lambda/orchestrator/main.go` - Shell command construction
- `cmd/cloudformation.yaml` - Generic base image, command override
- `cmd/init.go` - Git credential setup
- `cmd/exec.go` - Project config and auto-detect support
- `internal/api/client.go` - Git parameters in requests
- `internal/config/config.go` - Removed code_bucket field

### Removed (4 files)
- ~~`docker/Dockerfile`~~ - Not needed
- ~~`docker/entrypoint.sh`~~ - Not needed
- ~~`docker/Makefile`~~ - Not needed
- ~~`docker/README.md`~~ - Not needed

## Testing

### Unit Tests (Future)
- Test `buildShellCommand()` with various inputs
- Test config parsing and merging
- Test git auto-detection

### Integration Tests
```bash
# Public repo
mycli exec --repo=https://github.com/hashicorp/terraform-guides "ls -la"

# Private repo with GitHub token
mycli exec --repo=https://github.com/company/private "cat README.md"

# Custom image
mycli exec --image=python:3.11 "python --version"

# With .mycli.yaml
cd project-with-config && mycli exec "terraform plan"

# Git auto-detect
cd git-repo-without-config && mycli exec "make test"
```

## Deployment

### Prerequisites
- AWS account with admin access
- AWS CLI configured
- Go 1.21+ installed

### Steps

1. **Build CLI:**
   ```bash
   go build -o mycli
   ```

2. **Deploy infrastructure:**
   ```bash
   ./mycli init --region us-east-1
   # Follow prompts to configure Git credentials (optional)
   ```

3. **Test execution:**
   ```bash
   ./mycli exec \
     --repo=https://github.com/hashicorp/terraform-guides \
     --image=hashicorp/terraform:1.6 \
     "terraform version"
   ```

4. **Check status and logs:**
   ```bash
   ./mycli status <task-arn>
   ./mycli logs <execution-id>
   ./mycli logs -f <execution-id>  # Follow logs
   ```

## Cost Efficiency

**Per-execution cost:** ~$0.0003 (5-minute execution)

**Monthly estimates:**
- 100 executions: ~$0.03
- 1,000 executions: ~$0.33
- 10,000 executions: ~$3.30

**Cost optimizations:**
- FARGATE_SPOT (default) - 70% savings
- No S3 storage costs
- Shallow git clones (`--depth 1`)
- Small task sizes (0.25 vCPU, 0.5 GB)

## Limitations

### What Works
- ✅ Any Docker image with `/bin/sh`
- ✅ Public and private Git repositories
- ✅ GitHub, GitLab, and SSH authentication
- ✅ Commands up to 30 minutes (configurable)
- ✅ Environment variable passing

### What Doesn't Work (Yet)
- ❌ Multi-step workflows (use scripts in your repo)
- ❌ Artifact storage (use S3 from your commands)
- ❌ Scheduled executions (use EventBridge)
- ❌ Real-time log streaming (poll-based only)

### Workarounds Available
See `IMPLEMENTATION_NOTES.md` for detailed workarounds.

## Security

### Git Credentials
- Stored in Lambda environment variables (encrypted at rest)
- Passed to container via shell script
- Cleaned up before container exits
- Never logged to CloudWatch

### API Authentication
- Bcrypt-hashed API key in Lambda
- Plaintext key in `~/.mycli/config.yaml` (0600 permissions)

### Network Isolation
- VPC with public subnets (for Git access)
- Security group: egress-only
- Each task in isolated container

## Future Enhancements

### Near-term
- [ ] Integration test suite
- [ ] More image examples in docs
- [ ] Support for custom task role ARNs
- [ ] Improved error messages

### Medium-term
- [ ] DynamoDB execution history
- [ ] S3 artifact storage support
- [ ] Web dashboard for monitoring
- [ ] Scheduled executions

### Long-term
- [ ] Multi-step workflows
- [ ] Team management
- [ ] Cost tracking
- [ ] SaaS offering

### Potential Shell Improvements
Currently using bash (works well), but could consider:
- **Go binary approach** - Better error handling, type safety
- **Python script approach** - Better structure, most images have Python
- **Hybrid approach** - Bash for simple, Go for complex

**Current status:** Bash is perfectly adequate. No urgent need to change.

## Documentation

- **`ARCHITECTURE.md`** - Complete system architecture
- **`IMPLEMENTATION_NOTES.md`** - Detailed implementation notes and decisions
- **`.mycli.yaml.example`** - Example project configuration
- **`CONTEXT.md`** - Original requirements and design

## Comparison to Alternatives

| Feature | mycli | GitHub Actions | AWS CodeBuild |
|---------|-------|----------------|---------------|
| Setup | `mycli init` | YAML in repo | Console config |
| Trigger | CLI command | Push/PR/Manual | Event-driven |
| Images | Any Docker image | GitHub runners | Any Docker image |
| Cost | $2-5/month | Free (2000 min) | $0.005/min |
| Control | Full (self-hosted) | Limited | AWS-managed |

**mycli sweet spot:** Ad-hoc CLI execution with flexible image support and self-hosted infrastructure.

## Conclusion

The implementation is **simpler and more flexible** than the original custom image approach:
- No Docker image to maintain
- Users choose their own tools
- Standard Docker images from Docker Hub
- Shell command construction in Lambda
- Works everywhere

**Status:** ✅ Ready for deployment and testing

See `ARCHITECTURE.md` and `IMPLEMENTATION_NOTES.md` for complete details.
