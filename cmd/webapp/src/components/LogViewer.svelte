<script lang="ts">
    import { onDestroy, onMount } from 'svelte';

    import type { LogEvent } from '../types/logs';
    import LogLine from './LogLine.svelte';

    interface Props {
        events: LogEvent[];
        showMetadata: boolean;
    }

    const { events = [], showMetadata = true }: Props = $props();

    const overscan = 12;
    const DEFAULT_ROW_HEIGHT = 20;
    const SCROLL_LOCK_THRESHOLD = 48;

    let containerEl: HTMLDivElement | null = null;
    let viewportHeight = $state(0);
    let scrollTop = $state(0);
    let rowHeight = $state(DEFAULT_ROW_HEIGHT);
    let autoScroll = $state(true);
    let resizeObserver: ResizeObserver | null = null;

    const totalHeight = $derived(events.length * rowHeight);
    const start = $derived(Math.max(0, Math.floor(scrollTop / rowHeight) - overscan));
    const end = $derived(
        Math.min(events.length, Math.ceil((scrollTop + viewportHeight) / rowHeight) + overscan)
    );
    const visibleEvents = $derived(events.slice(start, end));
    const offsetTop = $derived(start * rowHeight);

    function measureRow(node: HTMLElement, active: boolean): void | { destroy: () => void } {
        if (!active) {
            return;
        }

        const updateHeight = (height: number): void => {
            if (height > 0 && Math.abs(height - rowHeight) > 0.5) {
                rowHeight = height;
            }
        };

        if (typeof ResizeObserver === 'undefined') {
            updateHeight(node.getBoundingClientRect().height);
            return;
        }

        const observer = new ResizeObserver((entries) => {
            for (const entry of entries) {
                updateHeight(entry.contentRect.height);
            }
        });

        observer.observe(node);
        return {
            destroy: () => observer.disconnect()
        };
    }

    function updateAutoScroll(target: HTMLElement | null): void {
        if (!target) {
            return;
        }
        const distanceFromBottom = target.scrollHeight - (target.scrollTop + target.clientHeight);
        autoScroll = distanceFromBottom <= SCROLL_LOCK_THRESHOLD;
    }

    function handleScroll(event: Event): void {
        const target = event.currentTarget as HTMLElement | null;
        if (!target) {
            return;
        }

        scrollTop = target.scrollTop;
        viewportHeight = target.clientHeight;
        updateAutoScroll(target);
    }

    onMount(() => {
        if (!containerEl) {
            return;
        }

        viewportHeight = containerEl.clientHeight;

        if (typeof ResizeObserver !== 'undefined') {
            resizeObserver = new ResizeObserver(() => {
                if (containerEl) {
                    viewportHeight = containerEl.clientHeight;
                }
            });
            resizeObserver.observe(containerEl);
        }
    });

    onDestroy(() => {
        resizeObserver?.disconnect();
    });

    $effect(() => {
        if (events.length === 0 && containerEl) {
            containerEl.scrollTop = 0;
            autoScroll = true;
            scrollTop = 0;
        }
    });

    $effect(() => {
        if (!autoScroll || events.length === 0 || !containerEl) {
            return;
        }

        const targetTop = containerEl.scrollHeight;
        window.requestAnimationFrame(() => {
            containerEl?.scrollTo({
                top: targetTop,
                behavior: 'auto'
            });
        });
    });
</script>

<div class="log-viewer-container" bind:this={containerEl} on:scroll={handleScroll}>
    {#if events.length > 0}
        <div class="virtualizer" style={`height: ${totalHeight}px;`}>
            <div class="log-lines" style={`transform: translateY(${offsetTop}px);`}>
                {#each visibleEvents as event, index (event.event_id)}
                    <LogLine {event} {showMetadata} use:measureRow={index === 0} />
                {/each}
            </div>
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
        overflow: auto;
        min-height: 200px;
        max-height: 70vh;
        position: relative;
    }

    .virtualizer {
        position: relative;
        width: 100%;
    }

    .log-lines {
        margin: 0;
        padding: 0;
        position: absolute;
        left: 0;
        right: 0;
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
