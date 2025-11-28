<script lang="ts">
    import { showMetadata, logEvents } from '../stores/logs';
    import { executionId } from '../stores/execution';
    import { disconnectWebSocket, connectWebSocket } from '../lib/websocket';
    import { cachedWebSocketURL } from '../stores/websocket';
    import { formatTimestamp } from '../lib/ansi';

    let isPaused = $state(false);

    function toggleMetadata(): void {
        showMetadata.update((v) => !v);
    }

    function clearLogs(): void {
        logEvents.set([]);
    }

    function downloadLogs(): void {
        const content = $logEvents
            .map((e) => `[${formatTimestamp(e.timestamp)}] ${e.message}`)
            .join('\n');
        const blob = new Blob([content], { type: 'text/plain;charset=utf-8' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `runvoy-logs-${$executionId}.txt`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
    }

    function togglePause(): void {
        isPaused = !isPaused;
        if (isPaused) {
            disconnectWebSocket();
        } else {
            if ($cachedWebSocketURL) {
                connectWebSocket($cachedWebSocketURL);
            }
        }
    }
</script>

<div class="log-controls">
    <button onclick={togglePause} class:secondary={!isPaused}>
        {isPaused ? '▶️ Resume' : '⏸️ Pause'}
    </button>
    <button onclick={clearLogs} class="secondary"> 🗑️ Clear </button>
    <button onclick={downloadLogs} class="secondary" disabled={$logEvents.length === 0}>
        📥 Download
    </button>
    <button onclick={toggleMetadata} class="secondary">
        {$showMetadata ? '🙈 Hide' : '🙉 Show'} Metadata
    </button>
</div>

<style>
    .log-controls {
        display: flex;
        gap: 0.75rem;
        margin-bottom: 1rem;
        flex-wrap: wrap;
    }

    @media (max-width: 768px) {
        .log-controls {
            gap: 0.5rem;
        }

        .log-controls button {
            flex: 1 1 auto;
            min-width: calc(50% - 0.25rem);
            font-size: 0.875rem;
        }
    }
</style>
