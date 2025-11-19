# Contributing to Runvoy

Thank you for your interest in contributing to Runvoy! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Development Workflow](#development-workflow)
- [Coding Standards](#coding-standards)
- [Testing Requirements](#testing-requirements)
- [Documentation](#documentation)
- [Commit Messages](#commit-messages)
- [Pull Request Process](#pull-request-process)
- [Code Review](#code-review)

## Code of Conduct

This project adheres to a code of conduct that all contributors are expected to follow. Please be respectful, inclusive, and collaborative in all interactions.

## Getting Started

### Prerequisites

- **Go 1.25 or later** - [Install Go](https://golang.org/doc/install)
- **[just](https://github.com/casey/just)** - Command runner for development tasks
- **AWS Account** - For testing backend functionality (optional for CLI-only contributions)
- **Git** - Version control

### Setting Up Your Environment

1. **Fork the repository** on GitHub

2. **Clone your fork:**
   ```bash
   git clone https://github.com/YOUR_USERNAME/runvoy.git
   cd runvoy
   ```

3. **Add upstream remote:**
   ```bash
   git remote add upstream https://github.com/runvoy/runvoy.git
   ```

4. **Install development dependencies:**
   ```bash
   just dev-setup
   ```

5. **Install pre-commit hooks:**
   ```bash
   just install-hook
   ```

6. **Set up environment variables:**
   ```bash
   # Copy example env file (if available)
   cp .env.example .env
   # Or sync from AWS Lambda for local development
   just local-dev-sync
   ```

## Development Setup

The repository ships with a `justfile` to streamline common build, deploy, and QA flows. Run `just --list` to see all available commands.

### Prerequisites for Development

- Go 1.25 or later
- [just](https://github.com/casey/just) command runner
- AWS credentials configured in your shell environment

### Project Structure

```
runvoy/
├── cmd/                    # Application entry points
│   ├── cli/               # CLI client
│   ├── backend/           # Lambda functions
│   ├── local/             # Local dev server
│   └── webapp/            # Web application
├── internal/              # Private application code
│   ├── api/              # API types and handlers
│   ├── auth/             # Authentication & authorization
│   ├── backend/          # Core business logic
│   ├── client/           # CLI client libraries
│   ├── config/           # Configuration management
│   ├── database/         # Database interfaces
│   ├── providers/        # Cloud provider implementations
│   └── server/           # HTTP server components
├── docs/                  # Documentation
├── deploy/                # Infrastructure as Code
└── scripts/               # Utility scripts
```

### Environment Configuration

The `.env` file is automatically created when you run `just init` or `just local-dev-sync`. The `local-dev-sync` command syncs environment variables from the runvoy-orchestrator Lambda function to your local `.env` file for development.

### Environment Setup

First-time setup for new developers:

```bash
# Install dependencies and development tools
just dev-setup

# Install pre-commit hook
just install-hook

# Sync Lambda environment variables to local .env file
just local-dev-sync
```

### Common Commands

The project uses `just` for all development tasks. See all available commands:

```bash
just --list
```

**Local Development Workflow:**

```bash
# Build and run local server (rebuilds on each restart)
just run-local

# Run local server with hot reloading (rebuilds automatically on file changes)
just local-dev-server

# Sync environment variables from AWS
just local-dev-sync
```

**Testing and Quality Checks:**

```bash
# Run all tests
just test

# Run tests with coverage report
just test-coverage

# Lint code
just lint

# Format code
just fmt

# Run both lint and tests
just check
```

**Build and Deploy:**

```bash
# Build all binaries (CLI, orchestrator, event processor, local server)
just build

# Deploy all services
just deploy

# Deploy backend
just deploy-backend

# Deploy webapp
just deploy-webapp

# Or deploy individual services
just deploy-orchestrator
just deploy-event-processor
just deploy-webapp
```

**Infrastructure Management:**

```bash
# Initialize complete backend infrastructure
just init

# Create/update backend infrastructure
just create-backend-infra

# Destroy backend infrastructure
just destroy-backend-infra
```

**Other Useful Commands:**

```bash
# Seed admin user in AWS DynamoDB
just seed-admin-user admin@example.com runvoy-backend

# Update README with latest CLI help output
just update-readme-help

# Clean build artifacts
just clean
```

## Development Workflow

### 1. Create a Branch

Always create a new branch from `main`:

```bash
git checkout main
git pull upstream main
git checkout -b feature/your-feature-name
# or
git checkout -b fix/your-bug-description
```

### 2. Make Your Changes

- Write clean, maintainable code
- Follow the coding standards (see below)
- Add tests for new functionality
- Update documentation as needed

### 3. Test Your Changes

**Always run checks before committing:**

```bash
# This runs both linting and tests (also runs automatically on commit via pre-commit hook)
just check
```

**For local server testing:**

```bash
# Start local development server
just local-dev-server

# In another terminal, test with the CLI
just runvoy --help
just runvoy run "echo hello"
```

### 4. Commit Your Changes

See [Commit Messages](#commit-messages) for guidelines.

```bash
git add .
git commit -m "your commit message"
```

### 5. Push and Create Pull Request

```bash
git push origin feature/your-feature-name
```

Then create a Pull Request on GitHub.

## Coding Standards

### General Guidelines

- **Keep changes small** - Split large changes into smaller, focused PRs
- **Follow Go conventions** - Use `gofmt`, `goimports`, and `golangci-lint`
- **Prefer clarity over cleverness** - Write code that's easy to understand
- **Don't comment unless necessary** - Code should be self-documenting. Use comments only to explain "why", not "what"

### Code Style

1. **Run formatters:**
   ```bash
   just fmt
   ```

2. **Follow linting rules:**
   ```bash
   just lint
   ```

3. **Documentation:**
   - Use function-level documentation for exported functions
   - Keep inline comments minimal - only when code is ambiguous
   - Update `README.md` and `docs/ARCHITECTURE.md` when making significant changes

4. **Naming conventions:**
   - Follow Go naming conventions
   - Use descriptive names
   - Keep functions focused and small

### Special Considerations

- **README.md auto-update sections** - Don't edit sections between `<!-- CLI_HELP_START -->` and `<!-- CLI_HELP_END -->` markers manually
- **docs/CLI.md** - Auto-generated, don't edit directly. Run `just generate-cli-docs` to update
- **Breaking changes** - The project is pre-alpha, but still document significant API changes

## Testing Requirements

### Test Coverage

- **Aim for high coverage** on new code (80%+ for business logic)
- **Write tests first** when possible (TDD approach)
- **Test error paths** - Don't just test happy paths

### Test Structure

Follow the **AAA pattern** (Arrange, Act, Assert):

```go
func TestExample(t *testing.T) {
    // ARRANGE - Set up test data
    user := testutil.NewUserBuilder().Build()

    // ACT - Execute the function
    result, err := service.DoSomething(user)

    // ASSERT - Verify results
    assert.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

### Running Tests

```bash
# Run all tests
just test

# Run with coverage
just test-coverage

# Run specific package
go test ./internal/auth/...

# Run with verbose output
go test -v ./...
```

### Test Types

- **Unit tests** - Fast, isolated tests (no build tags needed)
- **Integration tests** - Use `//go:build integration` tag
- **E2E tests** - Use `//go:build e2e` tag

See [docs/TESTING_STRATEGY.md](docs/TESTING_STRATEGY.md) for comprehensive testing guidelines.

## Documentation

### Code Documentation

- **Export public APIs** - All exported functions, types, and packages should have documentation
- **Function documentation** - Use Go doc comments for exported functions
- **Avoid inline comments** - Only add comments when code needs disambiguation

### Project Documentation

When making significant changes, update:

- **README.md** - For user-facing changes or new features
- **docs/ARCHITECTURE.md** - For architectural changes
- **docs/CLI.md** - Auto-generated, run `just generate-cli-docs` after CLI changes

### Example Documentation Update

If you add a new CLI command:

1. Implement the command
2. Run `just generate-cli-docs` to update CLI documentation
3. Run `just update-readme-help` to update README
4. Update relevant sections in ARCHITECTURE.md if needed

## Commit Messages

### Format

Use clear, descriptive commit messages:

```
<type>: <subject>

<body>

<footer>
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Test additions or changes
- `refactor`: Code refactoring (no behavior change)
- `chore`: Maintenance tasks, dependencies, etc.
- `style`: Formatting, whitespace, etc.

### Examples

```
feat: add support for custom timeout per execution

Allow users to specify execution timeout via CLI flag or API parameter.
Defaults to 10 minutes if not specified.

Fixes #123
```

```
fix: handle missing CloudWatch log streams gracefully

Return 503 Service Unavailable instead of 500 when log stream doesn't
exist yet, allowing clients to retry.
```

```
docs: update architecture diagram with simplified component list

Simplified the AWS backend section to list technologies instead of
detailed flow diagram for better readability.
```

## Pull Request Process

### Before Submitting

1. ✅ **All tests pass** - `just check` succeeds
2. ✅ **Code is formatted** - `just fmt` has been run
3. ✅ **No linting errors** - `just lint` passes
4. ✅ **Documentation updated** - README, ARCHITECTURE, or CLI docs updated as needed
5. ✅ **Commit messages are clear** - Follow the commit message guidelines

### PR Description Template

```markdown
## Description
Brief description of what this PR does.

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Documentation update
- [ ] Refactoring
- [ ] Other (please describe)

## Testing
- [ ] Added tests for new functionality
- [ ] All existing tests pass
- [ ] Manually tested (describe scenarios)

## Checklist
- [ ] Code follows style guidelines
- [ ] Self-review completed
- [ ] Comments added for complex code
- [ ] Documentation updated
- [ ] No new warnings generated
- [ ] Tests added/updated
- [ ] All tests pass locally

## Related Issues
Closes #123
```

### PR Guidelines

- **One feature/fix per PR** - Keep PRs focused and reviewable
- **Small, incremental changes** - Easier to review and merge
- **Descriptive title** - Clear what the PR does
- **Link to issues** - Reference related issues
- **Respond to feedback** - Be open to suggestions and improvements

## Code Review

### Review Process

1. **Automated checks** - CI runs tests and linting automatically
2. **Human review** - At least one maintainer reviews the code
3. **Address feedback** - Make requested changes and update the PR
4. **Approval** - Once approved, maintainers will merge

### What Reviewers Look For

- ✅ Code correctness and functionality
- ✅ Test coverage and quality
- ✅ Code style and formatting
- ✅ Documentation completeness
- ✅ Performance considerations
- ✅ Security implications
- ✅ Backward compatibility (if applicable)

### Responding to Review Feedback

- **Be respectful** - Reviews are collaborative, not personal
- **Ask questions** - If feedback is unclear, ask for clarification
- **Make changes** - Address all feedback or discuss alternatives
- **Keep PR updated** - Push updates to address feedback

## Getting Help

- **Questions?** - Open a GitHub Discussion
- **Found a bug?** - Open an Issue
- **Need help with code?** - Ask in your PR comments

## Additional Resources

- [Architecture Documentation](docs/ARCHITECTURE.md)
- [Testing Strategy](docs/TESTING_STRATEGY.md)
- [Testing Quick Start](docs/TESTING_QUICKSTART.md)
- [CLI Documentation](docs/CLI.md)

Thank you for contributing to Runvoy! 🚀

## Release process

To release a new version:

1. bump the version in [VERSION](./VERSION)
2. update the [CHANGELOG](./CHANGELOG.md) with the new version and changelog entries
3. commit the changes
4. run `just release` (create a GitHub release and upload the binaries to S3 bucket via Goreleaser)

NOTE: we might want to add a GitHub Actions workflow to automate the release process: <https://goreleaser.com/ci/actions/>
