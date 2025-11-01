# GitHub Issues to Create

This document contains all GitHub issues that should be created for the runvoy project improvements.

**Date:** 2025-11-01

---

## 🚨 P0: Critical Issues

### Issue 1: Increase Test Coverage to 70%
**Labels:** `priority: critical`, `type: testing`, `good first issue`

**Title:** Increase test coverage from <5% to 70%+ across all packages

**Description:**
Currently, the project has severely limited test coverage:
- `internal/client`: 1.2%
- `internal/server`: 35.6%
- Most other packages: 0%

This poses significant risks for:
- Regression bugs
- Refactoring confidence
- Code quality assurance

**Tasks:**
- [ ] Add unit tests for `internal/app` (service layer logic)
- [ ] Add unit tests for `internal/database/dynamodb` (repository operations)
- [ ] Add unit tests for `internal/events` (cost calculation, status determination)
- [ ] Add unit tests for `internal/auth` (API key validation)
- [ ] Add unit tests for `internal/app/aws` (ECS runner)
- [ ] Add integration tests for all API endpoints
- [ ] Set up Codecov integration
- [ ] Configure CI to block PRs that decrease coverage

**Acceptance Criteria:**
- All critical packages have >70% coverage
- CI/CD blocks merges if coverage decreases
- Coverage badge added to README

**Priority:** P0 - Critical
**Effort:** High (2-3 weeks)
**Impact:** High

---

### Issue 2: Configure Branch Protection Rules
**Labels:** `priority: critical`, `type: ci/cd`, `infrastructure`

**Title:** Configure GitHub branch protection rules for main branch

**Description:**
Currently, the main branch has no protection rules, allowing direct pushes and merges without quality checks.

**Tasks:**
- [ ] Require PR before merging to main
- [ ] Require status checks to pass (CI/CD)
- [ ] Require at least 1 approval for PRs
- [ ] Require linear history (no merge commits)
- [ ] Require branches to be up to date before merging
- [ ] Restrict force pushes
- [ ] Restrict deletions

**Acceptance Criteria:**
- No direct pushes to main are allowed
- All PRs require passing CI checks
- All PRs require code review

**Priority:** P0 - Critical
**Effort:** Low (1 hour)
**Impact:** High

---

### Issue 3: Add Code Coverage Reporting
**Labels:** `priority: critical`, `type: testing`, `ci/cd`

**Title:** Set up Codecov integration for test coverage tracking

**Description:**
We need visibility into test coverage trends over time and per PR.

**Tasks:**
- [ ] Create Codecov account
- [ ] Add Codecov token to GitHub Secrets
- [ ] Configure Codecov in `.github/workflows/ci.yml`
- [ ] Add coverage badge to README
- [ ] Configure coverage thresholds (70% minimum)
- [ ] Set up coverage diff comments on PRs

**Acceptance Criteria:**
- Coverage is tracked on all PRs
- Coverage trends are visible over time
- PRs show coverage diff in comments

**Priority:** P0 - Critical
**Effort:** Low (2 hours)
**Impact:** High

---

## 🔥 P1: High Priority Issues

### Issue 4: Add SECURITY.md with Vulnerability Reporting Process
**Labels:** `priority: high`, `type: security`, `documentation`

**Title:** Create SECURITY.md with vulnerability reporting guidelines

**Description:**
We need a clear process for security researchers and users to report vulnerabilities responsibly.

**Tasks:**
- [ ] Create SECURITY.md following GitHub's template
- [ ] Define supported versions for security patches
- [ ] Document how to report vulnerabilities (email/private issue)
- [ ] Define response SLA for security issues
- [ ] Add PGP key for encrypted communications (optional)
- [ ] Link SECURITY.md in README

**Acceptance Criteria:**
- SECURITY.md exists and is comprehensive
- Security tab appears on GitHub repository
- Clear contact method for vulnerability reports

**Priority:** P1 - High
**Effort:** Low (1-2 hours)
**Impact:** Medium

---

### Issue 5: Implement Rate Limiting on API Endpoints
**Labels:** `priority: high`, `type: security`, `type: enhancement`

**Title:** Add rate limiting to prevent API abuse

**Description:**
Currently, there's no rate limiting on API endpoints, making the system vulnerable to:
- DDoS attacks
- API abuse
- Cost overruns from malicious users

**Tasks:**
- [ ] Research rate limiting libraries for Go (e.g., `golang.org/x/time/rate`)
- [ ] Implement per-user rate limiting
- [ ] Implement global rate limiting
- [ ] Add rate limit headers to responses (`X-RateLimit-*`)
- [ ] Add rate limit exceeded error handling (429 status)
- [ ] Make rate limits configurable
- [ ] Document rate limits in API docs
- [ ] Add tests for rate limiting

**Acceptance Criteria:**
- Users are rate-limited per API key
- Rate limit information is returned in response headers
- Proper error messages for exceeded limits
- Configuration via environment variables

**Priority:** P1 - High
**Effort:** Medium (3-5 days)
**Impact:** High

---

### Issue 6: Fix Orphaned Tasks Handling
**Labels:** `priority: high`, `type: bug`, `good first issue`

**Title:** Implement proper handling for orphaned ECS tasks

**Description:**
Currently, there's a TODO in `internal/events/ecs_completion.go:50` regarding orphaned tasks (ECS tasks without database records).

```go
// TODO: figure out what to do with orphaned tasks or if we should fail the Lambda
```

**Tasks:**
- [ ] Define orphaned task handling strategy:
  - Option 1: Log and skip
  - Option 2: Create execution record retroactively
  - Option 3: Fail Lambda and alert
- [ ] Implement chosen strategy
- [ ] Add metrics for orphaned tasks
- [ ] Add tests for orphaned task scenarios
- [ ] Document expected behavior

**Acceptance Criteria:**
- Orphaned tasks are handled gracefully
- No Lambda failures from orphaned tasks
- Orphaned tasks are tracked in metrics
- Behavior is documented

**Priority:** P1 - High
**Effort:** Low (1-2 days)
**Impact:** Medium

---

### Issue 7: Fix Failed Cost Calculations
**Labels:** `priority: high`, `type: bug`, `good first issue`

**Title:** Improve error handling for failed cost calculations

**Description:**
Currently, there's a TODO in `internal/events/ecs_completion.go:71` about failed cost calculations.

```go
cost = 0.0 // Continue with zero cost rather than failing? TODO: figure out what to do with failed cost calculations
```

**Tasks:**
- [ ] Define cost calculation failure strategy:
  - Option 1: Set cost to 0 and log warning
  - Option 2: Retry calculation with backoff
  - Option 3: Mark execution with "cost unavailable" flag
- [ ] Add proper error logging with context
- [ ] Add metrics for failed cost calculations
- [ ] Add tests for cost calculation edge cases
- [ ] Document cost calculation limitations

**Acceptance Criteria:**
- Cost calculation failures don't break event processing
- Failures are logged with actionable context
- Users can identify executions with missing costs
- Metrics track cost calculation success rate

**Priority:** P1 - High
**Effort:** Low (1-2 days)
**Impact:** Medium

---

### Issue 8: Make Webviewer URL Configurable
**Labels:** `priority: high`, `type: enhancement`, `good first issue`

**Title:** Make webviewer URL configurable instead of hardcoded

**Description:**
Currently, the webviewer URL is hardcoded in `internal/constants/constants.go:130`.

```go
// TODO: Make this configurable in the future.
const WebviewerURL = "https://runvoy-releases.s3.us-east-2.amazonaws.com/webviewer.html"
```

This prevents:
- Custom deployments with different S3 buckets
- Self-hosted webviewer instances
- Development/testing with local webviewer

**Tasks:**
- [ ] Add `RUNVOY_WEBVIEWER_URL` environment variable
- [ ] Update constant to be loaded from env var with fallback
- [ ] Update `.env.example` with new variable
- [ ] Update documentation
- [ ] Add validation for webviewer URL format
- [ ] Add tests for URL configuration

**Acceptance Criteria:**
- Webviewer URL is configurable via env var
- Default value matches current hardcoded URL
- Invalid URLs are rejected with clear error
- Documentation is updated

**Priority:** P1 - High
**Effort:** Low (1 day)
**Impact:** Medium

---

### Issue 9: Add Configuration Validation on Startup
**Labels:** `priority: high`, `type: enhancement`

**Title:** Implement configuration validation to fail fast on invalid config

**Description:**
Currently, there's no validation of environment variables on startup, leading to:
- Runtime failures with cryptic errors
- Difficult troubleshooting
- Poor developer experience

**Tasks:**
- [ ] Create config validation package
- [ ] Add validation for all required env vars
- [ ] Add validation for env var formats (URLs, timeouts, etc.)
- [ ] Fail fast on startup with clear error messages
- [ ] Add `--validate-config` CLI flag
- [ ] Add tests for config validation
- [ ] Document all configuration options

**Acceptance Criteria:**
- Invalid configs fail immediately on startup
- Error messages clearly indicate what's wrong
- All env vars are validated
- Users can validate config without running the app

**Priority:** P1 - High
**Effort:** Medium (2-3 days)
**Impact:** High

---

## ⚡ P2: Medium Priority Issues

### Issue 10: Add OpenTelemetry for Distributed Tracing
**Labels:** `priority: medium`, `type: enhancement`, `observability`

**Title:** Implement distributed tracing with OpenTelemetry

**Description:**
Currently, there's limited visibility into request flows across components (Lambda, ECS, DynamoDB, etc.).

**Tasks:**
- [ ] Add OpenTelemetry SDK dependencies
- [ ] Implement tracer initialization
- [ ] Add trace context propagation
- [ ] Instrument HTTP handlers
- [ ] Instrument AWS SDK calls
- [ ] Instrument database operations
- [ ] Configure exporter (Jaeger/Zipkin/AWS X-Ray)
- [ ] Add example Grafana dashboard
- [ ] Document tracing setup

**Acceptance Criteria:**
- All requests are traceable end-to-end
- Trace context propagates across all components
- Traces are exportable to standard backends
- Documentation includes setup guide

**Priority:** P2 - Medium
**Effort:** High (1-2 weeks)
**Impact:** High

---

### Issue 11: Implement Application Metrics
**Labels:** `priority: medium`, `type: enhancement`, `observability`

**Title:** Add Prometheus metrics for monitoring

**Description:**
We need operational metrics for monitoring system health and performance.

**Tasks:**
- [ ] Add Prometheus client library
- [ ] Implement `/metrics` endpoint
- [ ] Add execution count metrics (by status, user)
- [ ] Add execution duration histograms (P50, P90, P99)
- [ ] Add API error rate metrics
- [ ] Add cost metrics (total, per user, per execution)
- [ ] Add concurrent execution gauge
- [ ] Create example Grafana dashboard
- [ ] Document metrics and their meaning

**Acceptance Criteria:**
- Metrics endpoint is available
- Key operational metrics are exported
- Dashboards provide insights into system health
- Metrics are documented

**Priority:** P2 - Medium
**Effort:** Medium (1 week)
**Impact:** High

---

### Issue 12: Implement Lock Enforcement
**Labels:** `priority: medium`, `type: feature`, `enhancement`

**Title:** Implement execution locking to prevent concurrent operations

**Description:**
Lock names are currently stored in execution records but not actively enforced. This is a core feature for safe stateful operations (e.g., Terraform).

**Tasks:**
- [ ] Implement lock acquisition in DynamoDB
- [ ] Add lock TTL and automatic release
- [ ] Implement lock renewal for long-running tasks
- [ ] Add lock conflict detection and error handling
- [ ] Return clear error messages on lock conflicts
- [ ] Add lock metrics and monitoring
- [ ] Create CLI commands for lock management
  - [ ] `runvoy locks list`
  - [ ] `runvoy locks release <lock-name>`
  - [ ] `runvoy locks status <lock-name>`
- [ ] Add tests for lock scenarios
- [ ] Document lock behavior

**Acceptance Criteria:**
- Concurrent executions with same lock are prevented
- Locks are automatically released on completion/failure
- Stale locks are detected and cleaned up
- Users receive clear error messages on conflicts
- Lock status is queryable

**Priority:** P2 - Medium
**Effort:** High (2 weeks)
**Impact:** High

---

### Issue 13: Support Custom Container Images
**Labels:** `priority: medium`, `type: feature`, `enhancement`

**Title:** Allow users to specify custom Docker images for executions

**Description:**
The `image` field in execution requests is currently accepted but not used. Users should be able to run commands in custom Docker images.

**Tasks:**
- [ ] Implement task definition override for custom images
- [ ] Add image validation before task start
- [ ] Support private ECR repositories
- [ ] Add image allowlist/blocklist configuration
- [ ] Implement image caching for faster starts
- [ ] Add example Dockerfiles for common use cases
- [ ] Document image requirements and best practices
- [ ] Add tests for custom image scenarios

**Acceptance Criteria:**
- Users can specify custom images per execution
- Invalid images fail early with clear errors
- Private images work with proper IAM
- Image pull times are minimized
- Documentation includes examples

**Priority:** P2 - Medium
**Effort:** High (2 weeks)
**Impact:** High

---

### Issue 14: Add CHANGELOG.md
**Labels:** `priority: medium`, `type: documentation`, `good first issue`

**Title:** Create CHANGELOG.md following Keep a Changelog format

**Description:**
We need a changelog to track version history and communicate changes to users.

**Tasks:**
- [ ] Create CHANGELOG.md following [Keep a Changelog](https://keepachangelog.com/)
- [ ] Document all changes since project inception
- [ ] Set up automation to update changelog on releases
- [ ] Link CHANGELOG in README
- [ ] Add reminder to update changelog in PR template

**Acceptance Criteria:**
- CHANGELOG.md exists and follows standard format
- All versions are documented
- Changelog is updated with each release

**Priority:** P2 - Medium
**Effort:** Low (2-3 hours)
**Impact:** Low

---

### Issue 15: Add CONTRIBUTING.md
**Labels:** `priority: medium`, `type: documentation`, `good first issue`

**Title:** Create CONTRIBUTING.md with contribution guidelines

**Description:**
We need clear guidelines for community contributors.

**Tasks:**
- [ ] Create CONTRIBUTING.md
- [ ] Document development setup process
- [ ] Document coding standards and style guide
- [ ] Document PR process and expectations
- [ ] Document commit message conventions
- [ ] Add testing requirements
- [ ] Add code review guidelines
- [ ] Link CONTRIBUTING in README

**Acceptance Criteria:**
- CONTRIBUTING.md exists and is comprehensive
- New contributors can follow the guide successfully
- Standards are clearly defined

**Priority:** P2 - Medium
**Effort:** Low (3-4 hours)
**Impact:** Medium

---

### Issue 16: Add Status Badges to README
**Labels:** `priority: medium`, `type: documentation`, `good first issue`

**Title:** Add CI/CD and coverage status badges to README

**Description:**
Add visual indicators of project health to README.

**Tasks:**
- [ ] Add GitHub Actions CI badge
- [ ] Add Codecov coverage badge
- [ ] Add Go Report Card badge
- [ ] Add License badge
- [ ] Add Release version badge
- [ ] Add Documentation badge (when available)
- [ ] Add Go version badge

**Acceptance Criteria:**
- All badges are visible and functional
- Badges provide accurate real-time status
- README looks professional

**Priority:** P2 - Medium
**Effort:** Low (30 minutes)
**Impact:** Low

---

## 🎯 P3: Nice to Have Issues

### Issue 17: Add goreleaser for Automated Releases
**Labels:** `priority: low`, `type: enhancement`, `ci/cd`

**Title:** Implement goreleaser for multi-platform release builds

**Description:**
Automate the release process with multi-platform builds.

**Tasks:**
- [ ] Add `.goreleaser.yml` configuration
- [ ] Configure multi-platform builds (Linux/macOS/Windows, AMD64/ARM64)
- [ ] Add homebrew tap for macOS installation
- [ ] Add checksums and signatures
- [ ] Add GitHub release notes automation
- [ ] Add Docker image builds
- [ ] Test release process on tag push

**Acceptance Criteria:**
- Releases are fully automated on tag push
- All platforms are supported
- Release notes are auto-generated
- Installation is simplified

**Priority:** P3 - Low
**Effort:** Medium (1 week)
**Impact:** Medium

---

### Issue 18: Add Shell Completion Scripts
**Labels:** `priority: low`, `type: enhancement`, `good first issue`

**Title:** Generate shell completion scripts for bash/zsh/fish

**Description:**
Improve CLI UX with tab completion.

**Tasks:**
- [ ] Generate bash completion using Cobra
- [ ] Generate zsh completion using Cobra
- [ ] Generate fish completion using Cobra
- [ ] Add installation instructions to README
- [ ] Test completions on each shell

**Acceptance Criteria:**
- Completions work for all major shells
- Installation is documented
- Commands and flags are completable

**Priority:** P3 - Low
**Effort:** Low (1 day)
**Impact:** Low

---

## 📝 Notes

### How to Create These Issues

Since the `gh` CLI is not available, these issues should be created manually or via the GitHub web interface.

**Steps:**
1. Go to https://github.com/runvoy/runvoy/issues/new
2. Copy the title and description from each issue above
3. Add the specified labels
4. Submit the issue

### Priority Definitions

- **P0 (Critical):** Blocking issues, security vulnerabilities, broken main functionality
- **P1 (High):** Important features, significant bugs, high-impact improvements
- **P2 (Medium):** Nice-to-have features, minor bugs, quality improvements
- **P3 (Low):** Future enhancements, polish, convenience features

### Label Conventions

- `priority: critical` - Must be fixed immediately
- `priority: high` - Should be done soon (next 1-2 sprints)
- `priority: medium` - Important but not urgent
- `priority: low` - Future improvements

- `type: bug` - Something is broken
- `type: feature` - New functionality
- `type: enhancement` - Improvement to existing functionality
- `type: documentation` - Docs improvements
- `type: testing` - Test coverage improvements
- `type: security` - Security-related
- `type: ci/cd` - Build/deploy pipeline

- `good first issue` - Good for newcomers
- `help wanted` - Community help would be appreciated
- `infrastructure` - DevOps/infrastructure changes
- `observability` - Monitoring/logging/tracing

---

**Total Issues:** 18
**Critical:** 3
**High:** 6
**Medium:** 7
**Low:** 2
