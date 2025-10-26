# Implementation Notes

## Architecture Decision: No Custom Docker Image

### Decision
**Do not maintain a custom Docker image.** Instead, use standard Docker images and construct shell commands dynamically.

### Rationale

**Problems with custom image approach:**
- Maintenance burden (updating tools, versions, security patches)
- Build and publishing pipeline needed
- Users locked into our tool choices
- Large image size (800MB+ with all tools)
- Slower task starts (image pull time)

**Benefits of generic image approach:**
- ✅ Use any existing Docker image (terraform, python, node, etc.)
- ✅ No maintenance burden
- ✅ Users choose their own tools and versions
- ✅ Smaller images = faster starts
- ✅ Flexibility to change images per execution

### How It Works

**Lambda constructs a shell script** that:
1. Installs git if not present (apt-get, apk, or yum)
2. Configures git credentials (GitHub/GitLab token or SSH key)
3. Clones the repository to `/workspace/repo`
4. Changes to that directory
5. Executes the user's command
6. Cleans up credentials
7. Exits with the command's exit code

**ECS Task Override:**
```go
Command: []string{"/bin/sh", "-c", constructedShellScript}
```

### Example Usage

```bash
# Use Terraform image
mycli exec --image=hashicorp/terraform:1.6 \
  --repo=https://github.com/user/infra \
  "terraform plan"

# Use Python image
mycli exec --image=python:3.11 \
  --repo=https://github.com/user/scripts \
  "python script.py"

# Use default Ubuntu (git installed at runtime)
mycli exec --repo=https://github.com/user/app "make build"
```

## Shell Command Construction

### Current Implementation
**Bash script** - Generated dynamically by Lambda in Go.

### Advantages
- ✅ Works with any image that has `/bin/sh` (universal)
- ✅ Simple to understand and debug
- ✅ No compilation or build step needed
- ✅ Easy to modify and test
- ✅ Logs are human-readable

### Code Location
See `lambda/orchestrator/main.go` - `buildShellCommand()` function

### Script Structure
```bash
#!/bin/sh
set -e  # Exit on error

# 1. Install git if not present
if ! command -v git; then
  # Try apt-get (Ubuntu/Debian)
  # Try apk (Alpine)
  # Try yum (Amazon Linux/RHEL)
fi

# 2. Configure git credentials
if [[ -n "$GITHUB_TOKEN" ]]; then
  git config --global credential.helper store
  echo "https://$GITHUB_TOKEN@github.com" > ~/.git-credentials
fi

# 3. Clone repository
git clone --depth 1 --branch "$BRANCH" "$REPO_URL" /workspace/repo

# 4. Change directory
cd /workspace/repo

# 5. Execute user command
$USER_COMMAND

# 6. Cleanup
rm -f ~/.git-credentials ~/.ssh/id_rsa
exit $?
```

### Limitations & Future Improvements

**Current Limitations:**
1. Requires `/bin/sh` in container (almost universal)
2. Git installation adds ~5-10 seconds for minimal images
3. Bash string escaping for complex commands
4. Limited error handling compared to compiled code

**Future Improvements (if needed):**

The bash approach is pragmatic and works well for MVP. However, we could consider:

#### Option 1: Compiled Go Binary
Inject a small Go binary into the container:
```go
// Compiled once, injected at runtime
func main() {
    installGit()
    configureCredentials()
    cloneRepo()
    executeCommand()
    cleanup()
}
```

**Pros:**
- Better error handling
- Type safety
- More portable
- Faster (no git installation step)

**Cons:**
- Need to compile for multiple architectures
- Larger payload
- More complex to maintain

#### Option 2: Python Script
Similar to bash but with better structure:
```python
#!/usr/bin/env python3
import subprocess, os, sys

def main():
    install_git()
    configure_credentials()
    clone_repo()
    execute_command()
    cleanup()
```

**Pros:**
- Better error handling than bash
- Most images have Python
- Easier to test

**Cons:**
- Not as universal as bash
- Slower than compiled binary

#### Option 3: Hybrid Approach
Use bash for simple cases, switch to Go/Python for complex scenarios.

### Recommendation
**Stick with bash for now.** It's simple, works everywhere, and easy to debug. We can always evolve later if we hit limitations.

The key insight: We don't need a robust solution yet - we need a working one. Bash delivers that.

## Configuration Management

### Three-Level Priority System

1. **Command-line flags** (highest priority)
2. **`.mycli.yaml`** (project config)
3. **Git auto-detection** (convenience)

### Example Scenarios

**Scenario 1: Simple project with .mycli.yaml**
```yaml
# .mycli.yaml
repo: https://github.com/company/infrastructure
branch: main
image: hashicorp/terraform:1.6
env:
  TF_VAR_region: us-east-1
```

```bash
cd my-terraform-project
mycli exec "terraform plan"
# Uses all settings from .mycli.yaml
```

**Scenario 2: Override branch for different environment**
```bash
mycli exec --branch=staging "terraform plan"
# Uses .mycli.yaml but overrides branch
```

**Scenario 3: No config, auto-detect git repo**
```bash
cd my-git-repo  # has remote origin
mycli exec "make deploy"
# Auto-detects: git remote get-url origin
# Uses current branch
```

**Scenario 4: Explicit everything**
```bash
mycli exec \
  --repo=https://github.com/user/app \
  --branch=main \
  --image=python:3.11 \
  --env FOO=bar \
  "python script.py"
# No config files needed
```

## Git Credential Management

### Supported Methods

1. **GitHub Personal Access Token** (recommended)
   - Scope: `repo` (full control)
   - Format: `ghp_xxxxxxxxxxxx`
   - Stored in Lambda env: `GITHUB_TOKEN`

2. **GitLab Personal Access Token**
   - Scope: `read_repository`, `write_repository`
   - Stored in Lambda env: `GITLAB_TOKEN`

3. **SSH Private Key** (advanced)
   - Base64-encoded for environment variable storage
   - Supports GitHub, GitLab, Bitbucket
   - Stored in Lambda env: `SSH_PRIVATE_KEY`

### Security Model

**Storage:**
- Lambda environment variables (encrypted at rest by AWS)
- Never stored in CloudFormation outputs
- Never logged to CloudWatch

**Transmission:**
- Included in shell script as inline variables
- Written to files with 0600 permissions
- Deleted before container exits

**Rotation:**
- Update Lambda environment variables
- No CLI changes needed

### Setup Flow

```bash
$ mycli init

Configure Git credentials? [y/N]: y

Choose authentication method:
  1) GitHub Personal Access Token (recommended)
  2) GitLab Personal Access Token
  3) SSH Private Key
  4) Skip

Selection: 1
Enter GitHub token: ghp_xxxxx
✓ GitHub token configured
```

## Cost Analysis

### Per-Execution Cost Breakdown

**Assumptions:**
- 5-minute execution
- 0.25 vCPU, 0.5 GB memory
- FARGATE_SPOT pricing (us-east-1)
- Shallow git clone (~10 MB transfer)

**Costs:**
```
Fargate SPOT:
  vCPU:    $0.01242124 per vCPU hour
  Memory:  $0.00136308 per GB hour
  Duration: 5 min = 0.0833 hours
  
  Compute: (0.25 × $0.01242124 + 0.5 × $0.00136308) × 0.0833
         = $0.000315 per execution

Lambda: $0.0000002 per invocation
API Gateway: $0.0000035 per request
Data Transfer: $0.00001 (10 MB × $0.09/GB)

Total per execution: ~$0.00033
```

**Monthly costs:**
- 100 executions: ~$0.03
- 1,000 executions: ~$0.33
- 10,000 executions: ~$3.30

**Fixed costs:**
- CloudWatch Logs: $0.50/GB stored
- API Gateway: $3.50 per million requests
- Lambda: $0.20 per million requests

### Cost Optimization Tips

1. **Use FARGATE_SPOT** (default) - 70% savings over on-demand
2. **Right-size tasks** - Don't over-provision CPU/memory
3. **Short log retention** - 7 days is usually enough
4. **Shallow clones** - `--depth 1` (default)
5. **ARM64** - 20% cheaper than x86_64 (Lambda only)

## Testing Strategy

### Unit Tests (Future)
- Test `buildShellCommand()` function
- Test config parsing and merging
- Test git auto-detection

### Integration Tests
1. **Public repository:**
   ```bash
   mycli exec --repo=https://github.com/hashicorp/terraform-guides "ls -la"
   ```

2. **Private repository (GitHub):**
   ```bash
   mycli exec --repo=https://github.com/company/private "cat README.md"
   ```

3. **Custom image:**
   ```bash
   mycli exec --image=python:3.11 "python --version"
   ```

4. **With .mycli.yaml:**
   ```bash
   cd project-with-config
   mycli exec "terraform plan"
   ```

5. **Git auto-detect:**
   ```bash
   cd git-repo-without-config
   mycli exec "make test"
   ```

### Error Scenarios to Test
- Invalid repository URL
- Non-existent branch
- Missing Git credentials (private repo)
- Command failure (exit code != 0)
- Network timeout
- Invalid Docker image

## Known Limitations

### What Works
- ✅ Public Git repositories (GitHub, GitLab, Bitbucket)
- ✅ Private repos with token/SSH authentication
- ✅ Any Docker image from Docker Hub or ECR
- ✅ Commands up to ~30 minutes
- ✅ Environment variable passing
- ✅ Exit code propagation

### What Doesn't Work (Yet)
- ❌ Repositories > 1 GB (shallow clone helps)
- ❌ Multi-step workflows (use scripts in your repo)
- ❌ Artifact storage (logs only)
- ❌ Real-time streaming logs (poll only)
- ❌ Scheduled executions
- ❌ Git submodules
- ❌ Working directory override (always repo root)

### Workarounds

**Need artifacts?**
Upload to S3 from your command:
```bash
mycli exec "terraform apply && aws s3 cp terraform.tfstate s3://bucket/"
```

**Need multi-step workflow?**
Create a script in your repo:
```bash
# In your repo: run.sh
terraform init
terraform plan -out=plan
terraform apply plan
```

```bash
mycli exec "./run.sh"
```

**Need scheduled executions?**
Use EventBridge + Lambda to call the API.

## Deployment Checklist

### Prerequisites
- [ ] AWS account with admin access
- [ ] AWS CLI configured
- [ ] Go 1.21+ installed
- [ ] Git installed

### Initial Setup
1. [ ] Clone repository
2. [ ] Build CLI: `go build -o mycli`
3. [ ] Run init: `./mycli init --region us-east-1`
4. [ ] Configure Git credentials (interactive prompt)
5. [ ] Save API key (shown once)

### First Execution Test
```bash
# Test with public repo
./mycli exec \
  --repo=https://github.com/hashicorp/terraform-guides \
  --image=hashicorp/terraform:1.6 \
  "terraform version"

# Check status
./mycli status <task-arn>

# View logs
./mycli logs <execution-id>
```

### Production Readiness
- [ ] Review IAM task role permissions (add as needed)
- [ ] Adjust VPC CIDR if scaling beyond 250 concurrent tasks
- [ ] Set up CloudWatch alarms for failures
- [ ] Configure log retention based on compliance needs
- [ ] Document Git credential rotation process
- [ ] Create .mycli.yaml for production projects

## Troubleshooting

### Common Issues

**Issue: "Failed to clone repository"**
- Check repository URL is correct
- Verify branch exists
- Ensure Git credentials are configured
- Test git clone locally first

**Issue: "Command not found"**
- Image may not have required tools
- Try: `--image=ubuntu:22.04` (has apt-get)
- Or use an image with tools pre-installed

**Issue: Task takes too long to start**
- Large Docker images take time to pull
- Use smaller base images
- Consider multi-stage builds for custom images

**Issue: "No logs available"**
- Logs take 5-10 seconds to appear
- Use `mycli logs -f <execution-id>` to follow
- Check ECS task started successfully

**Issue: API key authentication fails**
- Check ~/.mycli/config.yaml has correct key
- Verify Lambda environment has correct hash
- Re-run `mycli init` if needed

## Next Steps

### Short-term (1-2 weeks)
- [ ] Add more examples to documentation
- [ ] Create integration test suite
- [ ] Add `mycli destroy` command improvements
- [ ] Support for custom task role ARNs

### Medium-term (1-3 months)
- [ ] DynamoDB execution history
- [ ] Web dashboard for monitoring
- [ ] Support for S3 artifact storage
- [ ] Scheduled executions via EventBridge

### Long-term (3-6 months)
- [ ] Multi-step workflows
- [ ] Team management
- [ ] Cost tracking
- [ ] SaaS offering
