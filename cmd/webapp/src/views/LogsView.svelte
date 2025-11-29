<script lang="ts">
    import { onDestroy } from 'svelte';
    import { get } from 'svelte/store';

    import ExecutionSelector from '../components/ExecutionSelector.svelte';
    import StatusBar from '../components/StatusBar.svelte';
    import WebSocketStatus from '../components/WebSocketStatus.svelte';
    import LogControls from '../components/LogControls.svelte';
    import LogViewer from '../components/LogViewer.svelte';
    import { executionId, executionStatus, startedAt, isCompleted } from '../stores/execution';
    import { logEvents } from '../stores/logs';
    import { cachedWebSocketURL, isConnected, isConnecting } from '../stores/websocket';
    import { connectWebSocket, disconnectWebSocket } from '../lib/websocket';
    import type APIClient from '../lib/api';
    import type { LogEvent } from '../types/logs';
    import type { ApiError, ExecutionStatusResponse } from '../types/api';

    interface Props {
        apiClient: APIClient | null;
    }

    const { apiClient = null }: Props = $props();

    let errorMessage = $state('');
    let currentExecutionId: string | null = $state(null);
    let lastProcessedExecutionId: string | null = $state(null);
    let isFetchingLogs = $state(false); // Guard to prevent duplicate fetchLogs calls
    const TERMINAL_STATES = ['SUCCEEDED', 'FAILED', 'STOPPED'];

    async function handleKillExecution(): Promise<void> {
        const id = get(executionId);
        if (!apiClient || !id) {
            return;
        }

        try {
            await apiClient.killExecution(id);
            // Don't fetch logs if WebSocket is connected or connecting - it will stream the updated status
            // Only fetch logs for terminal executions without WebSocket
            const wsActive = get(isConnected) || get(isConnecting);
            if (!wsActive) {
                await fetchLogs(id);
            }
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

        // Prevent duplicate calls - only one fetchLogs should be active at a time
        if (isFetchingLogs) {
            return;
        }

        isFetchingLogs = true;
        errorMessage = '';

        try {
            // Verify execution ID hasn't changed while we were waiting
            const currentId = get(executionId);
            if (currentId !== id) {
                // Execution ID changed, abort this fetch
                isFetchingLogs = false;
                return;
            }

            const response = await apiClient.getLogs(id);

            // Contract: Running executions return websocket_url and events=null.
            // Terminal executions return events array (never null) and no websocket_url.
            // One GET request should be sufficient - no need for multiple calls.

            if (response.websocket_url) {
                // Running execution: use WebSocket for streaming logs.
                // Events should be null for running executions per API contract.
                // Don't overwrite logs if WebSocket is already active to avoid race conditions.
                const wsActive = get(isConnected) || get(isConnecting);
                if (!wsActive) {
                    // WebSocket not active yet. For running executions, events should be null,
                    // so we don't set any initial logs - WebSocket will stream them.
                    // This ensures we never call fetchLogs() when WebSocket is connected.
                }
                // Set the URL so WebSocket will connect (if not already connected)
                cachedWebSocketURL.set(response.websocket_url);
                return;
            }

            // Terminal execution: events is a non-nil array (may be empty), no websocket_url.
            // Set logs directly from API response - this is the final state.
            const events = response.events ?? [];
            const eventsWithLines: LogEvent[] = events.map((event, index) => ({
                ...event,
                line: event.line ?? index + 1
            }));
            logEvents.set(eventsWithLines);

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
        } finally {
            isFetchingLogs = false;
        }
    }

    function resetForExecution(): void {
        disconnectWebSocket();
        logEvents.set([]); // Clear logs when switching executions to avoid stale data
        executionStatus.set('LOADING');
        startedAt.set(null);
        isCompleted.set(false);
        errorMessage = '';
        isFetchingLogs = false; // Reset fetch guard when switching executions
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

    $effect(() => {
        if (!apiClient) {
            disconnectWebSocket();
            lastProcessedExecutionId = null;
        }
    });

    $effect.root(() => {
        if (typeof window === 'undefined') {
            return;
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
                // Set lastProcessedExecutionId immediately to prevent duplicate calls
                // if the subscription fires multiple times in quick succession
                lastProcessedExecutionId = id;
                resetForExecution();
                // Don't fetch logs if WebSocket is connected or connecting - let it handle streaming
                const wsActive = get(isConnected) || get(isConnecting);
                const existingWebsocketURL = get(cachedWebSocketURL);
                // Set isFetchingLogs before calling fetchLogs to prevent race conditions
                // where multiple subscription fires could all pass the guard
                if (!wsActive && !existingWebsocketURL && !isFetchingLogs) {
                    isFetchingLogs = true;
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

{#if !currentExecutionId}
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
