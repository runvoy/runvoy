# Issue 2: Refactor CLI Commands for Testability (Low-Risk)

**Related:** #60, #1 (if created)  
**Priority:** P1  
**Effort:** 3-5 days  
**Labels:** `refactoring`, `testing`, `cli`

---

## Description

Refactor CLI commands to use dependency injection and extract business logic from cobra handlers. This enables comprehensive unit testing without breaking existing functionality.

**Goal:** Make CLI commands testable with mocks while maintaining 100% backward compatibility.

---

## Tasks

### Phase 2.1: Create Interfaces (2 days)

- [ ] **Create Client Interface**
  - [ ] Create `internal/client/interface.go` with `Interface` that defines all client methods
  - [ ] Ensure existing `Client` struct implements the interface (should be automatic)
  - [ ] Update commands to use `client.Interface` type instead of concrete `*client.Client`
  - [ ] Verify no breaking changes to existing code

- [ ] **Create Output Interface**
  - [ ] Create `cmd/runvoy/cmd/output_interface.go` with `OutputInterface`
  - [ ] Create wrapper struct that implements the interface using existing `output.*` functions
  - [ ] Document the interface contract

### Phase 2.2: Extract Business Logic (2-3 days)

- [ ] **Refactor `status.go`** (simplest, start here)
  - [ ] Create `StatusService` struct with `DisplayStatus()` method
  - [ ] Extract logic from `statusRun()` into service
  - [ ] Update `statusRun()` to use service
  - [ ] Add unit tests for `StatusService`
  - [ ] Verify command still works end-to-end

- [ ] **Refactor `logs.go`**
  - [ ] Create `LogsService` struct with `DisplayLogs()` method
  - [ ] Extract log formatting logic
  - [ ] Update `logsRun()` to use service
  - [ ] Add unit tests with mocks
  - [ ] Verify command still works end-to-end

- [ ] **Refactor `run.go`**
  - [ ] Create `RunService` struct with `ExecuteCommand()` method
  - [ ] Extract environment variable parsing (already done in Issue #1)
  - [ ] Extract command request building logic
  - [ ] Update `runRun()` to use service
  - [ ] Add unit tests with mocks
  - [ ] Verify command still works end-to-end

### Phase 2.3: Add Test Infrastructure (1 day)

- [ ] **Set up mocking framework**
  - [ ] Add `gomock` or similar to dependencies
  - [ ] Create mock generation scripts/commands
  - [ ] Generate mocks for `ClientInterface` and `OutputInterface`
  - [ ] Document mock usage in testing patterns

- [ ] **Create test utilities**
  - [ ] Add test helpers for creating mock clients
  - [ ] Add test helpers for capturing output
  - [ ] Create fixtures for common test data

---

## Acceptance Criteria

- [ ] All refactored commands have >80% unit test coverage
- [ ] All existing CLI functionality works identically (backward compatible)
- [ ] Commands can be tested with mocks (no real HTTP calls)
- [ ] Test infrastructure is documented
- [ ] CI passes all tests
- [ ] No performance regression
- [ ] Code review approved

---

## Implementation Notes

### Service Pattern Example

```go
// status.go
type StatusService struct {
    client client.Interface
    output OutputInterface
}

func NewStatusService(client client.Interface, output OutputInterface) *StatusService {
    return &StatusService{
        client: client,
        output: output,
    }
}

func (s *StatusService) DisplayStatus(ctx context.Context, executionID string) error {
    status, err := s.client.GetExecutionStatus(ctx, executionID)
    if err != nil {
        return fmt.Errorf("failed to get status: %w", err)
    }
    
    // Format and display status
    s.output.KeyValue("Execution ID", status.ExecutionID)
    s.output.KeyValue("Status", status.Status)
    // ... rest of formatting
    return nil
}

// Updated command handler
func statusRun(cmd *cobra.Command, args []string) {
    executionID := args[0]
    cfg, err := getConfigFromContext(cmd)
    if err != nil {
        output.Errorf("failed to load configuration: %v", err)
        return
    }

    c := client.New(cfg, slog.Default())
    service := NewStatusService(c, outputWrapper{})
    if err := service.DisplayStatus(cmd.Context(), executionID); err != nil {
        output.Errorf(err.Error())
    }
}
```

### Testing with Mocks

```go
// status_test.go
func TestStatusService_DisplayStatus(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockClient := mocks.NewMockInterface(ctrl)
    mockOutput := mocks.NewMockOutputInterface(ctrl)

    // Set expectations
    mockClient.EXPECT().
        GetExecutionStatus(gomock.Any(), "exec-123").
        Return(&api.ExecutionStatusResponse{
            ExecutionID: "exec-123",
            Status:      "RUNNING",
            StartedAt:   time.Now(),
        }, nil)

    mockOutput.EXPECT().KeyValue("Execution ID", "exec-123")
    mockOutput.EXPECT().KeyValue("Status", "RUNNING")
    // ... more expectations

    service := NewStatusService(mockClient, mockOutput)
    err := service.DisplayStatus(context.Background(), "exec-123")
    assert.NoError(t, err)
}
```

**Reference:** See `docs/CLI_TESTABILITY_ANALYSIS.md` section "Refactoring Proposals" for complete examples.

---

## Success Metrics

- Test coverage: 20% ? 60%+
- Number of services created: 3+
- Number of test files: 3+
- All CLI commands remain fully functional
- Zero breaking changes

---

## Testing Strategy

1. **Unit Tests:** Test service logic with mocks
2. **Integration Tests:** Test full command execution with test HTTP server
3. **Manual Testing:** Verify each command works as before

---

## Rollout Plan

1. Complete Phase 2.1 (interfaces) - review and merge
2. Refactor one command (`status.go`) - review and merge
3. Refactor remaining commands incrementally
4. Add comprehensive test suite

---

## References

- [CLI Testability Analysis](../docs/CLI_TESTABILITY_ANALYSIS.md)
- [Testing Strategy](../docs/TESTING_STRATEGY.md)
- Issue #1: Quick Win Tests (prerequisite)
