# Development Scripts

This directory contains helper scripts for developing mycli.

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

1. Builds the Lambda function from `lambda/orchestrator/`
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
