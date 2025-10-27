# Development Scripts

This directory contains helper scripts for developing mycli.

## update-lambda.sh

Quick Lambda function update helper for development. Updates the Lambda orchestrator code without requiring a full `destroy`/`init` cycle.

## update-cloudformation.sh

Quick CloudFormation stack update helper for development. Updates the CloudFormation infrastructure (IAM roles, ECS task definitions, etc.) without requiring a full `destroy`/`init` cycle.

### Usage

```bash
# Update with default stack name (mycli)
./scripts/update-cloudformation.sh

# Update with custom stack name
./scripts/update-cloudformation.sh my-custom-stack
```

### Prerequisites

- AWS CLI installed and configured
- mycli already initialized (`mycli init` completed)
- Appropriate AWS permissions to update CloudFormation stacks

### What it does

1. Reads current stack parameters from the existing stack
2. Updates the CloudFormation stack with the latest template
3. Waits for the update to complete
4. Reports success or any issues

### When to use

Use this script when you've modified:
- `deploy/cloudformation.yaml` - IAM permissions, resource definitions, etc.
- When you need to add/remove permissions for Lambda or task roles
- When updating CloudFormation infrastructure parameters

### Notes

- This is a **development helper** - use proper CI/CD for production
- The script reads region from `~/.mycli/config.yaml`
- Stack name defaults to `mycli` but can be overridden
- Preserves existing stack parameters (API keys, Git credentials, etc.)
- Much faster than full `mycli destroy && mycli init` cycle

### Troubleshooting

**Error: Config file not found**
- Run `mycli init` first to create the config file

**Error: Stack not found**
- Verify the stack name matches your CloudFormation stack
- Check that the stack exists: `aws cloudformation describe-stacks --stack-name mycli --region <region>`

**Error: Access denied**
- Ensure your AWS credentials have `cloudformation:UpdateStack` and related permissions

**Error: No updates to be performed**
- This is normal if the CloudFormation template hasn't changed
- The stack is already up to date

## update-lambda.sh

Quick Lambda function update helper for development. Updates the Lambda orchestrator code without requiring a full `destroy`/`init` cycle.

### Usage

```bash
# Update with default stack name (mycli)
./scripts/update-lambda.sh

# Update with custom stack name
./scripts/update-lambda.sh my-custom-stack
```

### Prerequisites

- AWS CLI installed and configured
- mycli already initialized (`mycli init` completed)
- Appropriate AWS permissions to update Lambda functions

### What it does

1. Builds the Lambda function from `backend/orchestrator/`
2. Creates a deployment zip package
3. Updates the Lambda function code in AWS
4. Waits for the update to complete
5. Cleans up temporary files

### Notes

- This is a **development helper** - use proper CI/CD for production
- The script reads region from `~/.mycli/config.yaml`
- Stack name defaults to `mycli` but can be overridden
- Much faster than full `mycli destroy && mycli init` cycle

### Troubleshooting

**Error: Config file not found**
- Run `mycli init` first to create the config file

**Error: Function not found**
- Verify the stack name matches your CloudFormation stack
- Check that the Lambda function exists: `aws lambda get-function --function-name mycli-orchestrator --region <region>`

**Error: Access denied**
- Ensure your AWS credentials have `lambda:UpdateFunctionCode` permission
