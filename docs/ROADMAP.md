# runvoy Project Roadmap

This document outlines the planned improvements and future direction for the runvoy project. Items are organized by priority and category.

**Last Updated:** 2025-11-01

---

## 🚨 Priority 0: Critical (Immediate Action Required)

### Testing Infrastructure
**Status:** In Progress
**Target:** Q4 2025

- [x] Fix broken test files (compilation errors)
- [ ] Increase test coverage to minimum 70%
  - [ ] Add unit tests for `internal/app` (service layer)
  - [ ] Add unit tests for `internal/providers/aws/database/dynamodb` (repository)
  - [ ] Add unit tests for `internal/app/events` (event processing)
  - [ ] Add unit tests for `internal/auth` (API key validation)
  - [ ] Add unit tests for `internal/providers/aws/app` (ECS runner)
- [ ] Add integration tests for API endpoints
- [ ] Add end-to-end tests for critical workflows
- [ ] Set up code coverage reporting (Codecov)

**Acceptance Criteria:**
- All packages have >70% test coverage
- CI/CD blocks merges if coverage decreases
- No breaking changes without tests

---

### CI/CD Pipeline
**Status:** In Progress
**Target:** Q4 2025

- [x] Add GitHub Actions workflow for testing
- [x] Add GitHub Actions workflow for linting
- [x] Add GitHub Actions workflow for security scanning
- [x] Add GitHub Actions workflow for multi-platform builds
- [ ] Configure branch protection rules
- [ ] Add status badges to README
- [ ] Set up automated deployments to staging
- [ ] Add release automation workflow

**Acceptance Criteria:**
- All PRs run through CI/CD automatically
- Main branch requires passing checks before merge
- Releases are automated via GitHub Actions

---

## 🔥 Priority 1: High (Next 1-2 Months)

### Security Enhancements
**Status:** Planned
**Target:** Q1 2026

- [x] Add Dependabot for dependency updates
- [x] Add govulncheck to pre-commit hooks
- [ ] Add SECURITY.md with vulnerability reporting process
- [ ] Implement rate limiting on API endpoints
- [ ] Add request size limits
- [ ] Add API key rotation mechanism
- [ ] Implement audit log export functionality
- [ ] Add support for MFA on admin operations

**Acceptance Criteria:**
- All dependencies scanned weekly
- Security vulnerabilities patched within 7 days
- Rate limiting prevents abuse
- Audit logs are tamper-proof

---

### Configuration & Environment Management
**Status:** Planned
**Target:** Q1 2026

- [ ] Add configuration validation on startup
- [ ] Implement config struct with validation tags
- [ ] Add environment-specific configs (dev/staging/prod)
- [ ] Make webviewer URL configurable (remove hardcoded constant)
- [ ] Add config hot-reloading for local development
- [ ] Document all configuration options comprehensively

**Acceptance Criteria:**
- Invalid configs fail fast with clear error messages
- All env vars are documented in .env.example
- Config can be validated without running the app

---

### Observability & Monitoring
**Status:** Planned
**Target:** Q1 2026

- [ ] Add OpenTelemetry for distributed tracing
- [ ] Implement application metrics (Prometheus format)
  - [ ] Execution counts by status
  - [ ] Execution duration percentiles
  - [ ] API error rates
  - [ ] Queue depths
- [ ] Add structured error contexts throughout codebase
- [ ] Implement health check with detailed component status
- [ ] Add performance logging for slow operations (>1s)
- [ ] Create Grafana dashboard templates

**Acceptance Criteria:**
- All requests are traceable end-to-end
- Key metrics are exported and queryable
- Alerts fire for critical failures
- Performance regressions are detected automatically

---

## ⚡ Priority 2: Medium (Next 3-6 Months)

### Lock Enforcement
**Status:** Planned
**Target:** Q1 2026

- [ ] Implement lock acquisition before task start
- [ ] Add lock TTL and automatic release
- [ ] Implement lock renewal for long-running tasks
- [ ] Add lock conflict detection and user notification
- [ ] Create lock management CLI commands
  - [ ] `runvoy locks list`
  - [ ] `runvoy locks release <lock-name>`
  - [ ] `runvoy locks status <lock-name>`
- [ ] Add lock metrics and monitoring

**Acceptance Criteria:**
- Concurrent executions with same lock are prevented
- Locks are automatically released on completion/failure
- Stale locks are detected and cleaned up
- Users receive clear error messages on lock conflicts

---

### Custom Container Images
**Status:** Planned
**Target:** Q2 2026

- [ ] Implement task definition override for custom images
- [ ] Add image validation before task start
- [ ] Support private ECR repositories
- [ ] Add image allowlist/blocklist configuration
- [ ] Implement image caching for faster starts
- [ ] Document custom image requirements and best practices
- [ ] Add example Dockerfiles for common use cases

**Acceptance Criteria:**
- Users can specify custom images per execution
- Invalid images fail early with clear errors
- Private images are supported with proper IAM
- Image pull times are minimized via caching

---

### Enhanced Logging & Tailing
**Status:** Partially Implemented
**Target:** Q2 2026

- [ ] Implement server-side log filtering
- [ ] Add log pagination for large executions
- [ ] Implement log search within execution
- [ ] Add log export in multiple formats (JSON, text, CSV)
- [ ] Improve log streaming performance
- [ ] Add log retention policies
- [ ] Implement log archival to S3

**Acceptance Criteria:**
- Users can filter logs by severity/keywords
- Large log sets don't overwhelm the client
- Logs are searchable without downloading entire set
- Old logs are archived automatically

---

### Request ID Improvements
**Status:** Planned
**Target:** Q2 2026

- [ ] Generate request IDs in local server middleware
- [ ] Support X-Request-ID header passthrough
- [ ] Propagate request ID across all service boundaries
- [ ] Add request ID to all log entries
- [ ] Implement request tracing dashboard
- [ ] Document request ID usage for debugging

**Acceptance Criteria:**
- Request IDs work consistently in all environments
- Every log entry includes request ID
- Users can trace requests across all components

---

## 🎯 Priority 3: Nice to Have (6+ Months)

### Multi-Cloud Support
**Status:** Planned
**Target:** Q3 2026

- [ ] Implement GCP Cloud Run provider
- [ ] Implement Azure Container Instances provider
- [ ] Abstract CloudWatch logging to generic interface
- [ ] Add provider-specific configuration validation
- [ ] Document multi-cloud deployment guides
- [ ] Add provider cost comparison tool

**Acceptance Criteria:**
- Users can deploy to GCP/Azure with minimal changes
- Provider-specific features are documented
- Cost tracking can be implemented when cost calculation feature is added

---

### Advanced Features
**Status:** Planned
**Target:** Q3-Q4 2026

- [ ] Implement execution scheduling/cron
- [ ] Add execution dependencies and workflows
- [ ] Support parallel execution limits
- [ ] Add execution templates/presets
- [ ] Implement execution cost calculation and tracking
  - Calculate Fargate cost per execution based on vCPU, memory, and duration
  - Support multi-cloud cost calculation (AWS, GCP, Azure)
  - Store cost data in execution records
- [ ] Implement cost budgets and alerts
- [ ] Add execution analytics and insights
- [ ] Support execution retries with backoff
- [ ] Implement execution cancellation (vs kill)
- [ ] Add execution time limits enforcement

**Acceptance Criteria:**
- Users can schedule recurring executions
- Complex workflows are supported
- Execution costs are tracked and visible
- Costs are predictable and controllable
- Failed executions can retry automatically

---

### Permission & Access Control
**Status:** Planned
**Target:** Q4 2026

- [ ] Implement role-based access control (RBAC)
- [ ] Add resource-level permissions
- [ ] Support team/organization hierarchy
- [ ] Implement OAuth/SAML integration
- [ ] Add permission audit logging
- [ ] Create admin dashboard for user management
- [ ] Support service accounts for CI/CD

**Acceptance Criteria:**
- Granular permissions are enforceable
- Enterprise SSO is supported
- Permission changes are audited
- Teams can manage their own resources

---

### Developer Experience
**Status:** Ongoing
**Target:** Continuous

- [ ] Add goreleaser for automated releases
- [ ] Create homebrew formula for easy installation
- [ ] Add shell completion scripts (bash/zsh/fish)
- [ ] Improve error messages with actionable suggestions
- [ ] Add interactive setup wizard
- [ ] Create video tutorials and demos
- [ ] Build community contribution guidelines
- [ ] Add plugin system for extensibility

**Acceptance Criteria:**
- Installation takes <2 minutes
- New users can deploy successfully in <15 minutes
- Error messages include resolution steps
- Community can contribute features easily

---

## 📋 Technical Debt

### Immediate (Include in next sprint)
- [ ] Fix orphaned tasks handling (see `internal/providers/aws/events/backend.go`)
- [ ] Make webviewer URL configurable (see `internal/constants/constants.go:130`)
- [ ] Replace temporary admin seeding script with permanent solution
- [ ] Add error handling for DynamoDB conditional check failures
- [ ] Improve test helper functions for DynamoDB interactions

### Medium-term (Next 2-3 months)
- [ ] Refactor HTTP routes into constants
- [ ] Extract common error messages to constants
- [ ] Consolidate AWS SDK client initialization
- [ ] Add more godoc comments for exported functions
- [ ] Improve package organization for better discoverability
- [ ] Add code examples for complex APIs

### Long-term (Nice to have)
- [ ] Consider migrating to structured error types (vs wrapped errors)
- [ ] Evaluate replacing chi router with stdlib (Go 1.22+)
- [ ] Consider using AWS CDK instead of CloudFormation
- [ ] Evaluate using DynamoDB streams vs EventBridge

---

## 🚀 Completed

### Q4 2025
- [x] Fixed broken test files compilation errors
- [x] Added comprehensive CI/CD pipeline with GitHub Actions
- [x] Added Dependabot for automated dependency updates
- [x] Added govulncheck to security scanning
- [x] Implemented multi-platform builds (Linux/macOS, AMD64/ARM64)
- [x] Added code coverage tracking
- [x] Implemented security scanning with Trivy

---

## 📊 Metrics & Success Criteria

### Code Quality
- **Target:** 80% test coverage by Q2 2026
- **Target:** Zero high/critical security vulnerabilities
- **Target:** <5 golangci-lint warnings per 1000 LOC
- **Target:** All public APIs documented with godoc

### Performance
- **Target:** P95 API response time <500ms
- **Target:** Execution start time <10s
- **Target:** Log retrieval <2s for 10MB logs

### Reliability
- **Target:** 99.9% API uptime
- **Target:** Zero data loss incidents
- **Target:** <1% failed executions due to platform issues

### Adoption
- **Target:** 100+ GitHub stars by Q2 2026
- **Target:** 10+ community contributors
- **Target:** 5+ production deployments

---

## 🤝 Contributing

This roadmap is a living document. Community feedback and contributions are welcome!

- **Propose features:** Open an issue with the `enhancement` label
- **Vote on priorities:** React with 👍 on existing issues
- **Contribute code:** See CONTRIBUTING.md (to be created)
- **Report bugs:** Open an issue with the `bug` label

---

## 📅 Release Schedule

### Versioning
We follow [Semantic Versioning](https://semver.org/):
- **Major (X.0.0):** Breaking changes
- **Minor (0.X.0):** New features, backwards compatible
- **Patch (0.0.X):** Bug fixes, backwards compatible

### Planned Releases
- **v0.1.0** (Q4 2025): Testing & CI/CD improvements
- **v0.2.0** (Q1 2026): Security & observability enhancements
- **v0.3.0** (Q2 2026): Lock enforcement & custom images
- **v1.0.0** (Q3 2026): Production-ready release with RBAC

---

## 🔗 Related Documents
- [ARCHITECTURE.md](ARCHITECTURE.md) - System architecture
- [README.md](README.md) - Getting started guide
- [CHANGELOG.md](CHANGELOG.md) - Version history (to be created)
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guidelines (to be created)
- [SECURITY.md](SECURITY.md) - Security policy (to be created)

---

**Questions or suggestions?** Open an issue or discussion on GitHub!
