# runvoy Documentation Index

This document provides a comprehensive index of all runvoy documentation, organized by topic and audience.

## Quick Links

- **New Users** ? Start with [README.md](../README.md) and [Quick Start Guide](#quick-start-guide)
- **Contributors** ? See [CONTRIBUTING.md](../CONTRIBUTING.md) and [Testing Documentation](#testing-documentation)
- **Architects** ? Read [ARCHITECTURE.md](ARCHITECTURE.md) and [System Design](#system-design)
- **Security** ? Review [SECURITY.md](../SECURITY.md) and [Security Documentation](#security-documentation)

---

## Documentation by Audience

### For End Users

| Document | Description | Status |
|----------|-------------|--------|
| [README.md](../README.md) | Quick start, installation, basic usage | ? Complete |
| [CHANGELOG.md](../CHANGELOG.md) | Version history and release notes | ? Current |

### For Developers

| Document | Description | Status |
|----------|-------------|--------|
| [CONTRIBUTING.md](../CONTRIBUTING.md) | Contribution guidelines and workflow | ? Complete |
| [TESTING_QUICKSTART.md](TESTING_QUICKSTART.md) | Quick start for testing | ? Complete |
| [TESTING_EXAMPLES.md](TESTING_EXAMPLES.md) | Testing patterns and examples | ? Complete |
| [TESTING_STRATEGY.md](TESTING_STRATEGY.md) | Comprehensive testing strategy | ? Complete |
| [TESTING_SUMMARY.md](TESTING_SUMMARY.md) | Current testing status | ? Current |
| [COVERAGE_REGRESSION_PREVENTION.md](COVERAGE_REGRESSION_PREVENTION.md) | Coverage guidelines | ? Complete |

### For Architects & Maintainers

| Document | Description | Status |
|----------|-------------|--------|
| [ARCHITECTURE.md](ARCHITECTURE.md) | System architecture and design | ? Complete |
| [PROJECT_ASSESSMENT_2025_11_02.md](PROJECT_ASSESSMENT_2025_11_02.md) | Latest project assessment | ? Current |
| [PROJECT_ANALYSIS.md](PROJECT_ANALYSIS.md) | Detailed project analysis | ? Complete |
| [ROADMAP.md](ROADMAP.md) | Project roadmap and priorities | ? Current |

### For Security Researchers

| Document | Description | Status |
|----------|-------------|--------|
| [SECURITY.md](../SECURITY.md) | Security policy and reporting | ? Complete |
| Security section in [ARCHITECTURE.md](ARCHITECTURE.md) | Security architecture details | ? Complete |

---

## Documentation by Topic

### Quick Start Guide

1. **[README.md](../README.md)** - Start here
   - Overview and key benefits
   - Quick start for admins and users
   - CLI installation
   - Common commands

2. **User Onboarding**
   - Admin setup: Backend deployment with `just init`
   - User setup: Configuration with `runvoy configure`
   - API key claiming with `runvoy claim`

3. **First Command**
   ```bash
   runvoy run "echo hello world"
   ```

### System Design

1. **[ARCHITECTURE.md](ARCHITECTURE.md)** - Complete architecture guide
   - System overview and design principles
   - Folder structure
   - Router architecture
   - Database schema
   - Event processing
   - Web viewer architecture
   - Error handling system
   - Logging architecture

2. **Key Architectural Components**
   - **Orchestrator Lambda**: HTTPS endpoint for synchronous requests
   - **Event Processor Lambda**: Asynchronous event handling
   - **DynamoDB Tables**: State management (users, executions, pending keys)
   - **ECS Fargate**: Command execution in ephemeral containers
   - **EventBridge**: Event-driven architecture for task completion
   - **CloudWatch Logs**: Execution logs and audit trail

3. **Design Patterns**
   - Provider abstraction for multi-cloud support
   - Repository pattern for data access
   - Middleware-based request handling
   - Sidecar container pattern for auxiliary tasks

### Testing Documentation

1. **[TESTING_QUICKSTART.md](TESTING_QUICKSTART.md)** - Start testing quickly
   - Running tests
   - Writing your first test
   - Using test utilities

2. **[TESTING_STRATEGY.md](TESTING_STRATEGY.md)** - Comprehensive strategy
   - Current state analysis
   - Three-layer testing approach (unit/integration/e2e)
   - Test infrastructure setup
   - Implementation patterns
   - 6-phase roadmap

3. **[TESTING_EXAMPLES.md](TESTING_EXAMPLES.md)** - Practical examples
   - Before/after refactoring examples
   - Repository testing patterns
   - HTTP handler testing patterns
   - Test fixture patterns

4. **[TESTING_SUMMARY.md](TESTING_SUMMARY.md)** - Current status
   - Coverage metrics by package
   - Implementation progress
   - What's completed vs. pending

5. **[COVERAGE_REGRESSION_PREVENTION.md](COVERAGE_REGRESSION_PREVENTION.md)** - Quality gates
   - Coverage requirements
   - CI integration
   - Enforcement policies

### Development Workflow

1. **[CONTRIBUTING.md](../CONTRIBUTING.md)** - Contribution guide
   - Getting started
   - Development workflow
   - Coding standards
   - Submitting changes
   - PR process

2. **Development Tools**
   - `justfile` commands (40+ recipes)
   - Pre-commit hooks
   - Local development server
   - Hot reloading with reflex

3. **Code Organization**
   ```
   runvoy/
   ??? cmd/           # Entry points (CLI, lambdas, local server)
   ??? internal/      # Core business logic
   ??? docs/          # Documentation
   ??? deployments/   # CloudFormation templates
   ??? scripts/       # Deployment and utility scripts
   ```

### Security Documentation

1. **[SECURITY.md](../SECURITY.md)** - Security policy
   - Supported versions
   - Security features
   - Vulnerability reporting
   - Security best practices
   - Known limitations
   - Security checklist

2. **Security Features**
   - API key hashing (SHA-256)
   - One-time claim tokens
   - Complete audit trail
   - IAM permissions scoping
   - Automated security scanning

3. **Security Best Practices**
   - For administrators
   - For team members
   - Infrastructure security
   - Configuration security

### Project Planning

1. **[ROADMAP.md](ROADMAP.md)** - Future direction
   - Priority 0: Critical (test coverage, CI/CD)
   - Priority 1: High (security, config management)
   - Priority 2: Medium (lock enforcement, custom images)
   - Priority 3: Nice to have (multi-cloud, advanced features)
   - Completed features

2. **[PROJECT_ASSESSMENT_2025_11_02.md](PROJECT_ASSESSMENT_2025_11_02.md)** - Current state
   - Overall project health (4.5/5)
   - Strengths and weaknesses
   - Detailed metrics
   - Recommendations by priority
   - Success metrics

3. **[PROJECT_ANALYSIS.md](PROJECT_ANALYSIS.md)** - Detailed analysis
   - Code quality metrics
   - Test coverage analysis
   - Technical debt
   - Quick wins implemented

4. **[CHANGELOG.md](../CHANGELOG.md)** - Version history
   - Unreleased changes
   - Version 0.1.0 details
   - Versioning strategy

---

## Documentation Standards

### File Locations

- **Root directory**: User-facing docs (README, CHANGELOG, CONTRIBUTING, SECURITY)
- **`docs/` directory**: Technical docs (ARCHITECTURE, ROADMAP, assessments, testing guides)
- **Code comments**: Inline documentation for complex logic
- **Godoc comments**: All public APIs

### Documentation Format

- **Markdown**: All documentation files use Markdown format
- **Table of Contents**: Long documents include TOC
- **Code Examples**: Includes syntax highlighting with language tags
- **Diagrams**: ASCII art or references to external diagrams
- **Line Numbers**: Code references include file paths and line numbers when relevant

### Keeping Documentation Updated

1. **Code Changes** ? Update inline comments and godoc
2. **API Changes** ? Update ARCHITECTURE.md
3. **CLI Changes** ? Run `just update-readme-help` (automated)
4. **Breaking Changes** ? Update CHANGELOG.md
5. **New Features** ? Update README.md and ROADMAP.md
6. **Security Issues** ? Update SECURITY.md
7. **Assessments** ? Create dated assessment files

### Automated Documentation

- **CLI Help**: Automatically updated in README via `just update-readme-help`
- **Code Coverage**: Tracked in CI and reported to Codecov
- **Test Results**: Displayed in GitHub Actions
- **Security Scans**: Automated via Dependabot and govulncheck

---

## Finding Information

### By Question Type

**"How do I...?"**
- Install runvoy ? [README.md - Installation](../README.md#installation)
- Deploy the backend ? [README.md - Deploy Backend](../README.md#deploy-the-backend-infrastructure-one-time-only)
- Run a command ? [README.md - Usage](../README.md#usage)
- Contribute code ? [CONTRIBUTING.md](../CONTRIBUTING.md)
- Report a bug ? [CONTRIBUTING.md - Submitting Issues](../CONTRIBUTING.md#submitting-changes)
- Write tests ? [TESTING_QUICKSTART.md](TESTING_QUICKSTART.md)

**"What is...?"**
- The architecture ? [ARCHITECTURE.md](ARCHITECTURE.md)
- The roadmap ? [ROADMAP.md](ROADMAP.md)
- The current status ? [PROJECT_ASSESSMENT_2025_11_02.md](PROJECT_ASSESSMENT_2025_11_02.md)
- The testing strategy ? [TESTING_STRATEGY.md](TESTING_STRATEGY.md)

**"Why does it...?"**
- Use event-driven architecture ? [ARCHITECTURE.md - Event Processor](ARCHITECTURE.md#event-processor-architecture)
- Hash API keys ? [SECURITY.md - Security Features](../SECURITY.md#security-features)
- Use sidecar containers ? [ARCHITECTURE.md - ECS Task Architecture](ARCHITECTURE.md#ecs-task-architecture)
- Have low test coverage ? [PROJECT_ASSESSMENT_2025_11_02.md - Test Coverage](PROJECT_ASSESSMENT_2025_11_02.md#1-test-coverage-25---critical)

**"When will...?"**
- Lock enforcement be implemented ? [ROADMAP.md - Lock Enforcement](ROADMAP.md#lock-enforcement)
- Rate limiting be added ? [ROADMAP.md - Security Enhancements](ROADMAP.md#security-enhancements)
- Version 1.0 be released ? [ROADMAP.md - Release Schedule](ROADMAP.md#release-schedule)

---

## Documentation Maintenance

### Review Schedule

- **Weekly**: TESTING_SUMMARY.md, PROJECT_ASSESSMENT (if testing in progress)
- **Monthly**: ROADMAP.md priorities, CHANGELOG.md
- **Per Release**: CHANGELOG.md, version-specific docs
- **On Major Changes**: ARCHITECTURE.md, README.md

### Outdated Documentation

If you find outdated documentation:
1. Open a GitHub issue with label `documentation`
2. Include: What's wrong, where it is, what it should say
3. Submit a PR with the fix (even better!)

### Missing Documentation

If you can't find information you need:
1. Check this index
2. Search across all documentation files
3. Open a GitHub Discussion to ask
4. Once answered, consider contributing the answer as documentation

---

## Contributing to Documentation

Documentation contributions are highly valued! To contribute:

1. **Small fixes**: Just submit a PR
2. **Large additions**: Open an issue first to discuss
3. **New documents**: Follow existing structure and naming
4. **Style guide**: Match existing documentation style

See [CONTRIBUTING.md](../CONTRIBUTING.md) for full guidelines.

---

## Document Status Legend

- ? **Complete**: Comprehensive and up-to-date
- ?? **Needs Update**: Mostly accurate but needs minor updates
- ?? **In Progress**: Being actively updated
- ? **Outdated**: Needs major revision or rewrite
- ?? **Planned**: Not yet created

---

## Quick Reference Card

### Most Important Documents

| I want to... | Read this |
|--------------|-----------|
| Get started quickly | [README.md](../README.md) |
| Understand the architecture | [ARCHITECTURE.md](ARCHITECTURE.md) |
| Contribute code | [CONTRIBUTING.md](../CONTRIBUTING.md) |
| Write tests | [TESTING_QUICKSTART.md](TESTING_QUICKSTART.md) |
| Report security issue | [SECURITY.md](../SECURITY.md) |
| See what's planned | [ROADMAP.md](ROADMAP.md) |
| Check project health | [PROJECT_ASSESSMENT_2025_11_02.md](PROJECT_ASSESSMENT_2025_11_02.md) |

---

**Last Updated**: November 2, 2025  
**Maintainer**: runvoy core team  
**Feedback**: Open an issue or discussion on GitHub
