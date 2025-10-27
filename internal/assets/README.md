# Embedded Assets

This directory contains CloudFormation templates that are embedded into the CLI binary at build time using Go's `embed` package.

## Directory Structure

The templates are organized by cloud provider to support future multi-cloud implementations:

```
internal/assets/
├── aws/                  # AWS CloudFormation templates
│   ├── cloudformation-backend.yaml
│   └── cloudformation-lambda-bucket.yaml
├── templates.go          # Go functions to access embedded templates
└── README.md             # This file
```

## Why Embed Templates?

Embedding templates provides several benefits:

1. **Self-contained binary**: The CLI is distributable as a single binary without external dependencies
2. **Version consistency**: Templates are guaranteed to match the CLI version they're built with
3. **No file system dependencies**: No need to ship template files separately or worry about file paths
4. **Reliability**: Templates can't be accidentally modified or deleted after installation

## Modifying Templates

To update CloudFormation templates:

1. Edit the template file directly in `internal/assets/aws/cloudformation-*.yaml`
2. Rebuild the CLI binary: `go build`

The templates are embedded at compile time, so changes require a rebuild.

## How It Works

1. `templates.go` uses `//go:embed aws/*.yaml` to embed the AWS templates
2. At build time, Go reads the template files and embeds them into the binary
3. The `GetCloudFormation*Template()` functions provide access to embedded content
4. No file system access required at runtime

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
