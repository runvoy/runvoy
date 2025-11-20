# Component Testing Guide

This guide will help you add tests for Svelte components in the webapp.

## Getting Started with Component Tests

### Example: Testing a Simple Component

Let's say you want to test `StatusBar.svelte`. Create `StatusBar.svelte.test.ts`:

```typescript
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import StatusBar from './StatusBar.svelte';

describe('StatusBar.svelte', () => {
  it('should render the status bar', () => {
    render(StatusBar);
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('should display status text', () => {
    render(StatusBar, { props: { status: 'RUNNING' } });
    expect(screen.getByText('RUNNING')).toBeInTheDocument();
  });
});
```

### Naming Convention

- **Svelte components:** `ComponentName.svelte`
- **Component tests:** `ComponentName.svelte.test.ts`
- **Place in same directory** as the component

## Testing Components with Props

```typescript
import { render, screen } from '@testing-library/svelte';
import MyComponent from './MyComponent.svelte';

it('should accept props', () => {
  render(MyComponent, {
    props: {
      title: 'Test Title',
      count: 5
    }
  });

  expect(screen.getByText('Test Title')).toBeInTheDocument();
});
```

## Testing Component Events

```typescript
import { render, screen, fireEvent } from '@testing-library/svelte';
import Button from './Button.svelte';

it('should emit click event', async () => {
  const { component } = render(Button, {
    props: { label: 'Click me' }
  });

  const clickHandler = vi.fn();
  component.$on('click', clickHandler);

  const button = screen.getByRole('button');
  await fireEvent.click(button);

  expect(clickHandler).toHaveBeenCalled();
});
```

## Testing Stores in Components

```typescript
import { render, screen } from '@testing-library/svelte';
import { executionId } from '../stores/execution';
import ExecutionDisplay from './ExecutionDisplay.svelte';

it('should display execution ID from store', () => {
  executionId.set('exec-123');
  render(ExecutionDisplay);
  expect(screen.getByText('exec-123')).toBeInTheDocument();
});
```

## Testing Async Operations

```typescript
import { render, screen, waitFor } from '@testing-library/svelte';
import DataLoader from './DataLoader.svelte';

it('should load data on mount', async () => {
  vi.mocked(global.fetch).mockResolvedValueOnce({
    ok: true,
    json: vi.fn().mockResolvedValueOnce({ data: 'loaded' })
  } as any);

  render(DataLoader);

  await waitFor(() => {
    expect(screen.getByText('loaded')).toBeInTheDocument();
  });
});
```

## Testing Component Slots

```typescript
import { render, screen } from '@testing-library/svelte';
import Container from './Container.svelte';

it('should render slot content', () => {
  render(Container, {
    slots: {
      default: 'Slot content here'
    }
  });

  expect(screen.getByText('Slot content here')).toBeInTheDocument();
});
```

## Testing Conditional Rendering

```typescript
import { render, screen } from '@testing-library/svelte';
import ErrorMessage from './ErrorMessage.svelte';

it('should show error message when error prop is set', () => {
  render(ErrorMessage, {
    props: { error: 'Something went wrong' }
  });

  expect(screen.getByText('Something went wrong')).toBeInTheDocument();
});

it('should hide error message when error is null', () => {
  render(ErrorMessage, {
    props: { error: null }
  });

  expect(screen.queryByText(/Something went wrong/)).not.toBeInTheDocument();
});
```

## Testing Form Interactions

```typescript
import { render, screen, fireEvent } from '@testing-library/svelte';
import LoginForm from './LoginForm.svelte';

it('should handle form submission', async () => {
  const { component } = render(LoginForm);

  const submitHandler = vi.fn();
  component.$on('submit', submitHandler);

  const input = screen.getByRole('textbox');
  await fireEvent.change(input, { target: { value: 'test@example.com' } });

  const form = screen.getByRole('form');
  await fireEvent.submit(form);

  expect(submitHandler).toHaveBeenCalled();
});
```

## Testing Component Lifecycle

```typescript
import { render } from '@testing-library/svelte';
import LifecycleComponent from './LifecycleComponent.svelte';

it('should handle mount and unmount', () => {
  const { unmount } = render(LifecycleComponent);

  // Component is mounted and running
  expect(true).toBe(true);

  // Unmount and cleanup
  unmount();

  // Component is cleaned up
  expect(true).toBe(true);
});
```

## Priority Component Tests (Quick Wins)

These components are good candidates for testing first:

### High Priority (User-Facing)
1. **RunView.svelte** - Main execution interface
   - Test command input
   - Test execution start
   - Test error handling

2. **LogsView.svelte** - Log display
   - Test log rendering
   - Test WebSocket connection
   - Test log filtering

3. **StatusBar.svelte** - Status display
   - Test status text rendering
   - Test status changes from store

### Medium Priority
4. **LogLine.svelte** - Individual log lines
   - Test ANSI color parsing
   - Test timestamp display

5. **ExecutionSelector.svelte** - Execution selection
   - Test selecting execution
   - Test emitting selection event

6. **ViewSwitcher.svelte** - View navigation
   - Test switching views
   - Test active view highlighting

### Lower Priority
7. **WebSocketStatus.svelte** - Connection status
8. **LogControls.svelte** - Log controls
9. **ClaimView.svelte** - Claim interface
10. **SettingsView.svelte** - Settings page

## Useful Testing Library Queries

```typescript
// By role (most recommended)
screen.getByRole('button', { name: /submit/i })
screen.getByRole('textbox')
screen.getByRole('status')

// By label text
screen.getByLabelText('Email')

// By placeholder
screen.getByPlaceholderText('Enter text...')

// By text
screen.getByText('Click me')

// By test ID (fallback)
screen.getByTestId('my-component')

// Query (doesn't throw if not found)
screen.queryByText('Optional text')

// Find (async version)
screen.findByText('Async text')
```

## Common Testing Patterns

### Mock API Responses
```typescript
vi.mocked(global.fetch).mockResolvedValueOnce({
  ok: true,
  json: vi.fn().mockResolvedValueOnce({ data: 'test' })
} as any);
```

### Wait for Async Operations
```typescript
await waitFor(() => {
  expect(screen.getByText('loaded')).toBeInTheDocument();
});
```

### Test User Input
```typescript
const input = screen.getByRole('textbox');
await fireEvent.change(input, { target: { value: 'new value' } });
```

### Test Form Submission
```typescript
const form = screen.getByRole('form');
await fireEvent.submit(form);
```

## Tips & Best Practices

1. **Test behavior, not implementation** - Focus on what the user sees/does
2. **Use semantic queries** - Prefer `getByRole` over `getByTestId`
3. **Test accessibility** - Good tests = accessible components
4. **Avoid testing internals** - Don't test store subscriptions directly
5. **Keep tests focused** - One assertion per test when possible
6. **Use meaningful names** - `it('should validate email on submit')`
7. **Mock external dependencies** - API calls, WebSockets, etc.
8. **Clean up after tests** - Stores are reset in `beforeEach`

## Running Component Tests

```bash
# Run all tests
npm test

# Run specific component tests
npm test src/components/RunView.svelte.test.ts

# Run with watch
npm test -- --watch

# Generate coverage
npm test:coverage
```

## Resources

- [Testing Library Docs](https://testing-library.com/docs/svelte-testing-library/intro/)
- [Vitest Docs](https://vitest.dev/)
- [Svelte Testing Docs](https://svelte.dev/docs#run-time-svelte-testing)

---

Once you've written component tests, they should pass with the existing test infrastructure. Good luck! 🚀
