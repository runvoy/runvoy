<script lang="ts">
    import { onDestroy } from 'svelte';
    import { goto } from '$app/navigation';

    import ExecutionSelector from '../components/ExecutionSelector.svelte';
    import StatusBar from '../components/StatusBar.svelte';
    import WebSocketStatus from '../components/WebSocketStatus.svelte';
    import LogControls from '../components/LogControls.svelte';
    import LogViewer from '../components/LogViewer.svelte';
    import { clearCaches as clearLogLineCaches } from '../components/LogLine.svelte';
    import { executionId } from '../stores/execution';
    import {
        cachedWebSocketURL,
        isConnected,
        isConnecting,
        connectionError
    } from '../stores/websocket';
    import {
        connectWebSocket,
        disconnectWebSocket,
        type WebSocketCallbacks
    } from '../lib/websocket';
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
    let showMetadata = $state(true);
    let events = $state<LogEvent[]>([]);
    let status = $state<string | null>(null);
    let executionStartedAt = $state<string | null>(null);
    let executionCompletedAt = $state<string | null>(null);
    let executionExitCode = $state<number | null>(null);
    let completed = $state(false);
    let killInitiated = $state(false);

    // Track the execution we last loaded to avoid stale responses
    let latestExecutionId: string | null = null;

    // Backend handles incremental log delivery, so we just append events directly
    function addLogEvent(event: LogEvent): void {
        // Compute line number from current events length (1-indexed)
        const lineNumber = events.length + 1;
        events.push({
            ...event,
            line: lineNumber
        });
    }

    const websocketCallbacks: WebSocketCallbacks = {
        onLogEvent: (event: LogEvent) => {
            addLogEvent(event);
        },
        onExecutionComplete: () => {
            completed = true;
            handleExecutionComplete();
        },
        onStatusRunning: () => {
            if (status === ExecutionStatus.STARTING || status === FrontendStatus.LOADING) {
                status = ExecutionStatus.RUNNING;
            }
        },
        onError: (error: string) => {
            errorMessage = error;
        }
    };

    async function handleKillExecution(): Promise<void> {
        if (!apiClient || !currentExecutionId) {
            return;
        }

        try {
            await apiClient.killExecution(currentExecutionId);
            // Kill request succeeded - update status and disable the button
            status = ExecutionStatus.TERMINATING;
            killInitiated = true;
            // If WebSocket is not active, refresh to get final status
            if (!$isConnected && !$isConnecting) {
                await fetchLogs(currentExecutionId);
            }
        } catch (error) {
            const err = error as ApiError;
            errorMessage = err.details?.error || err.message || 'Failed to stop execution';
            // On error, don't disable the button so user can retry
        }
    }

    function deriveStartedAtFromLogs(logEvents: LogEvent[] = []): string | null {
        if (!Array.isArray(logEvents) || logEvents.length === 0) {
            return null;
        }

        const timestamps = logEvents
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
            const isStale = latestExecutionId !== id;
            if (isStale) return;

            // Contract: Running executions return websocket_url and events=null.
            // Terminal executions return events array (never null) and no websocket_url.
            if (response.websocket_url) {
                // Running execution: use WebSocket for streaming logs
                status = response.status || ExecutionStatus.RUNNING;
                cachedWebSocketURL.set(response.websocket_url);
                return;
            }

            // Terminal execution: set logs from API response
            const responseEvents = response.events ?? [];
            const eventsWithLines: LogEvent[] = responseEvents.map((event, index) => ({
                ...event,
                line: index + 1
            }));
            events = eventsWithLines;

            if (!response.status) {
                errorMessage = 'Invalid API response: missing execution status';
                events = [];
                return;
            }

            status = response.status;
            const derivedStartedAt = deriveStartedAtFromLogs(eventsWithLines);
            executionStartedAt = derivedStartedAt;

            const terminal = isTerminalStatus(status);
            completed = terminal;

            // For terminal executions, fetch full status to get completed_at and exit_code
            if (terminal && apiClient) {
                try {
                    const statusResponse: ExecutionStatusResponse =
                        await apiClient.getExecutionStatus(id);
                    if (statusResponse.completed_at) {
                        executionCompletedAt = statusResponse.completed_at;
                    }
                    if (statusResponse.exit_code !== undefined) {
                        executionExitCode = statusResponse.exit_code;
                    }
                } catch {
                    // Non-fatal: we already have the status and logs, just missing metadata
                    // Don't set errorMessage as this is not critical
                }
            }
        } catch (error) {
            // Ignore if this fetch is stale
            const isStale = latestExecutionId !== id;
            if (isStale) return;
            const err = error as ApiError;
            errorMessage = err.details?.error || err.message || 'Failed to fetch logs';
            events = [];
        }
    }

    function resetState(): void {
        disconnectWebSocket();
        events = [];
        status = FrontendStatus.LOADING;
        executionStartedAt = null;
        executionCompletedAt = null;
        executionExitCode = null;
        completed = false;
        killInitiated = false;
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

            status = statusResponse.status;

            if (statusResponse.started_at) {
                executionStartedAt = statusResponse.started_at;
            }

            if (statusResponse.completed_at) {
                executionCompletedAt = statusResponse.completed_at;
            }

            if (statusResponse.exit_code !== undefined) {
                executionExitCode = statusResponse.exit_code;
            }

            const terminal = isTerminalStatus(status);
            completed = terminal;

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

        executionId.set(id);

        if (!apiClient) {
            return;
        }

        if (!id) {
            resetState();
            latestExecutionId = null;
            return;
        }

        const isNewExecution = latestExecutionId !== id;
        if (!isNewExecution) {
            return;
        }

        latestExecutionId = id;
        resetState();

        if ($isConnected || $isConnecting) {
            return;
        }

        fetchLogs(id);
    });

    // Handle WebSocket URL changes
    $effect(() => {
        const url = $cachedWebSocketURL;
        if (!apiClient) {
            return;
        }
        if (url) {
            connectWebSocket(url, websocketCallbacks);
        }
    });

    onDestroy(() => {
        disconnectWebSocket();
    });

    function handleToggleMetadata(): void {
        showMetadata = !showMetadata;
    }

    function handleClearLogs(): void {
        events = [];
        clearLogLineCaches();
    }

    function handlePause(): void {
        disconnectWebSocket();
    }

    function handleResume(): void {
        if ($cachedWebSocketURL) {
            connectWebSocket($cachedWebSocketURL, websocketCallbacks);
        }
    }
</script>

<ExecutionSelector executionId={currentExecutionId} onExecutionChange={handleExecutionChange} />

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
        <StatusBar
            {status}
            startedAt={executionStartedAt}
            completedAt={executionCompletedAt}
            exitCode={executionExitCode}
            {killInitiated}
            onKill={handleKillExecution}
        />
        <WebSocketStatus
            isConnecting={$isConnecting}
            isConnected={$isConnected}
            connectionError={$connectionError}
            isCompleted={completed}
        />
        <LogControls
            executionId={currentExecutionId}
            {events}
            {showMetadata}
            onToggleMetadata={handleToggleMetadata}
            onClear={handleClearLogs}
            onPause={handlePause}
            onResume={handleResume}
        />
        <LogViewer {events} {showMetadata} />
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
