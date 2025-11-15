<script>
    import { showMetadata, logEvents } from '../stores/logs.js';
    import { executionId } from '../stores/execution.js';
    import { disconnectWebSocket, connectWebSocket } from '../lib/websocket.js';
    import { cachedWebSocketURL } from '../stores/websocket.js';
    import { formatTimestamp } from '../lib/ansi.js';

    let isPaused = false;

    function toggleMetadata() {
        showMetadata.update((v) => !v);
    }

    function clearLogs() {
        logEvents.set([]);
    }

    function downloadLogs() {
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

    function togglePause() {
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
    <button on:click={togglePause} class:secondary={!isPaused}>
        {isPaused ? '▶️ Resume' : '⏸️ Pause'}
    </button>
    <button on:click={clearLogs} class="secondary"> 🗑️ Clear </button>
    <button on:click={downloadLogs} class="secondary" disabled={$logEvents.length === 0}>
        📥 Download
    </button>
    <button on:click={toggleMetadata} class="secondary">
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
