# Embedded Assets

This directory contains CloudFormation templates and Lambda source code that are embedded into the CLI binary at build time using Go's `embed` package.

## Directory Structure

The assets are organized by cloud provider to support future multi-cloud implementations:

```
internal/assets/
├── aws/                  # AWS CloudFormation templates
│   ├── cloudformation-backend.yaml
│   └── cloudformation-lambda-bucket.yaml
├── lambda/               # Lambda function source code
│   └── orchestrator/
│       └── main.go       # Lambda handler code
├── templates.go          # Go functions to access embedded assets
└── README.md             # This file
```

## Why Embed Assets?

Embedding assets provides several benefits:

1. **Self-contained binary**: The CLI is distributable as a single binary without external dependencies
2. **Version consistency**: Assets are guaranteed to match the CLI version they're built with
3. **No file system dependencies**: No need to ship files separately or worry about file paths
4. **Reliability**: Assets can't be accidentally modified or deleted after installation

## Modifying Assets

### CloudFormation Templates
To update CloudFormation templates:

1. Edit the template file directly in `internal/assets/aws/cloudformation-*.yaml`
2. Rebuild the CLI binary: `go build`

### Lambda Code
To update Lambda function code:

1. Edit the Lambda source in `internal/assets/lambda/orchestrator/*.go`
2. Rebuild the CLI binary: `go build`

All assets are embedded at compile time, so changes require a rebuild.

## How It Works

1. `templates.go` uses `//go:embed` directives to embed assets:
   - `aws/*.yaml` - CloudFormation templates
   - `lambda/**/*.go` - Lambda source code
2. At build time, Go reads the files and embeds them into the binary
3. Helper functions provide access to embedded content
4. Lambda code is extracted to a temp directory and built on-demand during `init`
5. No file system access required at runtime (except for temporary Lambda builds)

## Verification

To verify templates are embedded, build and check the binary:
```bash
go build -o runvoy
strings runvoy | grep "AWSTemplateFormatVersion"
```

You should see the CloudFormation templates embedded in the binary output.

## Future Multi-Cloud Support

The `aws/` directory organization allows for future cloud provider support:
- `aws/` - AWS CloudFormation templates (current)
- `gcp/` - Google Cloud Deployment Manager (future)
- `azure/` - Azure Resource Manager templates (future)
