<script lang="ts">
    import { executionId } from '../stores/execution';
    import { switchExecution, clearExecution } from '../lib/executionState';

    // Track pending user input that hasn't been committed to store yet
    let pendingInput: string | null = null;

    // Current input value (for reading)
    const currentInputValue = $derived(pendingInput ?? ($executionId || ''));

    // Writable $derived: reads from store, allows local edits via pendingInput
    let inputValue = $derived({
        get: () => currentInputValue,
        set: (value: string) => {
            // Store user input as pending until committed (on blur/enter)
            pendingInput = value;
        }
    });

    function handleKeyPress(event: KeyboardEvent): void {
        if (event.key === 'Enter') {
            const value = currentInputValue.trim();
            if (value) {
                switchExecution(value);
                pendingInput = null; // Clear pending after commit
            }
        }
    }

    function handleBlur(): void {
        const newId = currentInputValue.trim();
        if (newId && newId !== $executionId) {
            switchExecution(newId);
        }
        pendingInput = null; // Clear pending after commit
    }

    // Handle browser back/forward buttons
    $effect.root(() => {
        if (typeof window === 'undefined') {
            return;
        }

        const onPopState = () => {
            const urlParams = new URLSearchParams(window.location.search);
            const newExecutionId = urlParams.get('execution_id') || urlParams.get('executionId');

            if (newExecutionId && newExecutionId !== $executionId) {
                switchExecution(newExecutionId, { updateHistory: false });
                pendingInput = null; // Clear pending when store changes externally
            } else if (!newExecutionId && $executionId) {
                clearExecution({ updateHistory: false });
                pendingInput = null; // Clear pending when store changes externally
            }
        };

        window.addEventListener('popstate', onPopState);

        return () => {
            window.removeEventListener('popstate', onPopState);
        };
    });
</script>

<div class="execution-selector">
    <label for="exec-id-input">
        Execution ID:
        <input
            id="exec-id-input"
            type="text"
            bind:value={inputValue}
            onkeypress={handleKeyPress}
            onblur={handleBlur}
            placeholder="Enter execution ID"
            autocomplete="off"
        />
    </label>
</div>

<style>
    .execution-selector {
        margin-bottom: 1.5rem;
    }

    label {
        display: block;
        margin-bottom: 0.5rem;
        font-weight: 500;
    }

    input {
        width: 100%;
        font-family: 'Monaco', 'Courier New', monospace;
        font-size: 0.9375rem;
    }

    @media (max-width: 768px) {
        .execution-selector {
            margin-bottom: 1rem;
        }

        input {
            font-size: 0.875rem;
        }
    }
</style>
