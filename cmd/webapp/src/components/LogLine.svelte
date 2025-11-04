<script>
    import { showMetadata } from '../stores/logs.js';
    import { parseAnsi } from '../lib/ansi.js';

    export let event;

    $: formattedTimestamp = new Date(event.timestamp).toISOString();
    $: ansiHtml = parseAnsi(event.message);
</script>

<div class="log-line">
    {#if $showMetadata}
        <span class="line-number">{event.line}</span>
        <span class="timestamp">{formattedTimestamp}</span>
    {/if}
    <span class="message">{@html ansiHtml}</span>
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
</style>
