<script lang="ts">
    import { onDestroy, untrack } from 'svelte';
    import { goto } from '$app/navigation';

    import ExecutionSelector from '../components/ExecutionSelector.svelte';
    import StatusBar from '../components/StatusBar.svelte';
    import WebSocketStatus from '../components/WebSocketStatus.svelte';
    import LogControls from '../components/LogControls.svelte';
    import LogViewer from '../components/LogViewer.svelte';
    import { clearCaches as clearLogLineCaches } from '../components/LogLine.svelte';
    import { executionId as executionIdStore } from '../stores/execution';
    import { LogsManager } from '../lib/logs';
    import { createExecutionKiller } from '../lib/execution';
    import { ExecutionStatus, isKillableStatus } from '../lib/constants';
    import type APIClient from '../lib/api';

    interface Props {
        apiClient: APIClient;
        currentExecutionId: string | null;
    }

    const { apiClient, currentExecutionId = null }: Props = $props();

    // UI-only state
    let showMetadata = $state(true);

    // Create manager and killer instances (apiClient is stable, capture initial value)
    const logsManager = new LogsManager({ apiClient: untrack(() => apiClient) });
    const killer = createExecutionKiller(untrack(() => apiClient));

    // Destructure stores
    const { events, metadata, connection, phase, error: logsError } = logsManager.stores;
    const killState = killer.state;

    // Handle execution ID changes
    $effect(() => {
        const id = currentExecutionId;

        // Only update the store when we actually have an execution ID.
        // Leaving the previous value intact allows it to survive navigation
        // between routes until an explicit ID is provided again.
        if (id) {
            executionIdStore.set(id);
            logsManager.loadExecution(id);
            killer.reset();
            return;
        }

        logsManager.reset();
        killer.reset();
    });

    onDestroy(() => {
        logsManager.destroy();
    });

    async function handleKill(): Promise<void> {
        if (!currentExecutionId) return;

        const success = await killer.kill(currentExecutionId);
        if (success) {
            logsManager.setStatus(ExecutionStatus.TERMINATING);

            // If not streaming, refresh to get updated status
            if ($connection === 'disconnected') {
                logsManager.loadExecution(currentExecutionId);
            }
        }
    }

    function handleExecutionChange(newId: string): void {
        goto(`/logs?execution_id=${encodeURIComponent(newId)}`, { replaceState: false });
    }

    function handleToggleMetadata(): void {
        showMetadata = !showMetadata;
    }

    function handleClearLogs(): void {
        logsManager.clearLogs();
        clearLogLineCaches();
    }

    function handlePause(): void {
        logsManager.pause();
    }

    function handleResume(): void {
        logsManager.resume();
    }

    const canKill = $derived(
        isKillableStatus($metadata?.status ?? null) &&
            !$killState.isKilling &&
            !$killState.killInitiated
    );
</script>

<div class="logs-container">
    {#if $logsError}
        <div class="error-box">
            <p>{$logsError}</p>
        </div>
    {/if}

    {#if $killState.error}
        <div class="error-box">
            <p>{$killState.error}</p>
        </div>
    {/if}

    {#if !currentExecutionId}
        <div class="empty-state">
            <ExecutionSelector
                executionId={currentExecutionId}
                onExecutionChange={handleExecutionChange}
                embedded
            />
            <div class="info-box">
                <p>
                    Enter an execution ID above or provide <code>?execution_id=&lt;id&gt;</code> in the
                    URL
                </p>
            </div>
        </div>
    {:else if !$logsError && $metadata}
        <div class="logs-panel">
            <div class="panel-header">
                <div class="header-left">
                    <ExecutionSelector
                        executionId={currentExecutionId}
                        onExecutionChange={handleExecutionChange}
                        embedded
                    />
                    <WebSocketStatus
                        isConnecting={$connection === 'connecting'}
                        isConnected={$connection === 'connected'}
                        connectionError={$logsError}
                        isCompleted={$phase === 'completed'}
                    />
                </div>
            </div>
            <StatusBar
                status={$metadata?.status ?? null}
                startedAt={$metadata?.startedAt ?? null}
                completedAt={$metadata?.completedAt ?? null}
                exitCode={$metadata?.exitCode ?? null}
                command={$metadata.command}
                imageId={$metadata.imageId}
                killInitiated={$killState.killInitiated}
                onKill={canKill ? handleKill : null}
            />
            <LogControls
                executionId={currentExecutionId}
                events={$events}
                {showMetadata}
                onToggleMetadata={handleToggleMetadata}
                onClear={handleClearLogs}
                onPause={handlePause}
                onResume={handleResume}
            />
            <LogViewer events={$events} {showMetadata} />
        </div>
    {:else if !$logsError}
        <div class="info-box">
            <p>Loading execution metadata...</p>
        </div>
    {/if}
</div>

<style>
    .logs-container {
        display: flex;
        flex-direction: column;
        height: calc(100vh - 5rem);
        min-height: 400px;
    }

    .empty-state {
        padding: 1rem;
    }

    .logs-panel {
        display: flex;
        flex-direction: column;
        flex: 1;
        min-height: 0;
        border: 1px solid var(--pico-border-color);
        border-radius: var(--pico-border-radius);
        overflow: hidden;
    }

    .panel-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 0.75rem;
        padding: 0.5rem 0.75rem;
        background-color: var(--pico-card-background-color);
        border-bottom: 1px solid var(--pico-border-color);
    }

    .header-left {
        display: flex;
        align-items: center;
        gap: 0.75rem;
        flex: 1;
        min-width: 0;
    }

    .header-left :global(.execution-selector) {
        margin-bottom: 0;
        flex: 1;
        min-width: 0;
    }

    .header-left :global(.execution-selector label) {
        margin-bottom: 0;
    }

    .header-left :global(.execution-selector input) {
        margin-bottom: 0;
        padding: 0.25rem 0.5rem;
        font-size: 0.8125rem;
        height: auto;
    }

    code {
        background: var(--pico-code-background-color);
        padding: 0.125rem 0.375rem;
        border-radius: 0.25rem;
        font-size: 0.8125rem;
    }

    .error-box {
        border-left: 4px solid var(--pico-color-red-500);
        padding: 0.5rem 0.75rem;
        margin-bottom: 0.5rem;
        border-radius: var(--pico-border-radius);
        background-color: color-mix(in srgb, var(--pico-color-red-500) 10%, transparent);
    }

    .error-box p {
        margin: 0;
        color: var(--pico-color-red-500);
        font-weight: 600;
        font-size: 0.8125rem;
    }

    .info-box {
        padding: 0.75rem 1rem;
        border-radius: var(--pico-border-radius);
        background-color: var(--pico-secondary-background);
    }

    .info-box p {
        margin: 0;
        color: var(--pico-muted-color);
        font-size: 0.875rem;
    }

    @media (max-width: 768px) {
        .logs-container {
            height: calc(100vh - 6rem);
        }

        .panel-header {
            flex-wrap: wrap;
            padding: 0.5rem;
        }

        .header-left {
            width: 100%;
        }

        .error-box,
        .info-box {
            padding: 0.5rem 0.75rem;
        }
    }
</style>
