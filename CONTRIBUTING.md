# Contributing to runvoy

Thank you for your interest in contributing to runvoy! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Coding Standards](#coding-standards)
- [Documentation](#documentation)
- [Community](#community)

## Code of Conduct

### Our Pledge

We are committed to providing a welcoming and inclusive environment for all contributors. We expect:

- **Respectful Communication**: Treat others with kindness and respect
- **Constructive Feedback**: Provide helpful, actionable feedback
- **Collaboration**: Work together towards common goals
- **Inclusivity**: Welcome contributors of all backgrounds and experience levels

### Unacceptable Behavior

- Harassment, discrimination, or personal attacks
- Trolling, insulting comments, or inflammatory remarks
- Publishing others' private information without permission
- Any conduct that would be inappropriate in a professional setting

## Getting Started

### Prerequisites

- **Go 1.25+**: [Install Go](https://golang.org/doc/install)
- **just**: [Install just](https://github.com/casey/just#installation)
- **AWS Account**: For testing cloud deployments (optional)
- **Git**: For version control

### Initial Setup

1. **Fork the repository** on GitHub

2. **Clone your fork**:
   ```bash
   git clone https://github.com/YOUR_USERNAME/runvoy.git
   cd runvoy
   ```

3. **Add upstream remote**:
   ```bash
   git remote add upstream https://github.com/ORIGINAL_OWNER/runvoy.git
   ```

4. **Install dependencies**:
   ```bash
   just dev-setup
   ```

5. **Install pre-commit hooks**:
   ```bash
   just install-hooks
   ```

6. **Create `.env` file**:
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

### Building the Project

```bash
# Build all binaries
just build

# Build only CLI
just build-cli

# Build and run CLI
just runvoy --help
```

### Running Locally

```bash
# Start local development server
just run-local

# Or with hot reloading
just local-dev-server
```

## Development Workflow

### Creating a Feature Branch

```bash
# Update main branch
git checkout main
git pull upstream main

# Create feature branch
git checkout -b feature/my-new-feature
```

### Branch Naming Conventions

- **Features**: `feature/description`
- **Bug Fixes**: `fix/description`
- **Documentation**: `docs/description`
- **Refactoring**: `refactor/description`
- **Tests**: `test/description`

Examples:
- `feature/add-rate-limiting`
- `fix/execution-status-race-condition`
- `docs/update-architecture-diagram`
- `test/add-database-integration-tests`

### Making Changes

1. **Write code** following our [coding standards](#coding-standards)

2. **Add tests** for new functionality:
   ```bash
   # Run tests frequently
   just test
   
   # Check coverage
   just test-coverage
   ```

3. **Format code**:
   ```bash
   # Format Go code
   just fmt
   ```

4. **Run linter**:
   ```bash
   just lint
   ```

5. **Commit changes**:
   ```bash
   git add .
   git commit -m "feat: add rate limiting middleware"
   ```

### Commit Message Guidelines

We follow [Conventional Commits](https://www.conventionalcommits.org/):

**Format**: `<type>(<scope>): <subject>`

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Adding or updating tests
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `chore`: Build process, dependencies, tooling
- `style`: Code formatting (no functional changes)

**Examples**:
```
feat(auth): add API key rotation
fix(server): handle nil pointer in status endpoint
docs(readme): update installation instructions
test(database): add user repository tests
refactor(app): extract AWS runner interface
```

## Testing

### Running Tests

```bash
# All tests
just test

# Specific package
go test ./internal/auth/...

# With coverage
just test-coverage

# Watch mode (rerun on changes)
just test-watch
```

### Writing Tests

1. **Create test files** named `<filename>_test.go`

2. **Use table-driven tests**:
   ```go
   func TestSomething(t *testing.T) {
       tests := []struct {
           name    string
           input   string
           want    string
           wantErr bool
       }{
           {name: "valid input", input: "test", want: "TEST", wantErr: false},
           {name: "empty input", input: "", want: "", wantErr: true},
       }
       
       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               got, err := Something(tt.input)
               if tt.wantErr {
                   assert.Error(t, err)
               } else {
                   assert.NoError(t, err)
                   assert.Equal(t, tt.want, got)
               }
           })
       }
   }
   ```

3. **Use test utilities**:
   ```go
   import "runvoy/internal/testutil"
   
   user := testutil.NewUserBuilder().
       WithEmail("test@example.com").
       Build()
   ```

4. **Follow AAA pattern** (Arrange, Act, Assert)

5. **Test error paths** as thoroughly as happy paths

### Test Coverage Requirements

- **Minimum**: 70% overall coverage
- **New Code**: Must have tests (no exceptions)
- **Critical Paths**: 90%+ coverage required
  - Authentication
  - Database operations
  - API handlers
  - Event processing

## Submitting Changes

### Before Submitting

1. **Update documentation** if behavior changed

2. **Run all checks**:
   ```bash
   just check  # Runs lint + test
   ```

3. **Run pre-commit hooks**:
   ```bash
   just pre-commit-all
   ```

4. **Update CHANGELOG.md** (if applicable)

5. **Rebase on latest main**:
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

### Creating a Pull Request

1. **Push to your fork**:
   ```bash
   git push origin feature/my-new-feature
   ```

2. **Open PR on GitHub** with:
   - Clear title following commit conventions
   - Description of changes
   - Link to related issue (if any)
   - Screenshots/examples (if UI changes)
   - Testing notes

3. **PR Template**:
   ```markdown
   ## Description
   Brief description of changes
   
   ## Motivation and Context
   Why is this change needed? What problem does it solve?
   
   ## How Has This Been Tested?
   - [ ] Unit tests added
   - [ ] Integration tests added
   - [ ] Manually tested locally
   
   ## Types of Changes
   - [ ] Bug fix (non-breaking change which fixes an issue)
   - [ ] New feature (non-breaking change which adds functionality)
   - [ ] Breaking change (fix or feature that would cause existing functionality to change)
   
   ## Checklist
   - [ ] Code follows project style guidelines
   - [ ] Tests added for new functionality
   - [ ] Documentation updated
   - [ ] All tests pass locally
   - [ ] Pre-commit hooks pass
   ```

4. **Respond to review feedback** promptly

5. **Update PR** as needed:
   ```bash
   git add .
   git commit -m "address review feedback"
   git push origin feature/my-new-feature
   ```

### PR Review Process

1. **Automated checks** must pass:
   - Tests
   - Linting
   - Security scans
   - Coverage gates

2. **Code review** by maintainer(s):
   - Review within 3 business days
   - Address all comments
   - Get approval before merge

3. **Merge requirements**:
   - All checks pass ?
   - At least 1 approval ?
   - No merge conflicts ?
   - Branch up-to-date with main ?

## Coding Standards

### Go Code Style

1. **Follow Go conventions**:
   - Use `gofmt` for formatting
   - Use `goimports` for import organization
   - Follow [Effective Go](https://golang.org/doc/effective_go)

2. **Package organization**:
   ```
   internal/
   ??? app/         # Service layer (business logic)
   ??? database/    # Data access layer
   ??? server/      # HTTP handlers and routing
   ??? auth/        # Authentication logic
   ??? ...
   ```

3. **Error handling**:
   - Use our custom error types (`internal/errors`)
   - Wrap errors with context: `errors.Wrap(err, "context")`
   - Return structured errors from API handlers

4. **Naming conventions**:
   - Interfaces: `<Thing>er` (e.g., `Runner`, `Repository`)
   - Constructors: `New<Type>()`
   - Test functions: `Test<FunctionName>_<Scenario>`

5. **Comments**:
   - Public functions/types must have godoc comments
   - Complex logic should have explanatory comments
   - Prefer self-documenting code over excessive comments

### Example Code Structure

```go
// Package server provides HTTP handlers for the runvoy API.
package server

import (
    "context"
    "net/http"
    
    "runvoy/internal/app"
    "runvoy/internal/errors"
)

// HandleRun handles POST /api/v1/run requests.
// It validates the request, starts an execution, and returns the execution details.
func HandleRun(svc *app.Service) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // ARRANGE - Parse request
        var req RunRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            errors.WriteError(w, errors.ErrBadRequest("invalid request body", err))
            return
        }
        
        // ACT - Execute business logic
        exec, err := svc.RunCommand(r.Context(), &req)
        if err != nil {
            errors.WriteError(w, err)
            return
        }
        
        // ASSERT/RESPOND - Return response
        w.WriteHeader(http.StatusAccepted)
        json.NewEncoder(w).Encode(exec)
    }
}
```

## Documentation

### What to Document

1. **Code changes** ? Update inline comments and godoc

2. **API changes** ? Update `docs/ARCHITECTURE.md`

3. **CLI changes** ? Run `just update-readme-help` (automated)

4. **Configuration changes** ? Update README.md and `.env.example`

5. **Breaking changes** ? Update CHANGELOG.md and migration guide

### Documentation Standards

- **README.md**: User-facing documentation
- **ARCHITECTURE.md**: Technical implementation details
- **Code comments**: For complex logic only
- **Godoc**: For all public APIs

### Updating Documentation

```bash
# Update CLI help in README (automatic)
just update-readme-help

# Check documentation links
# (TODO: add link checker to CI)
```

## Community

### Getting Help

- **GitHub Discussions**: Ask questions, share ideas
- **GitHub Issues**: Report bugs, request features
- **Pull Requests**: Submit code contributions

### Communication Guidelines

- **Be respectful**: Treat others as you'd like to be treated
- **Be clear**: Provide context and details
- **Be patient**: Maintainers are volunteers
- **Be constructive**: Suggest solutions, not just problems

### Ways to Contribute

Not just code! We welcome:

- ?? **Bug Reports**: Detailed issue reports help everyone
- ?? **Documentation**: Improve docs, add examples
- ?? **Feature Ideas**: Suggest improvements
- ?? **Testing**: Write tests, report test failures
- ?? **Design**: Improve UI, UX, diagrams
- ?? **Advocacy**: Write blog posts, give talks
- ?? **Community**: Answer questions, help others

## Recognition

Contributors are recognized in:

- CHANGELOG.md (release notes)
- GitHub contributors page
- Release announcements (with permission)

Thank you for contributing to runvoy! ??

---

**Questions?** Open a GitHub Discussion or contact the maintainers.

**Last Updated**: November 2, 2025
