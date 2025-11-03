# Issue 1: Add Quick Win Tests for CLI Commands

**Related:** #60  
**Priority:** P1  
**Effort:** 1-2 days  
**Labels:** `testing`, `cli`, `good first issue`

---

## Description

Add immediate test coverage for pure functions and helper methods in the CLI commands package. These are low-risk, high-value tests that can be added without any refactoring.

**Current Coverage:** 9.3% (only `parseTimeout()` tested)

---

## Tasks

- [ ] **Test `extractUserEnvVars()` function**
  - Extract the environment variable parsing logic from `run.go` into a pure function
  - Create `run_test.go` with table-driven tests
  - Test cases:
    - Extracts `RUNVOY_USER_*` prefixed environment variables correctly
    - Handles variables without the prefix (ignores them)
    - Returns empty map when no matching variables exist
    - Handles edge cases (empty strings, special characters in values)
    - Handles multiple variables with same prefix

- [ ] **Test helper functions in `root.go`**
  - Add tests for `getConfigFromContext()` error cases
  - Add tests for `getStartTimeFromContext()` with various context scenarios
  - Verify edge cases and nil handling

- [ ] **Add validation tests for command argument parsing**
  - Test argument validation in commands that use `cobra.ExactArgs()` and `cobra.MinimumNArgs()`
  - Ensure proper error messages when arguments are missing

---

## Acceptance Criteria

- [ ] All pure functions in CLI commands have test coverage
- [ ] Test coverage increases to at least 20% for `cmd/runvoy/cmd/` package
- [ ] All tests pass in CI
- [ ] No existing functionality is broken
- [ ] Tests follow table-driven test pattern where appropriate

---

## Implementation Notes

**Example: `extractUserEnvVars()` extraction and test**

1. **Extract function** from `run.go`:
```go
// run.go
func extractUserEnvVars(envVars []string) map[string]string {
    envs := make(map[string]string)
    for _, env := range envVars {
        parts := strings.SplitN(env, "=", constants.EnvVarSplitLimit)
        if len(parts) == 2 && strings.HasPrefix(parts[0], "RUNVOY_USER_") {
            envs[strings.TrimPrefix(parts[0], "RUNVOY_USER_")] = parts[1]
        }
    }
    return envs
}
```

2. **Update `runRun()` to use the extracted function:**
```go
envs := extractUserEnvVars(os.Environ())
```

3. **Add comprehensive tests** in `run_test.go`

**Reference:** See `docs/CLI_TESTABILITY_ANALYSIS.md` for detailed examples.

---

## Success Metrics

- Test coverage: 9.3% ? 20%+
- Number of new test functions: 3-5
- All tests pass with `go test ./cmd/runvoy/cmd -v`

---

## References

- [CLI Testability Analysis](../docs/CLI_TESTABILITY_ANALYSIS.md)
- [Testing Strategy](../docs/TESTING_STRATEGY.md)
