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
        <span>Execution ID:</span>
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

    .execution-selector.embedded label {
        margin-bottom: 0;
        font-size: 0.8125rem;
    }

    .execution-selector.embedded label span {
        display: none;
    }

    label {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        margin-bottom: 0.5rem;
        font-weight: 500;
        flex-wrap: wrap;
    }

    input {
        flex: 0 1 40%;
        font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
        font-size: 0.9375rem;
    }

    .execution-selector.embedded input {
        flex: 1;
        min-width: 20ch;
        max-width: 36ch;
        padding: 0.25rem 0.5rem;
        font-size: 0.8125rem;
        height: auto;
    }

    @media (max-width: 768px) {
        .execution-selector {
            margin-bottom: 1rem;
        }

        .execution-selector.embedded {
            margin-bottom: 0;
        }

        input {
            flex-basis: 100%;
            font-size: 0.875rem;
        }

        .execution-selector.embedded input {
            max-width: none;
            min-width: 0;
        }
    }
</style>
