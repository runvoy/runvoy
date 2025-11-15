<script>
    import { logEvents } from '../stores/logs.js';
    import LogLine from './LogLine.svelte';
    import { afterUpdate } from 'svelte';

    let container;
    let autoScroll = true;

    function handleScroll() {
        if (!container) return;
        // A little tolerance for scroll position
        const isScrolledToBottom =
            container.scrollHeight - container.clientHeight <= container.scrollTop + 5;
        autoScroll = isScrolledToBottom;
    }

    afterUpdate(() => {
        // Auto-scroll to bottom when new logs arrive
        if (autoScroll && container) {
            container.scrollTop = container.scrollHeight;
        }
    });
</script>

<div class="log-viewer-container" bind:this={container} on:scroll={handleScroll}>
    {#if $logEvents.length > 0}
        <pre><code>
            {#each $logEvents as event (event.line)}
                    <LogLine {event} />
                {/each}
        </code></pre>
    {:else}
        <div class="placeholder">
            <p>Waiting for logs...</p>
        </div>
    {/if}
</div>

<style>
    .log-viewer-container {
        background-color: var(--pico-code-background-color);
        border: 1px solid var(--pico-border-color);
        border-radius: var(--pico-border-radius);
        padding: 1rem;
        overflow-x: auto;
        min-height: 200px;
    }

    pre {
        margin: 0;
    }

    .placeholder {
        display: flex;
        align-items: center;
        justify-content: center;
        height: 100%;
        min-height: 180px;
        color: var(--pico-muted-color);
    }

    @media (max-width: 768px) {
        .log-viewer-container {
            padding: 0.75rem;
            min-height: 300px;
        }

        .placeholder {
            min-height: 250px;
        }
    }
</style>
