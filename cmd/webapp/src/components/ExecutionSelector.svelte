<script lang="ts">
    import { executionId } from '../stores/execution';
    import { switchExecution, clearExecution } from '../lib/executionState';

    // Store value for syncing
    const storeValue = $derived($executionId || '');

    // Track pending user input
    let pendingInput: string | null = null;

    // Current value to display (pending input or store value)
    const currentValue = $derived(pendingInput ?? storeValue);

    function handleKeyPress(event: KeyboardEvent): void {
        if (event.key === 'Enter') {
            const value = currentValue.trim();
            if (value) {
                switchExecution(value);
                pendingInput = null; // Clear pending after commit
            }
        }
    }

    function handleBlur(): void {
        const newId = currentValue.trim();
        if (newId && newId !== $executionId) {
            switchExecution(newId);
        }
        pendingInput = null; // Clear pending after commit
    }

    function handleInput(event: { currentTarget: { value: string } | null }): void {
        const target = event.currentTarget;
        if (target) {
            pendingInput = target.value;
        }
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
            value={currentValue}
            oninput={handleInput}
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
