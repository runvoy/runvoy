<script>
    import { executionId, executionStatus, isCompleted, startedAt } from '../stores/execution.js';
    import { apiEndpoint, apiKey } from '../stores/config.js';
    import APIClient from '../lib/api.js';

    let statusTimer;
    let apiClient;

    $: if ($apiEndpoint && $apiKey) {
        apiClient = new APIClient($apiEndpoint, $apiKey);
    }

    async function fetchStatus() {
        if (!apiClient || !$executionId || $isCompleted) {
            clearInterval(statusTimer);
            return;
        }

        try {
            const response = await apiClient.getStatus($executionId);
            executionStatus.set(response.status);
            startedAt.set(response.started_at);

            const terminalStates = ['SUCCEEDED', 'FAILED', 'STOPPED'];
            if (terminalStates.includes(response.status)) {
                isCompleted.set(true);
                clearInterval(statusTimer);
            }
        } catch (error) {
            console.error('Failed to fetch status:', error);
            // Stop polling on error to avoid spamming
            clearInterval(statusTimer);
        }
    }

    // Poll for status when executionId changes
    $: {
        if ($executionId && apiClient) {
            // Reset state for new execution
            isCompleted.set(false);
            executionStatus.set('LOADING');
            startedAt.set(null);

            // Fetch immediately, then poll
            fetchStatus();
            clearInterval(statusTimer);
            statusTimer = setInterval(fetchStatus, 5000); // Poll every 5 seconds
        }
    }

    // Cleanup timer on component destroy
    import { onDestroy } from 'svelte';
    onDestroy(() => {
        clearInterval(statusTimer);
    });

    $: statusClass = $executionStatus ? $executionStatus.toLowerCase() : 'loading';
    $: formattedStartedAt = $startedAt ? new Date($startedAt).toLocaleString() : 'N/A';
</script>

<div class="status-bar">
    <div class="status-item">
        <strong>Status:</strong>
        <span class="status-badge {statusClass}">{$executionStatus || 'LOADING'}</span>
    </div>
    <div class="status-item">
        <strong>Started:</strong>
        <span>{formattedStartedAt}</span>
    </div>
    <div class="status-item">
        <strong>Execution ID:</strong>
        <code class="execution-id">{$executionId}</code>
    </div>
</div>

<style>
    .status-bar {
        display: flex;
        flex-wrap: wrap;
        gap: 1.5rem;
        align-items: center;
        padding: 1rem;
        background-color: var(--pico-card-background-color);
        border: 1px solid var(--pico-card-border-color);
        border-radius: var(--pico-border-radius);
        margin-bottom: 1rem;
    }

    .status-item {
        display: flex;
        align-items: center;
        gap: 0.5rem;
    }

    .execution-id {
        font-size: 0.9em;
        padding: 0.2em 0.4em;
    }

    .status-badge {
        padding: 0.25em 0.75em;
        border-radius: 1em;
        font-weight: bold;
        font-size: 0.8em;
        text-transform: uppercase;
        color: #fff;
    }

    .status-badge.loading { background-color: #78909c; } /* Blue Grey */
    .status-badge.running { background-color: #2196f3; } /* Blue */
    .status-badge.succeeded { background-color: #4caf50; } /* Green */
    .status-badge.failed { background-color: #f44336; } /* Red */
    .status-badge.stopped { background-color: #ff9800; } /* Orange */
</style>
