<script lang="ts" context="module">
    import type { AnsiSegment } from '../lib/ansi';

    const ansiCache = new Map<string, AnsiSegment[]>();
    const timestampCache = new Map<number, string>();
</script>

<script lang="ts">
    import { parseAnsi, formatTimestamp, type AnsiSegment } from '../lib/ansi';
    import type { LogEvent } from '../types/logs';

    interface Props {
        event: LogEvent;
        showMetadata: boolean;
    }

    const { event, showMetadata = true }: Props = $props();

    function getAnsiSegments(logEvent: LogEvent): AnsiSegment[] {
        const cached = ansiCache.get(logEvent.event_id);
        if (cached) {
            return cached;
        }

        const parsed = parseAnsi(logEvent.message);
        ansiCache.set(logEvent.event_id, parsed);
        return parsed;
    }

    function getFormattedTimestamp(logEvent: LogEvent): string {
        const cached = timestampCache.get(logEvent.timestamp);
        if (cached) {
            return cached;
        }

        const formatted = formatTimestamp(logEvent.timestamp);
        timestampCache.set(logEvent.timestamp, formatted);
        return formatted;
    }

    const formattedTimestamp = $derived(getFormattedTimestamp(event));
    const ansiSegments: AnsiSegment[] = $derived(getAnsiSegments(event));
</script>

<div class="log-line">
    {#if showMetadata}
        <span class="line-number">{event.line}</span>
        <span class="timestamp">{formattedTimestamp}</span>
    {/if}
    <span class="message">
        {#each ansiSegments as segment, index (index)}
            {#if segment.className}
                <span class={segment.className}>{segment.text}</span>
            {:else}
                {segment.text}
            {/if}
        {/each}
    </span>
</div>

<style>
    .log-line {
        display: flex;
        font-family: 'Monaco', 'Courier New', monospace;
        font-size: 0.9em;
        margin: 0;
        padding: 0;
        line-height: 1.4;
    }

    .line-number,
    .timestamp {
        color: var(--pico-muted-color);
        margin-right: 1rem;
        user-select: none;
    }

    .line-number {
        text-align: right;
        min-width: 3ch;
    }

    .timestamp {
        min-width: 24ch;
    }

    .message {
        white-space: pre-wrap;
        word-break: break-all;
    }

    @media (max-width: 768px) {
        .log-line {
            font-size: 0.8em;
            flex-wrap: wrap;
        }

        .line-number,
        .timestamp {
            margin-right: 0.5rem;
            font-size: 0.9em;
        }

        .timestamp {
            min-width: 18ch;
        }

        .message {
            width: 100%;
            margin-top: 0.25rem;
        }

        .line-number:first-child + .timestamp + .message {
            margin-top: 0;
        }
    }
</style>
