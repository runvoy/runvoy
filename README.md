# mycli

Remote execution environment for your commands. Run commands in isolated, repeatable environments without local execution hassles.

## Features

- 🚀 One-command setup
- 🔒 Secure isolated execution
- ☁️ Runs in your AWS account (self-hosted)
- 🎯 No local dependency hell
- 📝 Full audit trail via CloudWatch

## Quick Start

```bash
# 1. Initialize infrastructure in your AWS account
mycli init

# 2. Execute commands remotely
mycli exec "terraform apply"
mycli exec "ansible-playbook deploy.yml"
mycli exec "./my-script.sh"

# 3. Check status and logs
mycli status exec_abc123
mycli logs exec_abc123
```

## Installation

```bash
go install mycli@latest
```

## Architecture

- **CLI**: Go application (this repo)
- **Compute**: AWS Fargate (serverless containers)
- **Storage**: S3 for code, CloudWatch for logs
- **Auth**: API keys with bcrypt hashing
- **IaC**: CloudFormation for deployment

## TODO / Future Enhancements

### Execution History & Metadata
- Add `executions` DynamoDB table to store:
  - Execution history (list all past runs)
  - Custom tags/labels for executions
  - User/project association
  - Cost tracking
  - Execution duration statistics
- Benefits: queryable history, audit trail, cost analysis
- Current: CLI queries ECS/CloudWatch directly (stateless)

### API Keys Management
- Move API key storage to DynamoDB table (currently stored as Lambda environment variable)
  - Benefits: support for multiple keys, key rotation, audit trail
  - Current: single key hashed with bcrypt in Lambda env var (simpler, no DB needed)
- Multiple keys per user
- Key rotation
- Scoped permissions (read-only vs execute)
- Rate limiting per key

### Enhanced Logging
- S3 archival of CloudWatch logs for long-term retention
- Structured logging with JSON output
- Log search/filtering

## License

MIT
