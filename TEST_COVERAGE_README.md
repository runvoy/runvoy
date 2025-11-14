# Runvoy Test Coverage Analysis

This directory contains comprehensive analysis of the runvoy test structure and coverage.

## Documents

### 1. TEST_COVERAGE_ANALYSIS.md (22 KB) - **START HERE**
Comprehensive analysis covering:
- Current test distribution (59 test files across codebase)
- Detailed coverage breakdown by package
- Critical gaps (0% coverage areas)
- High-priority gaps (50-70% coverage)
- Testing frameworks and patterns in use
- Current best practices and weaknesses
- Detailed test templates for each area
- 3-phase implementation roadmap with time estimates

**Best for:** Understanding the big picture, detailed planning, implementation templates

### 2. TEST_COVERAGE_QUICK_REFERENCE.md (8.9 KB) - **FOR QUICK LOOKUP**
Quick reference guide covering:
- Current status summary
- Critical gaps at a glance
- High-priority gaps summary
- Testing framework stack
- Key patterns used in codebase
- Recommended implementation order (Phase 1-3)
- Quick start code examples
- Test execution commands
- Success metrics

**Best for:** Quick lookups while implementing, copy-paste code patterns, phase planning

## Quick Overview

### Current Status
- **Overall Coverage:** 49.8% (2,422/4,859 statements)
- **Threshold:** 45% (passing)
- **Target:** 70%+

### Critical Gaps (0% Coverage - Highest Priority)
1. **Event Processor Status Determination** (15-20 hours)
   - File: `internal/providers/aws/events/backend.go`
   - Impact: Core execution tracking
   
2. **User Management Service** (15-20 hours)
   - File: `internal/app/users.go`
   - Impact: Authentication system

3. **Secrets Management** (10-15 hours)
   - Files: `internal/app/secrets.go`, `internal/server/handlers_secrets.go`
   - Impact: Secure credential handling

### High-Priority Gaps (50-70% Coverage)
4. **HTTP Handlers** - Error paths (59.6%)
5. **DynamoDB Repositories** - CRUD operations (49.2%)
6. **AWS App Integration** - Task definitions (39.0%)

### Well-Tested Areas (70%+ Coverage)
- CLI Commands (13 files)
- Core Services (8 files)
- Logger (98.6%), Errors (94.4%), Constants (87.5%)

## Testing Framework Stack

- **Go testing** - Standard library (all tests)
- **testify** - Assertions only (v1.11.1)
- **Manual mocks** - No mockgen or external frameworks
- **Builder pattern** - `testutil.NewUserBuilder()`, `testutil.NewExecutionBuilder()`
- **Table-driven tests** - Standard Go pattern

## Implementation Order

### Phase 1 (Critical - 40-50 hours)
1. Event Processor Status Determination (15-20h)
2. User Management Service (15-20h)
3. Secrets Management (10-15h)

### Phase 2 (Important - 30-40 hours)
4. HTTP Handler Error Paths (15-20h)
5. DynamoDB Repositories (10-15h)

### Phase 3 (Polish - 20-30 hours)
6. AWS App Integration (15-20h)
7. WebSocket Handling (10-15h)

## Key Files for Testing

- **Test utilities:** `/internal/testutil/`
  - `fixtures.go` - UserBuilder, ExecutionBuilder
  - `assert.go` - Custom assertion helpers

- **Test patterns:**
  - `internal/app/mocks_test.go` - Manual mock examples
  - `internal/providers/aws/events/backend_test.go` - Table-driven tests
  - `internal/server/handlers_test.go` - HTTP handler tests

- **Test configuration:**
  - `.testcoverage.yml` - Coverage threshold (45%)
  - `.github/workflows/coverage.yml` - CI/CD workflow

## Quick Commands

```bash
# Run all tests
just test

# Generate coverage report
just gen-coverage

# View HTML coverage
open coverage.html

# Check coverage threshold
just test-coverage

# Run specific test
go test -run TestName ./path/to/package
```

## How to Use This Analysis

1. **First time?** Read `TEST_COVERAGE_ANALYSIS.md` sections 1-4 for overview
2. **Want code examples?** Jump to section 5 in `TEST_COVERAGE_ANALYSIS.md` or `TEST_COVERAGE_QUICK_REFERENCE.md`
3. **Ready to implement?** Follow Phase 1 in implementation roadmap
4. **Need quick reference?** Use `TEST_COVERAGE_QUICK_REFERENCE.md` while coding
5. **Checking coverage?** Use `just test-coverage` and `coverage.html`

## Test Patterns Used

### Manual Mocking
```go
type mockRepository struct {
    getFunc func(ctx context.Context, id string) (*api.Item, error)
}

func (m *mockRepository) Get(ctx context.Context, id string) (*api.Item, error) {
    if m.getFunc != nil {
        return m.getFunc(ctx, id)
    }
    return nil, nil
}
```

### Builder Pattern
```go
user := testutil.NewUserBuilder().
    WithEmail("user@example.com").
    WithLastUsed(time.Now()).
    Build()
```

### Table-Driven Tests
```go
tests := []struct {
    name     string
    input    string
    expected string
}{
    {"case 1", "input1", "output1"},
    {"case 2", "input2", "output2"},
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        result := Function(tt.input)
        assert.Equal(t, tt.expected, result)
    })
}
```

## Success Metrics

After Phase 1: ~58% overall coverage
After Phase 2: ~65% overall coverage
After Phase 3: ~75% overall coverage (target reached)

## Contact Points

- **Coverage reports:** `coverage.out` and `coverage.html`
- **Test configuration:** `.testcoverage.yml`
- **CI/CD:** `.github/workflows/coverage.yml`
- **Test utilities:** `internal/testutil/`

---

**Last Updated:** November 14, 2025
**Analysis Scope:** 59 test files, 4,859 statements across runvoy codebase
**Recommendation:** Start with Phase 1 Critical items for maximum impact
