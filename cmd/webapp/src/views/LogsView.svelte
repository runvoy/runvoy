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

<article class="logs-card">
    <ExecutionSelector
        executionId={currentExecutionId}
        onExecutionChange={handleExecutionChange}
        embedded
    />

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
        <div class="info-box">
            <p>
                Enter an execution ID above or provide <code>?execution_id=&lt;id&gt;</code> in the URL
            </p>
        </div>
    {:else if !$logsError && $metadata}
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
        <WebSocketStatus
            isConnecting={$connection === 'connecting'}
            isConnected={$connection === 'connected'}
            connectionError={$logsError}
            isCompleted={$phase === 'completed'}
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
    {:else if !$logsError}
        <div class="info-box">
            <p>Loading execution metadata...</p>
        </div>
    {/if}
</article>

<style>
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
        border-left: 4px solid var(--pico-color-red-500);
        padding: 1rem 1.5rem;
        margin-top: 1.5rem;
        border-radius: var(--pico-border-radius);
        background-color: color-mix(in srgb, var(--pico-color-red-500) 10%, transparent);
    }

    .error-box p {
        margin: 0;
        color: var(--pico-color-red-500);
        font-weight: bold;
    }

    .info-box {
        padding: 1rem 1.5rem;
        margin-top: 1.5rem;
        border-radius: var(--pico-border-radius);
        background-color: var(--pico-secondary-background);
    }

    .info-box p {
        margin: 0;
        color: var(--pico-muted-color);
    }

    @media (max-width: 768px) {
        .logs-card {
            padding: 1.5rem;
        }

        .error-box,
        .info-box {
            padding: 0.875rem 1rem;
            margin-top: 1rem;
        }

        code {
            font-size: 0.85em;
            padding: 0.2rem 0.4rem;
        }
    }
</style>
