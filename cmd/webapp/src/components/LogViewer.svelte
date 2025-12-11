<script lang="ts">
    import { onDestroy, onMount, tick } from 'svelte';

    import type { LogEvent } from '../types/logs';
    import LogLine from './LogLine.svelte';

    interface Props {
        events: LogEvent[];
        showMetadata: boolean;
    }

    const { events = [], showMetadata = true }: Props = $props();

    const OVERSCAN = 10;
    const ROW_HEIGHT = 20;
    const SCROLL_LOCK_THRESHOLD = 50;
    const SCROLL_THROTTLE_MS = 16; // ~60fps

    let containerEl: HTMLDivElement | null = null;
    let viewportHeight = $state(0);
    let scrollTop = $state(0);
    let autoScroll = $state(true);
    let resizeObserver: ResizeObserver | null = null;
    let scrollRafId: number | null = null;
    let lastScrollTime = 0;

    // Compute visible range
    const totalHeight = $derived(events.length * ROW_HEIGHT);
    const startIndex = $derived(Math.max(0, Math.floor(scrollTop / ROW_HEIGHT) - OVERSCAN));
    const endIndex = $derived(
        Math.min(events.length, Math.ceil((scrollTop + viewportHeight) / ROW_HEIGHT) + OVERSCAN)
    );
    const offsetY = $derived(startIndex * ROW_HEIGHT);

    // Track previous events length to detect new logs
    let prevEventsLength = 0;

    function handleScroll(event: Event): void {
        const now = performance.now();
        if (now - lastScrollTime < SCROLL_THROTTLE_MS) {
            // Schedule update for next frame if not already scheduled
            if (!scrollRafId) {
                scrollRafId = requestAnimationFrame(() => {
                    scrollRafId = null;
                    processScroll(event);
                });
            }
            return;
        }
        processScroll(event);
    }

    function processScroll(event: Event): void {
        lastScrollTime = performance.now();
        const target = event.currentTarget as HTMLElement | null;
        if (!target) return;

        scrollTop = target.scrollTop;
        viewportHeight = target.clientHeight;

        // Check if user scrolled away from bottom
        const distanceFromBottom = target.scrollHeight - (target.scrollTop + target.clientHeight);
        autoScroll = distanceFromBottom <= SCROLL_LOCK_THRESHOLD;
    }

    function scrollToBottom(): void {
        if (!containerEl) return;
        containerEl.scrollTop = containerEl.scrollHeight;
    }

    onMount(() => {
        if (!containerEl) return;

        viewportHeight = containerEl.clientHeight;
        prevEventsLength = events.length;

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
        if (scrollRafId) {
            cancelAnimationFrame(scrollRafId);
        }
    });

    // Reset scroll when events are cleared
    $effect(() => {
        if (events.length === 0 && containerEl) {
            containerEl.scrollTop = 0;
            autoScroll = true;
            scrollTop = 0;
            prevEventsLength = 0;
        }
    });

    // Auto-scroll when new logs arrive (only if autoScroll enabled)
    $effect(() => {
        const currentLength = events.length;
        if (currentLength <= prevEventsLength) {
            prevEventsLength = currentLength;
            return;
        }

        prevEventsLength = currentLength;

        if (!autoScroll || !containerEl) return;

        // Use tick to wait for DOM update, then scroll
        tick().then(() => {
            if (containerEl && autoScroll) {
                scrollToBottom();
            }
        });
    });
</script>

<div class="log-viewer-container" bind:this={containerEl} onscroll={handleScroll}>
    {#if events.length > 0}
        <div class="virtualizer" style="height: {totalHeight}px;">
            <div class="log-lines" style="transform: translateY({offsetY}px);">
                {#each { length: endIndex - startIndex } as _, i (events[startIndex + i]?.event_id ?? i)}
                    {@const event = events[startIndex + i]}
                    {#if event}
                        <div class="log-row">
                            <LogLine {event} {showMetadata} />
                        </div>
                    {/if}
                {/each}
            </div>
        </div>
    {:else}
        <div class="placeholder">
            <p>Waiting for logs...</p>
        </div>
    {/if}
</div>

{#if !autoScroll && events.length > 0}
    <button class="scroll-to-bottom" onclick={scrollToBottom} type="button">
        ↓ Scroll to bottom
    </button>
{/if}

<style>
    .log-viewer-container {
        background-color: #0d1117;
        padding: 0.5rem 0.75rem;
        overflow: auto;
        flex: 1;
        min-height: 200px;
        position: relative;
        contain: strict;
    }

    .virtualizer {
        position: relative;
        width: 100%;
        contain: layout style;
    }

    .log-lines {
        margin: 0;
        padding: 0;
        position: absolute;
        top: 0;
        left: 0;
        right: 0;
        will-change: transform;
        contain: layout style;
    }

    .log-row {
        margin: 0;
        padding: 0;
        height: 20px;
        contain: layout style paint;
    }

    .placeholder {
        display: flex;
        align-items: center;
        justify-content: center;
        height: 100%;
        min-height: 180px;
        color: #8b949e;
    }

    .scroll-to-bottom {
        position: fixed;
        bottom: 1.5rem;
        right: 1.5rem;
        padding: 0.375rem 0.75rem;
        font-size: 0.75rem;
        background: var(--pico-primary);
        color: var(--pico-primary-inverse);
        border: none;
        border-radius: var(--pico-border-radius);
        cursor: pointer;
        z-index: 10;
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
    }

    .scroll-to-bottom:hover {
        opacity: 0.9;
    }

    @media (max-width: 768px) {
        .log-viewer-container {
            padding: 0.5rem;
            min-height: 250px;
        }

        .placeholder {
            min-height: 200px;
        }

        .scroll-to-bottom {
            bottom: 1rem;
            right: 1rem;
        }
    }
</style>
