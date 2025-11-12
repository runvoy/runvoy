<script>
    import { onDestroy, onMount } from 'svelte';
    import { get } from 'svelte/store';

    import ExecutionSelector from '../components/ExecutionSelector.svelte';
    import StatusBar from '../components/StatusBar.svelte';
    import WebSocketStatus from '../components/WebSocketStatus.svelte';
    import LogControls from '../components/LogControls.svelte';
    import LogViewer from '../components/LogViewer.svelte';
    import { executionId, executionStatus, startedAt, isCompleted } from '../stores/execution.js';
    import {
        logEvents,
        logsRetryCount,
        MAX_LOGS_RETRIES,
        LOGS_RETRY_DELAY,
        STARTING_STATE_DELAY
    } from '../stores/logs.js';
    import { cachedWebSocketURL } from '../stores/websocket.js';
    import { connectWebSocket, disconnectWebSocket } from '../lib/websocket.js';

    export let apiClient = null;
    export let isConfigured = false;
    // Allow delay override for testing (e.g., set to 0 in tests to avoid waiting)
    export let delayFn = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

    let errorMessage = '';
    let fetchLogsTimer;
    let currentExecutionId = null;
    let websocketURL = null;
    const TERMINAL_STATES = ['SUCCEEDED', 'FAILED', 'STOPPED'];

    function deriveStartedAtFromLogs(events = []) {
        if (!Array.isArray(events) || events.length === 0) {
            return null;
        }

        const timestamps = events
            .map((event) => event.timestamp)
            .filter((ts) => typeof ts === 'number' && !Number.isNaN(ts));

        if (timestamps.length === 0) {
            return null;
        }

        const earliestTimestamp = Math.min(...timestamps);
        return new Date(earliestTimestamp).toISOString();
    }

    async function fetchLogs() {
        const id = get(executionId);
        if (!apiClient || !id) {
            return;
        }

        clearTimeout(fetchLogsTimer);
        errorMessage = '';

        // Smart initial wait: Check execution status first
        // If STARTING or TERMINATING, wait before first log poll to avoid unnecessary 404s
        try {
            const statusResponse = await apiClient.getExecutionStatus(id);
            const status = statusResponse.status || 'UNKNOWN';

            if (status === 'STARTING') {
                errorMessage = 'Execution is starting (Fargate provisioning takes ~15 seconds)...';
                await delayFn(STARTING_STATE_DELAY);
                errorMessage = ''; // Clear the message after waiting
            } else if (status === 'TERMINATING') {
                errorMessage = 'Execution is terminating, waiting for final state...';
                await delayFn(LOGS_RETRY_DELAY);
                errorMessage = ''; // Clear the message after waiting
            }
        } catch (statusError) {
            // If status check fails, proceed with normal retry logic
            console.warn('Failed to check execution status:', statusError);
        }

        try {
            const response = await apiClient.getLogs(id);
            const events = response.events || [];
            const eventsWithLines = events.map((event, index) => ({
                ...event,
                line: index + 1
            }));
            logEvents.set(eventsWithLines);
            cachedWebSocketURL.set(response.websocket_url || null);
            logsRetryCount.set(0);

            const status = response.status || 'UNKNOWN';
            executionStatus.set(status);

            if (response.started_at) {
                startedAt.set(response.started_at);
            } else {
                const derivedStartedAt = deriveStartedAtFromLogs(events);
                startedAt.set(derivedStartedAt);
            }

            const terminal = TERMINAL_STATES.includes(status);
            isCompleted.set(terminal);

            if (terminal) {
                cachedWebSocketURL.set(null);
            }
        } catch (error) {
            const retryCount = get(logsRetryCount);
            if (error.status === 404 && retryCount < MAX_LOGS_RETRIES) {
                const attempt = retryCount + 1;
                errorMessage = `Execution not found, retrying... (${attempt}/${MAX_LOGS_RETRIES})`;
                logsRetryCount.set(attempt);
                fetchLogsTimer = setTimeout(fetchLogs, LOGS_RETRY_DELAY);
            } else {
                errorMessage = error.details?.error || error.message || 'Failed to fetch logs';
                logEvents.set([]);
            }
        }
    }

    $: currentExecutionId = $executionId;
    $: websocketURL = $cachedWebSocketURL;

    $: {
        if (apiClient && currentExecutionId) {
            disconnectWebSocket();
            executionStatus.set('LOADING');
            startedAt.set(null);
            isCompleted.set(false);
            fetchLogs();
        }
    }

    $: {
        if (websocketURL) {
            connectWebSocket(websocketURL);
        }
    }

    $: showWelcome = !isConfigured;

    $: if (!apiClient) {
        clearTimeout(fetchLogsTimer);
        disconnectWebSocket();
    }

    onMount(() => {
        if (typeof window === 'undefined') {
            return undefined;
        }

        const handleExecutionComplete = () => {
            const id = get(executionId);
            if (!apiClient || !id) {
                return;
            }
            fetchLogs();
        };

        window.addEventListener('runvoy:execution-complete', handleExecutionComplete);

        return () => {
            window.removeEventListener('runvoy:execution-complete', handleExecutionComplete);
        };
    });

    onDestroy(() => {
        clearTimeout(fetchLogsTimer);
        disconnectWebSocket();
    });
</script>

<ExecutionSelector />

{#if errorMessage}
    <article class="error-box">
        <p>{errorMessage}</p>
    </article>
{/if}

{#if showWelcome}
    <article>
        <header>
            <strong>Welcome to runvoy Log Viewer</strong>
        </header>
        <p>To get started:</p>
        <ol>
            <li>Click the "⚙️ Configure API" button to set your API endpoint and key</li>
            <li>Enter an execution ID in the field above</li>
            <li>View logs and monitor execution status in real-time</li>
        </ol>
        <footer>
            <small
                >Your credentials are stored locally in your browser and never sent to third
                parties.</small
            >
        </footer>
    </article>
{:else if !currentExecutionId}
    <article>
        <p>
            Enter an execution ID above or provide <code>?execution_id=&lt;id&gt;</code> in the URL
        </p>
    </article>
{:else}
    <section>
        <StatusBar />
        <WebSocketStatus />
        <LogControls />
        <LogViewer />
    </section>
{/if}

<style>
    article {
        margin-top: 2rem;
    }

    code {
        background: var(--pico-code-background-color);
        padding: 0.25rem 0.5rem;
        border-radius: 0.25rem;
        font-size: 0.9em;
    }

    .error-box {
        background-color: var(--pico-card-background-color);
        border: 1px solid var(--pico-card-border-color);
        border-left: 4px solid var(--pico-color-red-500);
        padding: 1rem 1.5rem;
        margin-top: 2rem;
        border-radius: var(--pico-border-radius);
    }

    .error-box p {
        margin: 0;
        color: var(--pico-color-red-500);
        font-weight: bold;
    }
</style>
