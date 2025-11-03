# Issue 3: Complete CLI Commands Refactoring and Test Coverage

**Related:** #60, #1, #2 (if created)  
**Priority:** P2  
**Effort:** 1-2 weeks  
**Labels:** `refactoring`, `testing`, `cli`, `enhancement`

---

## Description

Complete the refactoring of all remaining CLI commands to achieve >80% test coverage. This builds on the foundation established in Issues #1 and #2, applying the same patterns to all commands.

**Goal:** Achieve comprehensive test coverage for all CLI commands while maintaining full backward compatibility.

---

## Tasks

### Phase 3.1: Refactor Remaining Commands (1 week)

- [ ] **Refactor `claim.go`**
  - [ ] Create `ClaimService` struct
  - [ ] Extract business logic from `runClaim()`
  - [ ] Add unit tests with mocks
  - [ ] Verify backward compatibility
  - [ ] Test error handling (invalid token, network errors, etc.)

- [ ] **Refactor `configure.go`**
  - [ ] Create `ConfigureService` struct
  - [ ] Extract configuration logic
  - [ ] Mock file system operations for testing
  - [ ] Add unit tests
  - [ ] Test interactive vs non-interactive flows

- [ ] **Refactor `kill.go`**
  - [ ] Create `KillService` struct
  - [ ] Extract command logic
  - [ ] Add unit tests with mocks
  - [ ] Test error scenarios

- [ ] **Refactor `list.go`**
  - [ ] Create `ListService` struct
  - [ ] Extract execution listing and formatting logic
  - [ ] Add unit tests
  - [ ] Test table formatting with various data sets

- [ ] **Refactor `users.go`** (subcommands)
  - [ ] Create `UsersService` struct
  - [ ] Extract user management logic for each subcommand
  - [ ] Add comprehensive unit tests
  - [ ] Test all user operations (create, list, revoke)

- [ ] **Refactor `images.go`** (subcommands)
  - [ ] Create `ImagesService` struct
  - [ ] Extract image management logic
  - [ ] Add unit tests
  - [ ] Test image registration, listing, unregistration

### Phase 3.2: Integration Testing (3-4 days)

- [ ] **Set up test HTTP server infrastructure**
  - [ ] Create test server that mimics API responses
  - [ ] Create test fixtures for common API responses
  - [ ] Create helper utilities for integration tests

- [ ] **Add integration tests for critical workflows**
  - [ ] Test full `run` command flow
  - [ ] Test `logs` command with various log formats
  - [ ] Test `status` command for all execution states
  - [ ] Test `claim` command flow
  - [ ] Test `users create` ? `claim` ? `run` workflow

- [ ] **Add error scenario integration tests**
  - [ ] Test network failures
  - [ ] Test invalid API responses
  - [ ] Test authentication failures
  - [ ] Test timeout scenarios

### Phase 3.3: Test Coverage and Documentation (2-3 days)

- [ ] **Achieve >80% coverage**
  - [ ] Identify uncovered code paths
  - [ ] Add tests for edge cases
  - [ ] Add tests for error paths
  - [ ] Verify coverage with `go test -cover`

- [ ] **Document testing patterns**
  - [ ] Update `docs/TESTING_EXAMPLES.md` with CLI command examples
  - [ ] Document mock usage patterns
  - [ ] Create testing guide for contributors
  - [ ] Add examples to test files as comments

- [ ] **Update CI/CD**
  - [ ] Ensure coverage reporting works in CI
  - [ ] Set coverage threshold (80%) in CI
  - [ ] Add coverage badge to README (if applicable)
  - [ ] Verify coverage tracking with Codecov

---

## Acceptance Criteria

- [ ] All CLI commands have >80% test coverage
- [ ] All commands refactored to use dependency injection pattern
- [ ] Integration tests cover critical user workflows
- [ ] All tests pass in CI
- [ ] Zero breaking changes to CLI behavior
- [ ] Documentation updated with testing patterns
- [ ] Code review approved

---

## Implementation Notes

### Service Pattern (applied consistently)

Each command should follow this pattern:

```go
// {command}.go
type {Command}Service struct {
    client client.Interface
    output OutputInterface
    // Add other dependencies as needed
}

func New{Command}Service(client client.Interface, output OutputInterface) *{Command}Service {
    return &{Command}Service{
        client: client,
        output: output,
    }
}

func (s *{Command}Service) Execute(ctx context.Context, args ...) error {
    // Business logic here
}

func {command}Run(cmd *cobra.Command, args []string) {
    // Minimal cobra integration code
    service := New{Command}Service(client.New(...), outputWrapper{})
    if err := service.Execute(cmd.Context(), ...); err != nil {
        output.Errorf(err.Error())
    }
}
```

### Test Structure

Each command should have:

1. **Unit tests** (`{command}_test.go`)
   - Test service logic with mocks
   - Test all error paths
   - Test edge cases
   - Use table-driven tests where appropriate

2. **Integration tests** (`{command}_integration_test.go`)
   - Test full command execution
   - Use `//go:build integration` tag
   - Test against mock HTTP server

### Example: Complete Test Suite

```go
// claim_test.go
package cmd

import (
    "context"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/golang/mock/gomock"
    "runvoy/internal/api"
    "runvoy/cmd/runvoy/cmd/mocks"
)

func TestClaimService_ClaimAPIKey(t *testing.T) {
    tests := []struct {
        name      string
        token     string
        mockSetup func(*mocks.MockInterface)
        wantErr   bool
    }{
        {
            name:  "successfully claims API key",
            token: "valid-token",
            mockSetup: func(m *mocks.MockInterface) {
                m.EXPECT().
                    ClaimAPIKey(gomock.Any(), "valid-token").
                    Return(&api.ClaimAPIKeyResponse{
                        APIKey: "sk_live_abc123",
                    }, nil)
            },
            wantErr: false,
        },
        {
            name:  "handles invalid token",
            token: "invalid-token",
            mockSetup: func(m *mocks.MockInterface) {
                m.EXPECT().
                    ClaimAPIKey(gomock.Any(), "invalid-token").
                    Return(nil, errors.New("invalid token"))
            },
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctrl := gomock.NewController(t)
            defer ctrl.Finish()
            
            mockClient := mocks.NewMockInterface(ctrl)
            mockOutput := mocks.NewMockOutputInterface(ctrl)
            tt.mockSetup(mockClient)
            
            service := NewClaimService(mockClient, mockOutput)
            err := service.ClaimAPIKey(context.Background(), tt.token)
            
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

---

## Success Metrics

- **Test Coverage:** >80% for `cmd/runvoy/cmd/` package
- **Number of Services:** 8+ (all commands)
- **Number of Test Files:** 8+ unit test files + integration tests
- **Test Execution Time:** <5 seconds for unit tests
- **Zero Breaking Changes:** All CLI commands work identically

---

## Testing Checklist

For each command, ensure:

- [ ] Service struct created
- [ ] Business logic extracted
- [ ] Unit tests with mocks (>80% coverage)
- [ ] Integration test for happy path
- [ ] Error path tests
- [ ] Edge case tests
- [ ] Backward compatibility verified
- [ ] Documentation updated

---

## Rollout Strategy

1. **Week 1:** Refactor remaining commands one at a time
   - Start with simplest (`kill.go`, `list.go`)
   - Move to more complex (`users.go`, `images.go`)
   - End with interactive (`configure.go`)

2. **Week 2:** Integration tests and coverage
   - Set up test infrastructure
   - Add integration tests
   - Fill coverage gaps
   - Update documentation

3. **Review and Merge:** Incremental PRs per command
   - Each command refactoring is a separate PR
   - Easier to review and maintain
   - Lower risk of breaking changes

---

## Dependencies

- ? Issue #1: Quick Win Tests (should be completed)
- ? Issue #2: Low-Risk Refactoring (interfaces and infrastructure should be in place)
- Mock generation tools configured
- Test utilities available

---

## References

- [CLI Testability Analysis](../docs/CLI_TESTABILITY_ANALYSIS.md)
- [Testing Strategy](../docs/TESTING_STRATEGY.md)
- [Testing Examples](../docs/TESTING_EXAMPLES.md)
- Issue #1: Quick Win Tests
- Issue #2: Low-Risk Refactoring

---

## Future Enhancements (Out of Scope)

- E2E tests with real backend (separate issue)
- Performance benchmarks
- CLI command fuzzing
- Accessibility testing
