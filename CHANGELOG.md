# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Comprehensive project assessment documentation
- SECURITY.md with vulnerability reporting process
- CONTRIBUTING.md with contribution guidelines
- CHANGELOG.md for version tracking

### Changed
- Documentation structure reorganized for better clarity
- Testing strategy fully documented with 6-phase roadmap

### Fixed
- None

### Security
- None

## [0.1.0] - 2025-11-02

### Added
- Initial public release
- Core execution platform with AWS Lambda + ECS Fargate backend
- API key authentication with SHA-256 hashing
- User management (create, revoke, claim)
- Command execution with audit trail
- Web-based log viewer
- CLI with 12 commands:
  - `runvoy run` - Execute commands remotely
  - `runvoy status` - Check execution status
  - `runvoy logs` - View execution logs
  - `runvoy kill` - Terminate running executions
  - `runvoy list` - List executions
  - `runvoy users create` - Create users
  - `runvoy users revoke` - Revoke users
  - `runvoy claim` - Claim API keys
  - `runvoy configure` - Configure CLI
  - `runvoy images register` - Register Docker images
  - `runvoy images list` - List registered images
  - `runvoy version` - Show version
- Event-driven architecture with EventBridge
- CloudWatch Logs integration
- DynamoDB for state management
- Custom Docker image support
- Git repository cloning in tasks
- Environment variable injection
- Sidecar container pattern
- Execution locking (recorded, not enforced)
- CloudFormation infrastructure templates
- Comprehensive documentation:
  - README.md with quick start
  - ARCHITECTURE.md with system design
  - ROADMAP.md with future plans
  - Testing strategy and examples

### Infrastructure
- Lambda function for orchestration (ARM64)
- Lambda function for event processing (ARM64)
- ECS Fargate cluster (ARM64)
- DynamoDB tables (api-keys, executions, pending-api-keys)
- EventBridge rules for task completion
- CloudWatch Logs for all components
- S3 bucket for releases and web viewer
- IAM roles with minimal permissions
- Lambda Function URLs for HTTPS endpoints

### Developer Tools
- `justfile` with 40+ commands
- Pre-commit hooks (golangci-lint, gofmt, goimports, security checks)
- CI/CD pipeline with GitHub Actions
- Dependabot for dependency updates
- Security scanning (Trivy, govulncheck)
- Local development server
- Hot reloading with reflex
- Coverage tracking with Codecov
- Test utilities (`internal/testutil`)

### Security
- API key hashing (SHA-256)
- One-time claim tokens (15-minute expiration)
- Complete audit trail
- IAM permissions scoped to runvoy resources
- Pre-commit security hooks
- Automated vulnerability scanning

### Documentation
- 430+ line README with examples
- 1,159+ line ARCHITECTURE documentation
- 387 line ROADMAP with priorities
- 429 line PROJECT_ANALYSIS
- 840+ line TESTING_STRATEGY
- Multiple testing guides and examples

### Known Limitations
- Test coverage: 11.1% (target: 80%)
- No rate limiting
- No lock enforcement
- Hardcoded webviewer URL
- No MFA for admin operations

---

## Release History

- **v0.1.0** (2025-11-02): Initial release
  - Core execution platform
  - User management
  - CLI tool
  - Web viewer
  - Comprehensive documentation

---

## Versioning Strategy

We follow [Semantic Versioning](https://semver.org/):

- **MAJOR** version: Breaking changes
- **MINOR** version: New features (backwards compatible)
- **PATCH** version: Bug fixes (backwards compatible)

### Version Tags

- `v0.1.0`: Initial release
- `v0.2.0`: Planned - Security enhancements (rate limiting, observability)
- `v0.3.0`: Planned - Lock enforcement, custom images
- `v1.0.0`: Planned - Production-ready with RBAC

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for how to contribute to runvoy.

---

## Links

- [GitHub Repository](https://github.com/OWNER/runvoy)
- [Documentation](docs/README.md)
- [Issue Tracker](https://github.com/OWNER/runvoy/issues)
- [Discussions](https://github.com/OWNER/runvoy/discussions)

---

**Note**: This changelog will be updated with each release. For unreleased changes, see the [Unreleased] section at the top.
