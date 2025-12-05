<script lang="ts" module>
    import type { AnsiSegment } from '../lib/ansi';

    // Bounded LRU cache to prevent memory leaks with large log volumes
    class LRUCache<K, V> {
        private cache = new Map<K, V>();
        private readonly maxSize: number;

        constructor(maxSize: number) {
            this.maxSize = maxSize;
        }

        get(key: K): V | undefined {
            const value = this.cache.get(key);
            if (value !== undefined) {
                // Move to end (most recently used)
                this.cache.delete(key);
                this.cache.set(key, value);
            }
            return value;
        }

        set(key: K, value: V): void {
            if (this.cache.has(key)) {
                this.cache.delete(key);
            } else if (this.cache.size >= this.maxSize) {
                // Remove oldest entry (first in map)
                const firstKey = this.cache.keys().next().value;
                if (firstKey !== undefined) {
                    this.cache.delete(firstKey);
                }
            }
            this.cache.set(key, value);
        }

        clear(): void {
            this.cache.clear();
        }
    }

    // Cache sizes tuned for ~100k logs with good hit rate
    const ansiCache = new LRUCache<string, AnsiSegment[]>(10000);
    const timestampCache = new LRUCache<number, string>(10000);

    export function clearCaches(): void {
        ansiCache.clear();
        timestampCache.clear();
    }
</script>

<script lang="ts">
    import { parseAnsi, formatTimestamp } from '../lib/ansi';
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
        height: 20px;
        overflow: hidden;
        white-space: nowrap;
    }

    .line-number,
    .timestamp {
        color: var(--pico-muted-color);
        margin-right: 1rem;
        user-select: none;
        flex-shrink: 0;
    }

    .line-number {
        text-align: right;
        min-width: 3ch;
    }

    .timestamp {
        min-width: 24ch;
    }

    .message {
        white-space: pre;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    @media (max-width: 768px) {
        .log-line {
            font-size: 0.8em;
        }

        .line-number,
        .timestamp {
            margin-right: 0.5rem;
            font-size: 0.9em;
        }

        .timestamp {
            min-width: 18ch;
        }
    }
</style>
