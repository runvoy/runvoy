<script lang="ts">
    import { onDestroy } from 'svelte';
    import { goto } from '$app/navigation';

    import ExecutionSelector from '../components/ExecutionSelector.svelte';
    import StatusBar from '../components/StatusBar.svelte';
    import WebSocketStatus from '../components/WebSocketStatus.svelte';
    import LogControls from '../components/LogControls.svelte';
    import LogViewer from '../components/LogViewer.svelte';
    import { executionId, executionStatus, startedAt, isCompleted } from '../stores/execution';
    import { logEvents } from '../stores/logs';
    import { cachedWebSocketURL, isConnected, isConnecting } from '../stores/websocket';
    import { connectWebSocket, disconnectWebSocket } from '../lib/websocket';
    import { ExecutionStatus, FrontendStatus, isTerminalStatus } from '../lib/constants';
    import type APIClient from '../lib/api';
    import type { LogEvent } from '../types/logs';
    import type { ApiError, ExecutionStatusResponse } from '../types/api';

    interface Props {
        apiClient: APIClient | null;
        currentExecutionId: string | null;
    }

    const { apiClient = null, currentExecutionId = null }: Props = $props();

    let errorMessage = $state('');

    // Track in-flight fetch to prevent duplicates
    let currentFetchId: string | null = null;

    async function handleKillExecution(): Promise<void> {
        if (!apiClient || !currentExecutionId) {
            return;
        }

        try {
            await apiClient.killExecution(currentExecutionId);
            // If WebSocket is not active, refresh to get final status
            if (!$isConnected && !$isConnecting) {
                await fetchLogs(currentExecutionId);
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

        errorMessage = '';

        try {
            const response = await apiClient.getLogs(id);

            // Verify this fetch is still relevant (execution ID hasn't changed)
            if (currentFetchId !== id) {
                return;
            }

            // Contract: Running executions return websocket_url and events=null.
            // Terminal executions return events array (never null) and no websocket_url.
            if (response.websocket_url) {
                // Running execution: use WebSocket for streaming logs
                const status = response.status || ExecutionStatus.RUNNING;
                executionStatus.set(status);
                cachedWebSocketURL.set(response.websocket_url);
                return;
            }

            // Terminal execution: set logs from API response
            const events = response.events ?? [];
            const eventsWithLines: LogEvent[] = events.map((event, index) => ({
                ...event,
                line: event.line ?? index + 1
            }));
            logEvents.set(eventsWithLines);

            if (!response.status) {
                errorMessage = 'Invalid API response: missing execution status';
                logEvents.set([]);
                return;
            }

            const status = response.status;
            executionStatus.set(status);
            const derivedStartedAt = deriveStartedAtFromLogs(eventsWithLines);
            startedAt.set(derivedStartedAt);

            const terminal = isTerminalStatus(status);
            isCompleted.set(terminal);
        } catch (error) {
            // Ignore if this fetch is stale
            if (currentFetchId !== id) {
                return;
            }
            const err = error as ApiError;
            errorMessage = err.details?.error || err.message || 'Failed to fetch logs';
            logEvents.set([]);
        }
    }

    function resetState(): void {
        disconnectWebSocket();
        logEvents.set([]);
        executionStatus.set(FrontendStatus.LOADING);
        startedAt.set(null);
        isCompleted.set(false);
        errorMessage = '';
        cachedWebSocketURL.set(null);
    }

    async function handleExecutionComplete(): Promise<void> {
        if (!apiClient || !currentExecutionId) {
            return;
        }
        // WebSocket disconnect means the task terminated
        // Fetch status to update execution status (SUCCEEDED, FAILED, STOPPED)
        try {
            const statusResponse: ExecutionStatusResponse =
                await apiClient.getExecutionStatus(currentExecutionId);

            if (!statusResponse.status) {
                errorMessage = 'Invalid API response: missing execution status';
                return;
            }

            const status = statusResponse.status;
            executionStatus.set(status);

            if (statusResponse.started_at) {
                startedAt.set(statusResponse.started_at);
            }

            const terminal = isTerminalStatus(status);
            isCompleted.set(terminal);

            if (terminal) {
                cachedWebSocketURL.set(null);
            }
        } catch (error) {
            const err = error as ApiError;
            errorMessage = err.details?.error || err.message || 'Failed to fetch execution status';
        }
    }

    function handleExecutionChange(newId: string): void {
        // Update URL, which will cause the page to re-render with new execution ID
        goto(`/logs?execution_id=${encodeURIComponent(newId)}`, { replaceState: false });
    }

    // Single effect that handles execution ID changes
    // This is the ONLY place that triggers log fetching
    $effect(() => {
        const id = currentExecutionId;

        // Sync to store for child components that read it
        executionId.set(id);

        if (!apiClient) {
            return;
        }

        if (!id) {
            // No execution ID - reset everything
            resetState();
            currentFetchId = null;
            return;
        }

        // Skip if we're already fetching this execution ID
        if (currentFetchId === id) {
            return;
        }

        // New execution ID - reset and fetch
        currentFetchId = id;
        resetState();

        // Don't fetch if WebSocket is already connected for this execution
        if ($isConnected || $isConnecting) {
            return;
        }

        // Fetch logs for this execution
        fetchLogs(id);
    });

    // Handle WebSocket URL changes
    $effect(() => {
        const url = $cachedWebSocketURL;
        if (!apiClient) {
            return;
        }
        if (url) {
            connectWebSocket(url);
        }
    });

    // Handle execution complete event
    $effect(() => {
        if (typeof window === 'undefined') {
            return;
        }

        window.addEventListener('runvoy:execution-complete', handleExecutionComplete);

        return () => {
            window.removeEventListener('runvoy:execution-complete', handleExecutionComplete);
        };
    });

    onDestroy(() => {
        disconnectWebSocket();
    });
</script>

<ExecutionSelector onExecutionChange={handleExecutionChange} />

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
{:else if !errorMessage}
    <article class="logs-card">
        <StatusBar onKill={handleKillExecution} />
        <WebSocketStatus />
        <LogControls executionId={currentExecutionId} />
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
