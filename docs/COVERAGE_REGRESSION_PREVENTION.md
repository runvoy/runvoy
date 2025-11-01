# Preventing Test Coverage Regression

## What is Coverage Regression?

**Coverage regression** occurs when new code is added without corresponding tests, causing the overall test coverage percentage to decrease. This is a common problem as projects grow:

### Example Scenario

```
Day 1: 500 lines of code, 400 covered → 80% coverage ✅
Day 2: 600 lines of code, 400 covered → 66% coverage ❌ REGRESSION
```

Even though no tests were removed, coverage dropped because new untested code was added.

## Why Prevention Matters

1. **Maintains Quality Standards**: Ensures new code meets the same quality bar as existing code
2. **Catches Gaps Early**: Identifies missing tests during code review, not in production
3. **Prevents Debt Accumulation**: Stops untested code from piling up
4. **Enforces Discipline**: Makes testing a required part of the development workflow
5. **Visible Accountability**: Makes coverage changes visible in PRs

---

## Implementation Strategies

### Strategy 1: Minimum Coverage Threshold (Recommended for Start)

Set a minimum acceptable coverage percentage. CI fails if coverage drops below this threshold.

**Pros:**
- Simple to implement
- Clear pass/fail criteria
- Good for establishing baseline

**Cons:**
- Doesn't prevent small decreases
- Can be gamed by adding low-value tests
- May need adjustment as project evolves

### Strategy 2: No Coverage Decrease (Strict)

Require that coverage never decreases from the previous commit/PR base.

**Pros:**
- Ensures steady improvement or maintenance
- Prevents any regression
- Encourages comprehensive testing

**Cons:**
- Can be too strict early on
- May block legitimate refactoring
- Requires override mechanism for exceptions

### Strategy 3: Coverage Diff (Granular)

Check that new/changed code specifically has high coverage, regardless of overall percentage.

**Pros:**
- Focuses on new code quality
- Allows refactoring without penalty
- Most flexible approach

**Cons:**
- More complex to implement
- Requires coverage diff tools
- May miss gaps in unchanged code

### Strategy 4: Hybrid Approach (Recommended for Mature Projects)

Combine strategies:
- Minimum threshold: 70% overall
- New code requirement: 80% coverage
- Allow small decreases: -2% maximum

**Best of all worlds** - balanced and practical.

---

## Implementation for Runvoy

### Current Situation

Your current CI in `.github/workflows/ci.yml`:

```yaml
- name: Run tests
  run: go test -v -race -coverprofile=coverage.out ./...

- name: Upload coverage to Codecov
  uses: codecov/codecov-action@v4
  with:
    file: ./coverage.out
    flags: unittests
    name: codecov-umbrella
  continue-on-error: true  # ⚠️ Doesn't fail on coverage issues
```

**Problems:**
- No coverage threshold check
- `continue-on-error: true` means coverage never fails CI
- Coverage is reported but not enforced

---

## Solution 1: Simple Threshold Check (Quick Win)

Add a coverage check step to your CI workflow:

### Updated `.github/workflows/ci.yml`

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  test:
    name: Test
    runs-on: ubuntu-latest
    strategy:
      matrix:
        go-version: ['1.23.x', '1.24.x']

    steps:
      - name: Checkout code
        uses: actions/checkout@v5

      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version: ${{ matrix.go-version }}
          cache: true

      - name: Download dependencies
        run: go mod download

      - name: Verify dependencies
        run: go mod verify

      - name: Run tests
        run: go test -v -race -coverprofile=coverage.out ./...

      # NEW: Coverage threshold check
      - name: Check coverage threshold
        run: |
          COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          THRESHOLD=70.0

          echo "Current coverage: $COVERAGE%"
          echo "Minimum threshold: $THRESHOLD%"

          if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
            echo "❌ Coverage $COVERAGE% is below threshold $THRESHOLD%"
            exit 1
          else
            echo "✅ Coverage $COVERAGE% meets threshold $THRESHOLD%"
          fi

      - name: Upload coverage to Codecov
        uses: codecov/codecov-action@v4
        with:
          file: ./coverage.out
          flags: unittests
          name: codecov-umbrella
        # Remove continue-on-error or set to false
        continue-on-error: false

      - name: Generate coverage report
        run: go tool cover -html=coverage.out -o coverage.html

      - name: Archive coverage results
        uses: actions/upload-artifact@v5
        with:
          name: coverage-report-${{ matrix.go-version }}
          path: |
            coverage.out
            coverage.html
          retention-days: 30
```

### What This Does

1. ✅ Runs tests and generates coverage report
2. ✅ Extracts coverage percentage from report
3. ✅ Compares to minimum threshold (70%)
4. ✅ **Fails CI if coverage is below threshold**
5. ✅ Provides clear pass/fail message

### Setting the Right Threshold

```yaml
# Start conservative
THRESHOLD=10.0  # Current baseline (11%)

# Gradually increase
THRESHOLD=20.0  # After Phase 2
THRESHOLD=40.0  # After Phase 3
THRESHOLD=70.0  # Target
THRESHOLD=80.0  # Stretch goal
```

---

## Solution 2: Progressive Coverage Check (Better)

This version tracks coverage changes and prevents regression:

### Create `.github/scripts/check-coverage.sh`

```bash
#!/bin/bash
set -e

COVERAGE_FILE="coverage.out"
THRESHOLD=${COVERAGE_THRESHOLD:-70.0}
MAX_DECREASE=${MAX_COVERAGE_DECREASE:-2.0}

# Extract current coverage
CURRENT=$(go tool cover -func="$COVERAGE_FILE" | grep total | awk '{print $3}' | sed 's/%//')

echo "📊 Coverage Analysis"
echo "===================="
echo "Current coverage: $CURRENT%"
echo "Minimum threshold: $THRESHOLD%"

# Check against minimum threshold
if (( $(echo "$CURRENT < $THRESHOLD" | bc -l) )); then
    echo "❌ FAIL: Coverage $CURRENT% is below minimum threshold $THRESHOLD%"
    exit 1
fi

# If we have a baseline, check for regression
if [ -f "coverage-baseline.txt" ]; then
    BASELINE=$(cat coverage-baseline.txt)
    DIFF=$(echo "$CURRENT - $BASELINE" | bc -l)

    echo "Baseline coverage: $BASELINE%"
    echo "Coverage change: $DIFF%"

    if (( $(echo "$DIFF < -$MAX_DECREASE" | bc -l) )); then
        echo "❌ FAIL: Coverage decreased by more than $MAX_DECREASE%"
        echo "   Please add tests to cover the new code"
        exit 1
    elif (( $(echo "$DIFF < 0" | bc -l) )); then
        echo "⚠️  WARNING: Coverage decreased by $DIFF%"
        echo "   This is within acceptable range but please add tests"
    elif (( $(echo "$DIFF > 0" | bc -l) )); then
        echo "✅ GREAT: Coverage increased by $DIFF%!"
    else
        echo "✅ Coverage maintained at $CURRENT%"
    fi
else
    echo "ℹ️  No baseline found, establishing baseline at $CURRENT%"
fi

echo "===================="
echo "✅ Coverage check passed!"
```

### Updated CI Workflow

```yaml
- name: Run tests
  run: go test -v -race -coverprofile=coverage.out ./...

- name: Get baseline coverage (for PRs)
  if: github.event_name == 'pull_request'
  run: |
    git fetch origin ${{ github.base_ref }}
    git checkout origin/${{ github.base_ref }}
    go test -coverprofile=coverage-baseline.out ./... || echo "0.0" > coverage-baseline.txt
    if [ -f "coverage-baseline.out" ]; then
      go tool cover -func=coverage-baseline.out | grep total | awk '{print $3}' | sed 's/%//' > coverage-baseline.txt
    fi
    git checkout ${{ github.sha }}

- name: Check coverage
  run: |
    chmod +x .github/scripts/check-coverage.sh
    .github/scripts/check-coverage.sh
  env:
    COVERAGE_THRESHOLD: 70.0
    MAX_COVERAGE_DECREASE: 2.0
```

**This approach:**
- ✅ Checks absolute threshold
- ✅ Compares to baseline (previous commit)
- ✅ Allows small decreases (±2%)
- ✅ Provides detailed feedback

---

## Solution 3: Using Codecov for Regression (Advanced)

Codecov can automatically comment on PRs with coverage changes.

### Update `.codecov.yml` (Create in repo root)

```yaml
coverage:
  status:
    project:
      default:
        # Require 70% total coverage
        target: 70%
        threshold: 2%  # Allow 2% decrease
        if_ci_failed: error

    patch:
      default:
        # Require 80% coverage on changed code
        target: 80%
        if_ci_failed: error

comment:
  layout: "diff, files"
  behavior: default
  require_changes: false
  require_base: true
  require_head: true

github_checks:
  annotations: true
```

### Update CI to Use Codecov Token

```yaml
- name: Upload coverage to Codecov
  uses: codecov/codecov-action@v4
  with:
    file: ./coverage.out
    token: ${{ secrets.CODECOV_TOKEN }}  # Add token to GitHub secrets
    fail_ci_if_error: true  # Fail if upload fails
    flags: unittests
    name: codecov-umbrella
```

**Benefits:**
- ✅ Professional coverage tracking
- ✅ Beautiful PR comments
- ✅ Historical trends
- ✅ Multiple coverage targets (project vs patch)
- ✅ Integration with GitHub checks

**Example Codecov PR Comment:**

```
Coverage: 72.5% (+1.2%) compared to base branch

Files with Coverage Changes:
✅ internal/auth/apikey.go: 85% (+10%)
⚠️  internal/server/handlers.go: 45% (-5%)

View full report at codecov.io
```

---

## Solution 4: Differential Coverage (Most Sophisticated)

Check coverage only on new/changed code using `gocov` and `gocov-html`.

### Add Tools

```bash
go install github.com/axw/gocov/gocov@latest
go install github.com/matm/gocov-html/cmd/gocov-html@latest
```

### Create Script `.github/scripts/check-diff-coverage.sh`

```bash
#!/bin/bash
set -e

# Get list of changed files in this PR
CHANGED_FILES=$(git diff --name-only origin/$GITHUB_BASE_REF...HEAD | grep '\.go$' | grep -v '_test.go' || true)

if [ -z "$CHANGED_FILES" ]; then
    echo "No Go files changed, skipping coverage check"
    exit 0
fi

echo "📝 Changed files:"
echo "$CHANGED_FILES"

# Run tests with coverage
go test -coverprofile=coverage.out ./...

# Convert to gocov format
gocov convert coverage.out > coverage.json

# Calculate coverage for changed files
TOTAL_STATEMENTS=0
COVERED_STATEMENTS=0

for FILE in $CHANGED_FILES; do
    # Extract package from file path
    PKG=$(dirname "$FILE")

    # Get coverage stats for this file
    STATS=$(jq -r ".Packages[] | select(.Name == \"$PKG\") | .Functions[] | select(.File | endswith(\"$FILE\")) | \"\(.Statements) \(.Covered)\"" coverage.json)

    if [ -n "$STATS" ]; then
        while read -r STMTS COVERED; do
            TOTAL_STATEMENTS=$((TOTAL_STATEMENTS + STMTS))
            COVERED_STATEMENTS=$((COVERED_STATEMENTS + COVERED))
        done <<< "$STATS"
    fi
done

if [ $TOTAL_STATEMENTS -eq 0 ]; then
    echo "✅ No testable statements in changed files"
    exit 0
fi

COVERAGE=$(echo "scale=2; ($COVERED_STATEMENTS * 100) / $TOTAL_STATEMENTS" | bc)
THRESHOLD=80

echo ""
echo "📊 Differential Coverage Report"
echo "=============================="
echo "Changed files statements: $TOTAL_STATEMENTS"
echo "Covered statements: $COVERED_STATEMENTS"
echo "Differential coverage: $COVERAGE%"
echo "Required threshold: $THRESHOLD%"
echo ""

if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
    echo "❌ FAIL: New code coverage $COVERAGE% is below threshold $THRESHOLD%"
    echo "Please add tests for the changed code"
    exit 1
fi

echo "✅ PASS: New code coverage meets threshold!"
```

---

## Recommended Approach for Runvoy

Given your current stage (11% coverage, moving to 80%), I recommend a **phased approach**:

### Phase 1: Establish Baseline (Now)

```yaml
- name: Check coverage threshold
  run: |
    COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
    echo "Coverage: $COVERAGE%"
    echo "Baseline: 11.0%"

    # Don't fail yet, just report
    if (( $(echo "$COVERAGE < 10.0" | bc -l) )); then
      echo "⚠️  WARNING: Coverage below 10%"
    fi
```

**Goal:** Track coverage without blocking PRs.

### Phase 2: Soft Enforcement (After Test Infrastructure Complete)

```yaml
- name: Check coverage threshold
  run: |
    COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
    THRESHOLD=11.0  # Current baseline

    if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
      echo "❌ Coverage $COVERAGE% is below $THRESHOLD%"
      exit 1
    fi
```

**Goal:** Prevent coverage from going below current level.

### Phase 3: Progressive Improvement (During Test Writing)

Update threshold as coverage improves:

```yaml
# Week 1: THRESHOLD=11.0
# Week 2: THRESHOLD=20.0
# Week 4: THRESHOLD=40.0
# Week 6: THRESHOLD=60.0
# Week 8: THRESHOLD=70.0
```

**Goal:** Ratchet up standards as tests are added.

### Phase 4: Differential Coverage (Mature State)

Require 80% coverage on new code specifically:

```yaml
- name: Check differential coverage
  run: .github/scripts/check-diff-coverage.sh
  env:
    DIFF_COVERAGE_THRESHOLD: 80.0
```

**Goal:** All new code is well-tested.

---

## Practical Configuration

### Add to Justfile

```makefile
# Check if coverage meets threshold
test-coverage-check threshold='70.0':
    #!/usr/bin/env bash
    go test -coverprofile=coverage.out ./...
    COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
    echo "Coverage: $COVERAGE%"
    echo "Threshold: {{threshold}}%"
    if (( $(echo "$COVERAGE < {{threshold}}" | bc -l) )); then
        echo "❌ Coverage below threshold"
        exit 1
    fi
    echo "✅ Coverage meets threshold"

# Run before committing
pre-commit: lint test-coverage-check
```

### Pre-commit Hook

Create `.git/hooks/pre-push`:

```bash
#!/bin/bash
echo "Running coverage check..."
just test-coverage-check 11.0 || {
    echo "❌ Coverage check failed!"
    echo "Run 'just test-coverage' to see detailed report"
    exit 1
}
```

---

## Monitoring and Reporting

### Coverage Badge in README

Add to your `README.md`:

```markdown
[![codecov](https://codecov.io/gh/runvoy/runvoy/branch/main/graph/badge.svg)](https://codecov.io/gh/runvoy/runvoy)
```

### Coverage Trend Dashboard

Using Codecov, you get:
- 📈 Coverage over time graph
- 📊 Per-package coverage breakdown
- 🎯 Coverage goals tracking
- 📝 Detailed PR comments

---

## Best Practices

### DO ✅

1. **Start Conservative**: Set threshold at current level, increase gradually
2. **Make It Visible**: Show coverage in PRs and README
3. **Allow Exceptions**: Have process for legitimate coverage decreases
4. **Focus on Quality**: Don't chase 100% coverage with meaningless tests
5. **Review Coverage Reports**: Look at what's NOT covered
6. **Educate Team**: Help developers understand why coverage matters

### DON'T ❌

1. **Set Unrealistic Thresholds**: Don't jump to 80% overnight
2. **Block All Decreases**: Allow refactoring and edge cases
3. **Game the System**: Don't add tests just to increase percentage
4. **Ignore Context**: Some code is harder to test (UI, integration points)
5. **Make It Punitive**: Focus on improvement, not blame

---

## Example PR Workflow

### Before (No Coverage Checks)

```
Developer: *adds 200 lines of code, no tests*
CI: ✅ All checks passed
Reviewer: "Looks good!" *merges*
Result: Coverage drops from 40% to 35%
```

### After (With Coverage Checks)

```
Developer: *adds 200 lines of code, no tests*
CI: ❌ Coverage check failed (35% < 40%)
     "Please add tests for new code"
Developer: *adds tests*
CI: ✅ All checks passed (42% > 40%)
Reviewer: *sees coverage improved in PR comment*
Result: Coverage increases from 40% to 42%
```

---

## Summary

### Immediate Action Items

1. **Add coverage threshold check to CI** (Solution 1)
   - Set threshold at 10% (below current 11%)
   - Fail CI if coverage drops below

2. **Configure Codecov properly**
   - Remove `continue-on-error: true`
   - Add `.codecov.yml` with targets

3. **Add justfile command**
   - `just test-coverage-check 11.0`
   - Use in pre-commit hook

4. **Track coverage trend**
   - Add badge to README
   - Review coverage in every PR

### Long-term Strategy

- **Phase 1** (Now): Baseline tracking, no enforcement
- **Phase 2** (2 weeks): Prevent regression below current level
- **Phase 3** (6 weeks): Progressive threshold increases
- **Phase 4** (3 months): Differential coverage on new code

### Success Metrics

- ✅ Coverage never decreases
- ✅ All new code has ≥80% coverage
- ✅ Overall coverage reaches 70%+
- ✅ Team understands and values testing

---

Would you like me to implement any of these solutions in your CI workflow right now?
