# GitHub Issues Summary - CLI Testability Improvements

Three GitHub issues have been prepared to address testability improvements for CLI commands (Issue #60).

---

## Issue Templates Created

### 1. **GH_ISSUE_1_CLI_TESTS_QUICK_WINS.md** 
   - **Priority:** P1
   - **Effort:** 1-2 days
   - **Focus:** Add immediate tests for pure functions without refactoring
   - **Deliverables:**
     - Test `extractUserEnvVars()` function
     - Test helper functions in `root.go`
     - Add validation tests
   - **Goal:** Increase coverage from 9.3% to 20%+

### 2. **GH_ISSUE_2_CLI_REFACTORING_LOW_RISK.md**
   - **Priority:** P1  
   - **Effort:** 3-5 days
   - **Focus:** Refactor commands to use dependency injection
   - **Deliverables:**
     - Create Client and Output interfaces
     - Extract business logic from cobra handlers
     - Refactor 3 commands (`status`, `logs`, `run`)
     - Set up mocking infrastructure
   - **Goal:** Enable unit testing with mocks, achieve 60%+ coverage

### 3. **GH_ISSUE_3_CLI_REFACTORING_COMPLETE.md**
   - **Priority:** P2
   - **Effort:** 1-2 weeks
   - **Focus:** Complete refactoring of all remaining commands
   - **Deliverables:**
     - Refactor remaining 5 commands
     - Add integration tests
     - Achieve >80% test coverage
     - Update documentation
   - **Goal:** Comprehensive test coverage for all CLI commands

---

## How to Use

1. **Copy the markdown content** from each file
2. **Create a new GitHub issue** with the title from the template
3. **Paste the content** into the issue body
4. **Add labels** as specified in each template
5. **Set milestone/priority** as appropriate

---

## Dependencies

- Issue #1 should be completed before Issue #2
- Issue #2 should be completed before Issue #3
- Issue #60 (original) can be referenced in all three

---

## Quick Reference

| Issue | File | Effort | Coverage Goal |
|-------|------|--------|---------------|
| #1 | `GH_ISSUE_1_CLI_TESTS_QUICK_WINS.md` | 1-2 days | 9.3% ? 20% |
| #2 | `GH_ISSUE_2_CLI_REFACTORING_LOW_RISK.md` | 3-5 days | 20% ? 60% |
| #3 | `GH_ISSUE_3_CLI_REFACTORING_COMPLETE.md` | 1-2 weeks | 60% ? 80%+ |

---

## Related Documentation

- `docs/CLI_TESTABILITY_ANALYSIS.md` - Detailed analysis and examples
- `docs/TESTING_STRATEGY.md` - Overall testing strategy
- `docs/TESTING_EXAMPLES.md` - Code examples (if exists)
