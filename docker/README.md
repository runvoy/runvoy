# mycli Executor Docker Image

This Docker image provides a complete execution environment for running commands with Git repository support.

## Included Tools

- **Version Control:** Git, SSH client
- **Cloud:** AWS CLI v2
- **Infrastructure as Code:** Terraform, Ansible
- **Kubernetes:** kubectl, Helm
- **Languages:** Python 3, pip
- **Utilities:** curl, wget, jq, unzip

## Building the Image

```bash
# Build locally
docker build -t mycli/executor:latest .

# Build for ARM64 (for AWS Graviton/Fargate)
docker buildx build --platform linux/arm64 -t mycli/executor:latest .

# Build for multi-arch
docker buildx build --platform linux/amd64,linux/arm64 -t mycli/executor:latest .
```

## Pushing to AWS ECR Public

```bash
# Login to ECR Public
aws ecr-public get-login-password --region us-east-1 | docker login --username AWS --password-stdin public.ecr.aws

# Create repository (first time only)
aws ecr-public create-repository --repository-name mycli/executor --region us-east-1

# Tag and push
docker tag mycli/executor:latest public.ecr.aws/<your-alias>/mycli-executor:latest
docker push public.ecr.aws/<your-alias>/mycli-executor:latest
```

## Environment Variables

The entrypoint script expects these environment variables:

### Required
- `REPO_URL`: Git repository URL (HTTPS or SSH)
- `USER_COMMAND`: Command to execute in the repository

### Optional
- `REPO_BRANCH`: Git branch to checkout (default: `main`)
- `EXECUTION_ID`: Unique execution identifier (for logging)

### Git Authentication (at least one should be provided for private repos)
- `GITHUB_TOKEN`: GitHub Personal Access Token
- `GITLAB_TOKEN`: GitLab Personal Access Token
- `SSH_PRIVATE_KEY`: Base64-encoded SSH private key

### User Variables
- Any additional environment variables will be available to the executed command

## Testing Locally

```bash
# Test with a public repository
docker run --rm \
  -e REPO_URL=https://github.com/hashicorp/terraform-guides \
  -e REPO_BRANCH=main \
  -e USER_COMMAND="ls -la && cat README.md" \
  mycli/executor:latest

# Test with private repository (GitHub)
docker run --rm \
  -e REPO_URL=https://github.com/your-org/your-private-repo \
  -e REPO_BRANCH=main \
  -e USER_COMMAND="terraform init && terraform plan" \
  -e GITHUB_TOKEN=ghp_your_token_here \
  -e AWS_ACCESS_KEY_ID=$AWS_ACCESS_KEY_ID \
  -e AWS_SECRET_ACCESS_KEY=$AWS_SECRET_ACCESS_KEY \
  -e AWS_DEFAULT_REGION=us-east-1 \
  mycli/executor:latest
```

## Image Customization

To customize the image for your needs:

1. Modify the `Dockerfile` to add/remove tools
2. Update tool versions by changing the ARG values
3. Rebuild and push to your own registry

Example: Adding a new tool

```dockerfile
# Install Pulumi
RUN curl -fsSL https://get.pulumi.com | sh
ENV PATH="/root/.pulumi/bin:${PATH}"
```

## Security Notes

1. **Credentials**: Git credentials are automatically cleaned up after execution
2. **Isolation**: Each execution runs in a fresh container
3. **Minimal Persistence**: No data is persisted between executions
4. **SSH Keys**: SSH private keys are base64-encoded for safe environment variable transmission
