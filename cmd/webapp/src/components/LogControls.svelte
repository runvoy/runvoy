<script lang="ts">
    import type { LogEvent } from '../types/logs';
    import { formatTimestamp } from '../lib/ansi';

    interface Props {
        executionId: string | null;
        events: LogEvent[];
        showMetadata: boolean;
        onToggleMetadata: () => void;
        onClear: () => void;
        onPause: () => void;
        onResume: () => void;
    }

    const {
        executionId,
        events = [],
        showMetadata = false,
        onToggleMetadata,
        onClear,
        onPause,
        onResume
    }: Props = $props();

    let isPaused = $state(false);

    function downloadLogs(): void {
        const content = events
            .map((e) => `[${formatTimestamp(e.timestamp)}] ${e.message}`)
            .join('\n');
        const blob = new Blob([content], { type: 'text/plain;charset=utf-8' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `runvoy-logs-${executionId || 'logs'}.txt`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
    }

    function togglePause(): void {
        isPaused = !isPaused;
        if (isPaused) {
            onPause();
        } else {
            onResume();
        }
    }
</script>

<div class="log-controls">
    <button onclick={togglePause} class:secondary={!isPaused}>
        {isPaused ? '▶️ Resume' : '⏸️ Pause'}
    </button>
    <button onclick={onClear} class="secondary"> 🗑️ Clear </button>
    <button onclick={downloadLogs} class="secondary" disabled={events.length === 0}>
        📥 Download
    </button>
    <button onclick={onToggleMetadata} class="secondary">
        {showMetadata ? '🙈 Hide' : '🙉 Show'} Metadata
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
