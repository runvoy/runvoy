<script lang="ts">
    interface Props {
        executionId: string | null;
        onExecutionChange?: ((executionId: string) => void) | null;
        embedded?: boolean;
    }

    const { executionId = null, onExecutionChange = null, embedded = false }: Props = $props();

    // Track pending user input
    let pendingInput: string | null = $state(null);

    // Current value to display (pending input or prop value)
    const currentValue = $derived(pendingInput ?? executionId ?? '');

    function submitExecution(value: string): void {
        const trimmed = value.trim();
        if (trimmed && trimmed !== executionId) {
            pendingInput = null;
            if (onExecutionChange) {
                onExecutionChange(trimmed);
            }
        } else {
            pendingInput = null;
        }
    }

    function handleKeyPress(event: KeyboardEvent): void {
        if (event.key === 'Enter') {
            submitExecution(currentValue);
        }
    }

    function handleBlur(): void {
        submitExecution(currentValue);
    }

    function handleInput(event: { currentTarget: { value: string } | null }): void {
        const target = event.currentTarget;
        if (target) {
            pendingInput = target.value;
        }
    }
</script>

<div class="execution-selector" class:embedded>
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

    .execution-selector.embedded {
        margin-bottom: 0;
    }

    .execution-selector.embedded input {
        margin-bottom: 0;
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

        .execution-selector.embedded {
            margin-bottom: 0;
        }

        input {
            font-size: 0.875rem;
        }
    }
</style>
