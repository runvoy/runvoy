<script lang="ts">
    import { executionId } from '../stores/execution';
    import { switchExecution, clearExecution } from '../lib/executionState';

    let inputValue = $state($executionId || '');

    // Update input when store changes (e.g., from URL on mount)
    $effect(() => {
        inputValue = $executionId || '';
    });

    function handleKeyPress(event: KeyboardEvent): void {
        if (event.key === 'Enter') {
            switchExecution(inputValue.trim());
        }
    }

    function handleBlur(): void {
        const newId = inputValue.trim();
        if (newId && newId !== $executionId) {
            switchExecution(newId);
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
                inputValue = newExecutionId;
            } else if (!newExecutionId && $executionId) {
                clearExecution({ updateHistory: false });
                inputValue = '';
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
            on:keypress={handleKeyPress}
            on:blur={handleBlur}
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
