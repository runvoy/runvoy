<script>
    import { executionId } from '../stores/execution.js';
    import { logEvents, logsRetryCount } from '../stores/logs.js';
    import { cachedWebSocketURL, websocketConnection } from '../stores/websocket.js';

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

    function switchExecution(newExecutionId) {
        if (!newExecutionId || newExecutionId === $executionId) {
            return;
        }

        // Close existing WebSocket if any
        if ($websocketConnection) {
            $websocketConnection.close();
            websocketConnection.set(null);
        }

        // Reset state
        executionId.set(newExecutionId);
        logEvents.set([]);
        logsRetryCount.set(0);
        cachedWebSocketURL.set(null);

        // Update URL query parameter
        const urlParams = new URLSearchParams(window.location.search);
        urlParams.set('execution_id', newExecutionId);
        const newUrl = window.location.pathname + '?' + urlParams.toString();
        window.history.pushState({ executionId: newExecutionId }, '', newUrl);

        // Update page title
        document.title = `runvoy Logs - ${newExecutionId}`;
    }

    // Handle browser back/forward buttons
    if (typeof window !== 'undefined') {
        window.addEventListener('popstate', () => {
            const urlParams = new URLSearchParams(window.location.search);
            const newExecutionId = urlParams.get('execution_id') || urlParams.get('executionId');

            if (newExecutionId && newExecutionId !== $executionId) {
                executionId.set(newExecutionId);
                inputValue = newExecutionId;
                logEvents.set([]);
                logsRetryCount.set(0);

                if ($websocketConnection) {
                    $websocketConnection.close();
                    websocketConnection.set(null);
                }
            } else if (!newExecutionId && $executionId) {
                executionId.set(null);
                inputValue = '';
                logEvents.set([]);
                logsRetryCount.set(0);

                if ($websocketConnection) {
                    $websocketConnection.close();
                    websocketConnection.set(null);
                }
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
