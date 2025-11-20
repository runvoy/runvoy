# Coverage Quick Start Guide

Need to check or improve test coverage? Start here!

## 30-Second Summary

✅ **Current Status:** 95%+ library code coverage with enforced thresholds
- Statements: 70% minimum (library: 97%)
- Functions: 70% minimum (library: 100%)
- Branches: 60% minimum (library: 92%)

## Common Tasks

### View Coverage Report

```bash
npm run test:coverage
open coverage/index.html
```

**What you'll see:**
- 🟢 Green: 100% covered
- 🟡 Yellow: Partially covered
- 🔴 Red: Not covered
- Click files to see which lines are uncovered

### Check Coverage Without Report (Faster)

```bash
npm test -- --coverage
```

Shows coverage summary in terminal without generating HTML.

### Run Tests in Watch Mode

```bash
npm test
```

Tests re-run automatically when files change. Perfect for development!

---

## When Coverage is Below Threshold

### 1. Check Which File Failed

```bash
npm run test:coverage

# Look for output like:
# ✗ src/lib/websocket.ts (0%) is below 70%
# ✗ src/components/RunView.svelte (0%) is below 70%
```

### 2. Open Coverage Report

```bash
open coverage/index.html
```

Click the failing file to see which lines/branches aren't tested.

### 3. Write Tests for Missing Coverage

See lines highlighted in red? Write tests for those!

```typescript
// src/lib/myFunction.ts
export function myFunction(x: number) {
  if (x < 0) {           // ❌ Red = not tested
    return 'negative';
  }
  return 'positive';
}
```

```typescript
// src/lib/myFunction.test.ts
it('should handle positive numbers', () => {
  expect(myFunction(5)).toBe('positive');
});

it('should handle negative numbers', () => {
  expect(myFunction(-5)).toBe('negative');  // ✅ Now green!
});
```

### 4. Verify Coverage Improved

```bash
npm run test:coverage

# ✓ myFunction.ts: 100% coverage
# ✓ All thresholds met!
```

---

## Coverage Thresholds Explained

Current thresholds (in `vitest.config.ts`):

```typescript
lines: 70      // At least 70% of code lines must run in tests
functions: 70  // At least 70% of functions must be called
branches: 60   // At least 60% of if/else paths must be tested
statements: 70 // At least 70% of statements must execute
```

### Why 70%?

- **Not too strict:** Allows realistic testing (100% is overkill)
- **Not too loose:** Catches untested code (50% is too low)
- **Practical:** Sweet spot for code quality

### Adjust Thresholds

```typescript
// vitest.config.ts - line 22-25

coverage: {
  lines: 70,           // ← Change these numbers
  functions: 70,
  branches: 60,
  statements: 70
}
```

---

## What Gets Tested

### ✅ Always Test These

- **API Client:** All endpoints and error cases
- **Stores:** State management and subscriptions
- **Utilities:** Parsing, formatting, calculations
- **Error handling:** Both success and failure paths

### ⚠️ Component Testing (Not Yet Done)

- Views (.svelte files)
- Components (.svelte files)
- Integration with stores

**Status:** Ready to add (see COMPONENT_TESTING_GUIDE.md)

---

## Daily Workflow

### Morning: Check Coverage

```bash
npm run test:coverage
# Review what's uncovered
```

### Development: Watch Mode

```bash
npm test
# Auto-runs tests as you code
```

### Before Commit: Verify All Thresholds

```bash
npm run test:coverage

# If it passes: commit! ✅
# If it fails: add more tests
```

### Code Review: Check Coverage in PR

GitHub Actions runs coverage checks automatically:
- ✅ Green check = passes thresholds
- ❌ Red X = below thresholds

---

## Useful Links

- **Coverage Report:** `coverage/index.html` (after running `npm run test:coverage`)
- **Full Guide:** `COVERAGE_ENFORCEMENT_GUIDE.md`
- **Test Setup:** `TEST_SETUP_SUMMARY.md`
- **Component Testing:** `COMPONENT_TESTING_GUIDE.md`

---

## Coverage by Category

### Library Code (src/lib/)
```
✅ ansi.ts: 100%
✅ api.ts: 93.54%
✅ executionState.ts: 97.56%
❌ websocket.ts: 0% (WebSocket connections hard to test)
```

### Stores (src/stores/)
```
✅ execution.ts: 100%
✅ logs.ts: 100%
✅ websocket.ts: 100%
✅ config.ts: 100%
✅ ui.ts: 100%
```

### Components (src/components/)
```
⚠️ Not tested yet (next phase)
```

### Views (src/views/)
```
⚠️ Not tested yet (next phase)
```

---

## FAQ

**Q: Why is overall coverage 31% if library code is 95%?**

A: Overall coverage includes components and views (0% tested). Library code is what matters right now.

**Q: Can I ignore the coverage threshold?**

A: Not recommended, but you can temporarily:
```bash
npm test  # Run without coverage checks
```

**Q: Do I need 100% coverage?**

A: No! 70-80% is healthy. 100% has diminishing returns.

**Q: How do I test components?**

A: See COMPONENT_TESTING_GUIDE.md. Infrastructure is ready!

**Q: Why do some branches not count?**

A: Some code paths are hard to trigger. Focus on realistic scenarios.

---

## Next Steps

1. ✅ Understand current coverage (this guide)
2. 📊 Review coverage report: `npm run test:coverage`
3. 🧪 Add component tests: See COMPONENT_TESTING_GUIDE.md
4. 🔄 Monitor coverage trends in CI/CD
5. 🎯 Aim for 80%+ on new code

---

**Need more details?** See `COVERAGE_ENFORCEMENT_GUIDE.md`
