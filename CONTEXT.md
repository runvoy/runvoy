# mycli - Remote Command Execution

## Architecture
- AWS: API Gateway + Lambda + Fargate + S3 + DynamoDB
- API key auth (bcrypt hashed)
- CloudWatch Logs for output
- General purpose command execution (not just Terraform)

## What's built:
- CLI skeleton with commands: init, configure, exec, status, logs
- init: deploys CloudFormation, generates API key, saves config
- Config management in ~/.mycli/config.yaml

## Next: Design CloudFormation template
Resources needed:
- S3 bucket (code storage)
- DynamoDB table (API keys)
- API Gateway (REST API with IAM auth)
- Lambda (orchestrator)
- ECS Fargate (execution environment)
- CloudWatch Log Group
- IAM roles