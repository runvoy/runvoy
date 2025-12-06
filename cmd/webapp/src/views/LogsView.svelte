<script lang="ts">
    import { onDestroy } from 'svelte';
    import { goto } from '$app/navigation';
    import { writable, type Readable } from 'svelte/store';

    import ExecutionSelector from '../components/ExecutionSelector.svelte';
    import StatusBar from '../components/StatusBar.svelte';
    import WebSocketStatus from '../components/WebSocketStatus.svelte';
    import LogControls from '../components/LogControls.svelte';
    import LogViewer from '../components/LogViewer.svelte';
    import { clearCaches as clearLogLineCaches } from '../components/LogLine.svelte';
    import { executionId as executionIdStore } from '../stores/execution';
    import {
        LogsManager,
        type ConnectionStatus,
        type ExecutionMetadata,
        type ExecutionPhase
    } from '../lib/logs';
    import { createExecutionKiller, type KillState } from '../lib/execution';
    import { isKillableStatus } from '../lib/constants';
    import type APIClient from '../lib/api';
    import type { LogEvent } from '../types/logs';

    interface Props {
        apiClient: APIClient | null;
        currentExecutionId: string | null;
    }

    const { apiClient = null, currentExecutionId = null }: Props = $props();

    // UI-only state
    let showMetadata = $state(true);

    // Create manager and killer instances (only if apiClient is available)
    const logsManager = apiClient ? new LogsManager({ apiClient }) : null;
    const killer = apiClient ? createExecutionKiller(apiClient) : null;

    // Create fallback stores for when manager/killer are null
    const emptyEvents = writable<LogEvent[]>([]);
    const nullMetadata = writable<ExecutionMetadata | null>(null);
    const disconnectedConnection = writable<ConnectionStatus>('disconnected');
    const idlePhase = writable<ExecutionPhase>('idle');
    const nullError = writable<string | null>(null);
    const defaultKillState = writable<KillState>({
        isKilling: false,
        killInitiated: false,
        error: null
    });

    // Get stores with fallbacks
    const events: Readable<LogEvent[]> = logsManager?.stores.events ?? emptyEvents;
    const metadata: Readable<ExecutionMetadata | null> =
        logsManager?.stores.metadata ?? nullMetadata;
    const connection: Readable<ConnectionStatus> =
        logsManager?.stores.connection ?? disconnectedConnection;
    const phase: Readable<ExecutionPhase> = logsManager?.stores.phase ?? idlePhase;
    const logsError: Readable<string | null> = logsManager?.stores.error ?? nullError;
    const killState: Readable<KillState> = killer?.state ?? defaultKillState;

    // Single effect that handles execution ID changes
    $effect(() => {
        const id = currentExecutionId;

        // Sync to store for child components that read it
        executionIdStore.set(id);

        if (!logsManager) {
            return;
        }

        if (!id) {
            // No execution ID - reset everything
            logsManager.reset();
            killer?.reset();
            return;
        }

        // Load the execution (manager handles deduplication internally)
        logsManager.loadExecution(id);
        killer?.reset();
    });

    onDestroy(() => {
        logsManager?.destroy();
    });

    async function handleKill(): Promise<void> {
        if (!killer || !currentExecutionId || !logsManager) return;

        const success = await killer.kill(currentExecutionId);
        if (success) {
            // Update logs manager metadata to reflect TERMINATING status
            logsManager.setStatus('TERMINATING');

            // If not streaming, refresh to get updated status
            if ($connection === 'disconnected') {
                logsManager.loadExecution(currentExecutionId);
            }
        }
    }

    function handleExecutionChange(newId: string): void {
        // Update URL, which will cause the page to re-render with new execution ID
        goto(`/logs?execution_id=${encodeURIComponent(newId)}`, { replaceState: false });
    }

    function handleToggleMetadata(): void {
        showMetadata = !showMetadata;
    }

    function handleClearLogs(): void {
        logsManager?.clearLogs();
        clearLogLineCaches();
    }

    function handlePause(): void {
        logsManager?.pause();
    }

    function handleResume(): void {
        logsManager?.resume();
    }

    // Computed values for template
    const canKill = $derived(
        isKillableStatus($metadata?.status ?? null) &&
            !$killState.isKilling &&
            !$killState.killInitiated
    );
</script>

<ExecutionSelector executionId={currentExecutionId} onExecutionChange={handleExecutionChange} />

{#if $logsError}
    <article class="error-box">
        <p>{$logsError}</p>
    </article>
{/if}

{#if $killState.error}
    <article class="error-box">
        <p>{$killState.error}</p>
    </article>
{/if}

{#if !currentExecutionId}
    <article>
        <p>
            Enter an execution ID above or provide <code>?execution_id=&lt;id&gt;</code> in the URL
        </p>
    </article>
{:else if !$logsError}
    <article class="logs-card">
        <StatusBar
            status={$metadata?.status ?? null}
            startedAt={$metadata?.startedAt ?? null}
            completedAt={$metadata?.completedAt ?? null}
            exitCode={$metadata?.exitCode ?? null}
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
