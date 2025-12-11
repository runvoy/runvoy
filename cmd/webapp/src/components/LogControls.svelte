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
    <span class="log-count">{events.length} lines</span>
    <div class="control-buttons">
        <button
            class="control-btn"
            onclick={togglePause}
            title={isPaused ? 'Resume streaming' : 'Pause streaming'}
        >
            {isPaused ? '▶' : '⏸'}
        </button>
        <button class="control-btn" onclick={onClear} title="Clear logs"> 🗑 </button>
        <button
            class="control-btn"
            onclick={downloadLogs}
            disabled={events.length === 0}
            title="Download logs"
        >
            📥
        </button>
        <button
            class="control-btn"
            onclick={onToggleMetadata}
            title={showMetadata ? 'Hide timestamps' : 'Show timestamps'}
        >
            {showMetadata ? '🕐' : '🕐'}
        </button>
    </div>
</div>

<style>
    .log-controls {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0.25rem 0.75rem;
        background-color: var(--pico-code-background-color);
        border-top: 1px solid rgba(255, 255, 255, 0.1);
        font-size: 0.75rem;
    }

    .log-count {
        color: var(--pico-muted-color);
        font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
    }

    .control-buttons {
        display: flex;
        gap: 0.25rem;
    }

    .control-btn {
        padding: 0.25rem 0.5rem;
        background: transparent;
        border: none;
        border-radius: 0.25rem;
        cursor: pointer;
        font-size: 0.875rem;
        line-height: 1;
        color: var(--pico-muted-color);
        transition: background-color 0.15s ease;
    }

    .control-btn:hover:not(:disabled) {
        background-color: rgba(255, 255, 255, 0.1);
        color: var(--pico-color);
    }

    .control-btn:disabled {
        opacity: 0.4;
        cursor: not-allowed;
    }

    @media (max-width: 768px) {
        .log-controls {
            padding: 0.25rem 0.5rem;
        }
    }
</style>
