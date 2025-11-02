# Security Policy

## Supported Versions

Currently, runvoy is in active development. We recommend using the latest version from the `main` branch.

| Version | Supported          |
| ------- | ------------------ |
| main    | :white_check_mark: |
| 0.1.x   | :white_check_mark: |
| < 0.1   | :x:                |

## Security Features

runvoy implements several security best practices:

- **API Key Hashing**: All API keys are hashed using SHA-256 before storage
- **No Credential Sharing**: Team members never see AWS credentials
- **Complete Audit Trail**: Every execution is logged with user identification
- **One-Time Claim Tokens**: API keys are distributed via secure, one-time-use tokens
- **IAM Scoping**: Lambda functions have minimal, scoped permissions
- **Automated Security Scanning**: Dependabot and govulncheck run weekly
- **Pre-commit Security Hooks**: Detect secrets and vulnerabilities before commit

## Reporting a Vulnerability

We take security issues seriously. If you discover a security vulnerability, please follow these steps:

### DO NOT

- Open a public GitHub issue
- Disclose the vulnerability publicly before it's been addressed
- Exploit the vulnerability beyond the minimum necessary to demonstrate it

### DO

1. **Email the maintainers** at: [Add security contact email here]
2. **Include the following information**:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)
   - Your name/handle for acknowledgment (optional)

### What to Expect

- **Acknowledgment**: Within 48 hours
- **Initial Assessment**: Within 1 week
- **Status Updates**: Every week until resolved
- **Fix Timeline**: Critical issues within 7 days, others within 30 days
- **Disclosure**: Coordinated disclosure after fix is released

## Security Best Practices for Users

### For Administrators

1. **Protect API Keys**
   - Never commit API keys to version control
   - Store keys in `~/.runvoy/config.yaml` with restricted permissions (600)
   - Rotate keys regularly using `runvoy users revoke` and `runvoy users create`
   - Use separate keys for different environments (dev/staging/prod)

2. **Infrastructure Security**
   - Deploy runvoy in a dedicated AWS account or use IAM boundaries
   - Enable CloudTrail for audit logging
   - Review Lambda function permissions regularly
   - Use VPC endpoints for private communication (if needed)

3. **User Management**
   - Revoke API keys for departing team members immediately
   - Use claim tokens with short expiration (15 minutes default)
   - Share claim tokens via secure channels (not email/slack)
   - Monitor user activity via CloudWatch Logs

### For Team Members

1. **Protect Your API Key**
   - Never share your API key with others
   - Don't commit `~/.runvoy/config.yaml` to version control
   - Report lost/compromised keys to your admin immediately
   - Use different keys for personal testing vs team usage

2. **Secure Local Configuration**
   ```bash
   # Ensure config file has restrictive permissions
   chmod 600 ~/.runvoy/config.yaml
   ```

3. **Be Cautious with Commands**
   - Review commands before execution
   - Don't run untrusted scripts
   - Be aware of what permissions the task role has
   - Use locks to prevent concurrent operations on shared state

## Known Limitations

### Current Limitations (To Be Addressed)

1. **No Rate Limiting** (Planned for v0.2.0)
   - API endpoints currently have no rate limits
   - Could be vulnerable to DoS attacks
   - Mitigation: Use AWS WAF in front of Lambda Function URL

2. **No Request Size Limits** (Planned for v0.2.0)
   - Large requests could exhaust Lambda memory
   - Mitigation: Lambda has built-in limits (6MB payload)

3. **No Multi-Factor Authentication** (Planned for v1.0.0)
   - Only API key authentication supported
   - Consider implementing for admin operations

4. **Hardcoded Webviewer URL**
   - Currently points to public S3 bucket
   - Could be changed to self-hosted in future

### Not in Scope

1. **Secrets Management**
   - runvoy does not manage secrets for your commands
   - Use AWS Secrets Manager or Parameter Store in your task definitions
   - Pass secrets as environment variables via `--env` flag

2. **Network Isolation**
   - Tasks run in default VPC by default
   - Configure VPC in task definitions for network isolation

## Security Updates

We use Dependabot and govulncheck to automatically detect and update vulnerable dependencies:

- **Dependabot**: Runs weekly, creates PRs for outdated dependencies
- **govulncheck**: Runs in pre-commit hooks and CI/CD
- **Trivy**: Scans for vulnerabilities in CI/CD pipeline

Subscribe to GitHub repository notifications to receive security update alerts.

## Third-Party Security Audits

No formal security audits have been conducted yet. If you're interested in sponsoring an audit, please contact the maintainers.

## Security-Related Configuration

### Recommended CloudFormation Parameters

```yaml
# Enable deletion protection on critical resources
DeletionPolicy: Retain

# Use encryption at rest
DynamoDB:
  SSESpecification:
    SSEEnabled: true

# Enable VPC for Lambda functions (optional)
Lambda:
  VpcConfig:
    SubnetIds: [subnet-xxx]
    SecurityGroupIds: [sg-xxx]
```

### IAM Best Practices

The CloudFormation template includes minimal IAM permissions. Review and adjust based on your security requirements:

```yaml
# Orchestrator Lambda has permissions for:
- ECS task execution (scoped to runvoy cluster)
- DynamoDB read/write (scoped to runvoy tables)
- CloudWatch Logs write
- Task definition operations (scoped to runvoy-image-* family)

# Event Processor Lambda has permissions for:
- DynamoDB write (scoped to executions table)
- CloudWatch Logs write
```

## Compliance

### SOC 2 / ISO 27001

runvoy provides features that support compliance:

- **Audit Trail**: All executions logged with user identification
- **Access Control**: API key-based authentication
- **Encryption**: Data encrypted at rest (DynamoDB SSE) and in transit (HTTPS)
- **Monitoring**: CloudWatch Logs for all components

However, runvoy itself is not certified. Consult your compliance officer.

### GDPR

runvoy stores user email addresses in DynamoDB:

- **Data Controller**: You (the AWS account owner)
- **Data Retention**: No automatic deletion (you control lifecycle)
- **Right to Erasure**: Use `runvoy users revoke` and manual DynamoDB deletion

## Security Checklist for Deployment

Before deploying runvoy to production:

- [ ] Review IAM permissions in CloudFormation template
- [ ] Enable CloudTrail in your AWS account
- [ ] Configure API Gateway rate limiting (if using)
- [ ] Restrict Lambda Function URL to specific IPs (if possible)
- [ ] Enable DynamoDB point-in-time recovery
- [ ] Set up CloudWatch alarms for unusual activity
- [ ] Document incident response procedures
- [ ] Review and customize task execution role permissions
- [ ] Enable VPC for sensitive workloads (optional)
- [ ] Configure secrets management for task environment variables

## Attribution

We appreciate the security researchers who help keep runvoy secure. Security contributors will be acknowledged (with permission) in release notes.

---

**Last Updated**: November 2, 2025  
**Version**: 1.0
