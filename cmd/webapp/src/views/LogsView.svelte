<script lang="ts">
    import { onDestroy, onMount } from 'svelte';
    import { get } from 'svelte/store';

    import ExecutionSelector from '../components/ExecutionSelector.svelte';
    import StatusBar from '../components/StatusBar.svelte';
    import WebSocketStatus from '../components/WebSocketStatus.svelte';
    import LogControls from '../components/LogControls.svelte';
    import LogViewer from '../components/LogViewer.svelte';
    import { executionId, executionStatus, startedAt, isCompleted } from '../stores/execution';
    import { logEvents } from '../stores/logs';
    import { cachedWebSocketURL } from '../stores/websocket';
    import { connectWebSocket, disconnectWebSocket } from '../lib/websocket';
    import type APIClient from '../lib/api';
    import { normalizeLogEvents, type LogEvent } from '../types/logs';
    import type { ApiError, ExecutionStatusResponse } from '../types/api';

    export let apiClient: APIClient | null = null;
    export let isConfigured = false;

    let errorMessage = '';
    let currentExecutionId: string | null = null;
    let lastProcessedExecutionId: string | null = null;
    const TERMINAL_STATES = ['SUCCEEDED', 'FAILED', 'STOPPED'];

    async function handleKillExecution(): Promise<void> {
        const id = get(executionId);
        if (!apiClient || !id) {
            return;
        }

        try {
            await apiClient.killExecution(id);
            // Refetch logs to get updated status
            await fetchLogs(id);
        } catch (error) {
            const err = error as ApiError;
            errorMessage = err.details?.error || err.message || 'Failed to stop execution';
        }
    }

    function deriveStartedAtFromLogs(events: LogEvent[] = []): string | null {
        if (!Array.isArray(events) || events.length === 0) {
            return null;
        }

        const timestamps = events
            .map((event) => event.timestamp)
            .filter((ts: number) => typeof ts === 'number' && !Number.isNaN(ts));

        if (timestamps.length === 0) {
            return null;
        }

        const earliestTimestamp = Math.min(...timestamps);
        return new Date(earliestTimestamp).toISOString();
    }

    async function fetchLogs(id: string): Promise<void> {
        if (!apiClient || !id) {
            return;
        }

        errorMessage = '';

        try {
            const response = await apiClient.getLogs(id);
            const eventsWithLines: LogEvent[] = normalizeLogEvents(response.events);
            logEvents.set(eventsWithLines);

            // If backend offers a websocket URL, prefer streaming and let websocket handle lifecycle.
            if (response.websocket_url) {
                cachedWebSocketURL.set(response.websocket_url);
                return;
            }

            const status = response.status || 'UNKNOWN';
            executionStatus.set(status);
            const derivedStartedAt = deriveStartedAtFromLogs(eventsWithLines);
            startedAt.set(derivedStartedAt);

            const terminal = TERMINAL_STATES.includes(status);
            isCompleted.set(terminal);
        } catch (error) {
            const err = error as ApiError;
            errorMessage = err.details?.error || err.message || 'Failed to fetch logs';
            logEvents.set([]);
        }
    }

    function resetForExecution(id: string | null): void {
        disconnectWebSocket();
        executionStatus.set('LOADING');
        startedAt.set(null);
        isCompleted.set(false);
        errorMessage = '';
        lastProcessedExecutionId = id;
    }

    async function handleExecutionComplete(): Promise<void> {
        if (!apiClient || !currentExecutionId) {
            return;
        }
        // Websocket disconnect means the task terminated
        // Fetch status to update execution status (SUCCEEDED, FAILED, STOPPED)
        // No need to refetch logs - we already have them all from the websocket
        try {
            const statusResponse: ExecutionStatusResponse =
                await apiClient.getExecutionStatus(currentExecutionId);
            const status = statusResponse.status || 'UNKNOWN';
            executionStatus.set(status);

            if (statusResponse.started_at) {
                startedAt.set(statusResponse.started_at);
            }

            const terminal = TERMINAL_STATES.includes(status);
            isCompleted.set(terminal);

            if (terminal) {
                cachedWebSocketURL.set(null);
            }
        } catch (error) {
            const err = error as ApiError;
            errorMessage = err.details?.error || err.message || 'Failed to fetch execution status';
        }
    }

    $: showWelcome = !isConfigured;

    $: if (!apiClient) {
        disconnectWebSocket();
        lastProcessedExecutionId = null;
    }

    onMount(() => {
        if (typeof window === 'undefined') {
            return undefined;
        }

        const unsubscribeExecution = executionId.subscribe((id) => {
            currentExecutionId = id;
            if (!apiClient) {
                return;
            }
            if (!id) {
                lastProcessedExecutionId = null;
                return;
            }
            if (id !== lastProcessedExecutionId) {
                resetForExecution(id);
                const existingWebsocketURL = get(cachedWebSocketURL);
                if (!existingWebsocketURL) {
                    fetchLogs(id);
                }
            }
        });

        const unsubscribeCachedURL = cachedWebSocketURL.subscribe((url) => {
            if (!apiClient) {
                return;
            }
            if (url) {
                connectWebSocket(url);
            } else {
                disconnectWebSocket();
            }
        });

        window.addEventListener('runvoy:execution-complete', handleExecutionComplete);

        return () => {
            unsubscribeExecution();
            unsubscribeCachedURL();
            window.removeEventListener('runvoy:execution-complete', handleExecutionComplete);
        };
    });

    onDestroy(() => {
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
    <article class="logs-card">
        <StatusBar onKill={handleKillExecution} />
        <WebSocketStatus />
        <LogControls />
        <LogViewer />
    </article>
{/if}

<style>
    article {
        margin-top: 2rem;
    }

    .logs-card {
        background: var(--pico-card-background-color);
        border: 1px solid var(--pico-card-border-color);
        border-radius: var(--pico-border-radius);
        padding: 2rem;
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

    @media (max-width: 768px) {
        article {
            margin-top: 1.5rem;
        }

        .logs-card {
            padding: 1.5rem;
        }

        .error-box {
            padding: 0.875rem 1rem;
            margin-top: 1.5rem;
        }

        code {
            font-size: 0.85em;
            padding: 0.2rem 0.4rem;
        }
    }
</style>
