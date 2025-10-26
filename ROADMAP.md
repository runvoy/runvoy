# mycli Roadmap

## Current Status: MVP Complete ✅

Working features:
- ✅ Infrastructure deployment (`mycli init`)
- ✅ Remote execution with Git cloning
- ✅ Direct execution without Git (`--skip-git`)
- ✅ Status checking
- ✅ Log viewing (with task ARN)
- ✅ Git auto-detection
- ✅ Project configuration (`.mycli.yaml`)

## Known Issues

- [x] exec ignores Docker image, always uses ubuntu:22.04 (now configurable)
- [ ] Logs require full task ARN (verbose to copy/paste)
- [ ] Log stream name format includes container name (implementation detail exposed)
- [ ] No execution history tracking

## Near-term (Next 1-2 months)

### High Priority
- [ ] **DynamoDB execution history table**
  - Store: execution ID → task ARN, repo, command, status, timestamps
  - Enable: `mycli list` to show recent executions
  - Enable: `mycli logs <execution-id>` with short IDs
  - Benefit: Better UX, queryable history

- [ ] **Improve error messages**
  - Better git clone failures (auth, branch not found, etc.)
  - Clear feedback when task fails to start
  - Validate repo URL format before execution

- [ ] **Add execution timeout enforcement**
  - Currently timeout parameter is accepted but not used
  - Implement task timeout with graceful termination
  - Add to CloudWatch logs when timeout occurs

### Medium Priority
- [ ] **Multiple API keys support**
  - Move from Lambda env var to DynamoDB table
  - Support key rotation
  - Add key permissions (read-only vs execute)

- [ ] **Better log formatting**
  - Strip ANSI color codes option
  - Filter by log level
  - Search within logs

- [ ] **Execution aliases/shortcuts**
  - Save common commands: `mycli alias tf-plan "terraform plan"`
  - Run with: `mycli run tf-plan`

### Nice to Have
- [ ] **Web dashboard** (simple CloudFront + S3 static site)
  - View execution history
  - Real-time log streaming via WebSocket
  - Cost tracking graphs

- [ ] **Scheduled executions**
  - EventBridge integration
  - Cron syntax: `mycli schedule "0 0 * * *" "terraform plan"`

## Mid-term (3-6 months)

- [ ] **Custom Docker image per execution**
  - Dynamic task definition registration
  - Cache task definitions to avoid duplicates
  - Support private ECR images

- [ ] **S3 artifact storage**
  - Optional S3 bucket for command outputs
  - `mycli artifacts <execution-id>` to download
  - Auto-upload from `/workspace/artifacts`

- [ ] **Multi-step workflows**
  - YAML-based workflow definitions
  - Dependencies between steps
  - Conditional execution

- [ ] **Enhanced Git support**
  - Submodules support
  - Sparse checkout for large repos
  - Git LFS support

## Long-term (6-12 months)

- [ ] **Team features**
  - User accounts and authentication
  - Team workspaces
  - Shared execution history
  - Role-based access control

- [ ] **SaaS offering**
  - Hosted infrastructure option
  - User signup and billing
  - Usage metering
  - Multi-tenancy

- [ ] **Multi-cloud support**
  - Google Cloud Run backend
  - Azure Container Instances
  - Unified CLI for all clouds

## Ideas / Backlog

- [ ] Local execution mode (for development/testing)
- [ ] Integration with CI/CD platforms (GitHub Actions, GitLab CI)
- [ ] Notification webhooks (Slack, Discord, email)
- [ ] Cost optimization recommendations
- [ ] Execution templates/presets
- [ ] SSH into running container (for debugging)
- [ ] Execution replay/restart
- [ ] Export execution logs to external systems

## Completed

- ✅ Basic infrastructure deployment
- ✅ Git repository cloning
- ✅ Command execution in containers
- ✅ CloudWatch logging
- ✅ Configuration management
- ✅ Git auto-detection
- ✅ Direct execution mode (--skip-git)
- ✅ IAM permission fixes (ecs:TagResource)
- ✅ Documentation consolidation

---

## Contributing

Want to work on something? Check the roadmap above and:
1. Comment on an existing issue or create one
2. Reference this roadmap in your PR
3. Update this file when completing items

## Prioritization Criteria

**High Priority:**
- Fixes critical bugs
- Improves core UX significantly
- Unblocks other features

**Medium Priority:**
- Nice-to-have improvements
- Performance optimizations
- Developer experience

**Low Priority:**
- Edge cases
- Advanced features for power users
- Nice-to-have additions

---

Last Updated: 2025-10-26
