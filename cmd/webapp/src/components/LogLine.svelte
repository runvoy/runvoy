<script lang="ts">
    import { showMetadata } from '../stores/logs';
    import { parseAnsi, formatTimestamp, type AnsiSegment } from '../lib/ansi';
    import type { LogEvent } from '../types/stores';

    export let event: LogEvent;

    let ansiSegments: AnsiSegment[] = [];

    $: formattedTimestamp = formatTimestamp(event.timestamp);
    $: ansiSegments = parseAnsi(event.message);
</script>

<div class="log-line">
    {#if $showMetadata}
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
