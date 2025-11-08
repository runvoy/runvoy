<script>
    import { executionId } from '../stores/execution.js';
    import { switchExecution, clearExecution } from '../lib/executionState.js';

    let inputValue = $executionId || '';

    // Update input when store changes (e.g., from URL on mount)
    $: inputValue = $executionId || '';

    function handleKeyPress(event) {
        if (event.key === 'Enter') {
            switchExecution(inputValue.trim());
        }
    }

    function handleBlur() {
        const newId = inputValue.trim();
        if (newId && newId !== $executionId) {
            switchExecution(newId);
        }
    }

    // Handle browser back/forward buttons
    if (typeof window !== 'undefined') {
        window.addEventListener('popstate', () => {
            const urlParams = new URLSearchParams(window.location.search);
            const newExecutionId = urlParams.get('execution_id') || urlParams.get('executionId');

            if (newExecutionId && newExecutionId !== $executionId) {
                switchExecution(newExecutionId, { updateHistory: false });
                inputValue = newExecutionId;
            } else if (!newExecutionId && $executionId) {
                clearExecution({ updateHistory: false });
                inputValue = '';
            }
        });
    }
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
        margin-bottom: 1rem;
    }

    label {
        display: block;
        margin-bottom: 0.5rem;
    }

    input {
        width: 100%;
        font-family: 'Monaco', 'Courier New', monospace;
    }
</style>
