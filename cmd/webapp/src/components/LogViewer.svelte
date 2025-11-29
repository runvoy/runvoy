<script lang="ts">
    import { logEvents } from '../stores/logs';
    import LogLine from './LogLine.svelte';

    let container: HTMLDivElement | undefined;
    let autoScroll = $state(true);

    function handleScroll(): void {
        if (!container) return;
        // A little tolerance for scroll position
        const isScrolledToBottom =
            container.scrollHeight - container.clientHeight <= container.scrollTop + 5;
        autoScroll = isScrolledToBottom;
    }

    $effect(() => {
        // Auto-scroll to bottom when new logs arrive
        // Access $logEvents to track changes
        $logEvents;
        if (autoScroll && container) {
            container.scrollTop = container.scrollHeight;
        }
    });
</script>

<div class="log-viewer-container" bind:this={container} onscroll={handleScroll}>
    {#if $logEvents.length > 0}
        <div class="log-lines">
            {#each $logEvents as event (event.event_id)}
                <LogLine {event} />
            {/each}
        </div>
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

    .log-lines {
        margin: 0;
        padding: 0;
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
