<script lang="ts">
    import { FrontendStatus, isKillableStatus } from '../lib/constants';
    import type { ExecutionStatusValue } from '../types/status';

    interface Props {
        status: ExecutionStatusValue | null;
        startedAt: string | number | null;
        completedAt: string | null;
        exitCode: number | null;
        killInitiated?: boolean;
        onKill?: (() => void) | null;
        command: string;
        imageId: string;
    }

    const {
        status = null,
        startedAt = null,
        completedAt = null,
        exitCode = null,
        killInitiated = false,
        onKill = null,
        command,
        imageId
    }: Props = $props();

    let isKilling = $state(false);

    const statusClass = $derived(status ? status.toLowerCase() : 'loading');

    const duration = $derived.by(() => {
        if (!startedAt) return null;

        const startMs = typeof startedAt === 'number' ? startedAt : Date.parse(startedAt);
        if (Number.isNaN(startMs)) return null;

        const endMs = completedAt ? Date.parse(completedAt) : Date.now();
        if (Number.isNaN(endMs)) return null;

        const diffMs = endMs - startMs;
        const seconds = Math.floor(diffMs / 1000);
        if (seconds < 60) return `${seconds}s`;
        const minutes = Math.floor(seconds / 60);
        const remainingSeconds = seconds % 60;
        if (minutes < 60) return `${minutes}m ${remainingSeconds}s`;
        const hours = Math.floor(minutes / 60);
        const remainingMinutes = minutes % 60;
        return `${hours}h ${remainingMinutes}m`;
    });

    const formattedStartedAt = $derived.by(() => {
        if (!startedAt) return null;
        const dateValue = typeof startedAt === 'number' ? startedAt : Date.parse(startedAt);
        const date = new Date(dateValue);
        if (Number.isNaN(date.getTime())) return null;
        return date.toLocaleString();
    });

    async function handleKill(): Promise<void> {
        if (!onKill) return;
        isKilling = true;
        try {
            await onKill();
        } finally {
            isKilling = false;
        }
    }

    const canKill = $derived(isKillableStatus(status) && !isKilling && !killInitiated);
</script>

<div class="status-bar">
    <div class="status-left">
        <span class="status-badge {statusClass}">{status || FrontendStatus.LOADING}</span>
        <code class="command" title={command}>{command}</code>
    </div>
    <div class="status-right">
        {#if formattedStartedAt}
            <span class="meta" title="Started at {formattedStartedAt}">
                {formattedStartedAt}
            </span>
        {/if}
        {#if duration}
            <span class="meta duration">{duration}</span>
        {/if}
        {#if exitCode !== null}
            <code class="exit-code" class:error={exitCode !== 0}>exit: {exitCode}</code>
        {/if}
        <span class="meta image" title={imageId}>{imageId}</span>
        {#if onKill && isKillableStatus(status)}
            <button
                class="kill-button"
                onclick={handleKill}
                disabled={!canKill}
                title="Kill this execution"
            >
                {isKilling ? '⏹' : '⏹'}
            </button>
        {/if}
    </div>
</div>

<style>
    .status-bar {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 0.75rem;
        padding: 0.5rem 0.75rem;
        background-color: var(--pico-code-background-color);
        border-radius: var(--pico-border-radius) var(--pico-border-radius) 0 0;
        font-size: 0.8125rem;
        min-height: 2.5rem;
    }

    .status-left {
        display: flex;
        align-items: center;
        gap: 0.625rem;
        min-width: 0;
        flex: 1;
    }

    .status-right {
        display: flex;
        align-items: center;
        gap: 0.75rem;
        flex-shrink: 0;
    }

    .status-badge {
        padding: 0.125rem 0.5rem;
        border-radius: 0.75rem;
        font-weight: 600;
        font-size: 0.6875rem;
        text-transform: uppercase;
        color: #fff;
        white-space: nowrap;
    }

    .status-badge.loading {
        background-color: #78909c;
    }
    .status-badge.starting {
        background-color: #f39c12;
        color: #000;
    }
    .status-badge.running {
        background-color: #2196f3;
    }
    .status-badge.succeeded {
        background-color: #4caf50;
    }
    .status-badge.failed {
        background-color: #f44336;
    }
    .status-badge.stopped {
        background-color: #ff9800;
        color: #000;
    }
    .status-badge.terminating {
        background-color: #9c27b0;
    }

    .command {
        font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
        font-size: 0.8125rem;
        color: var(--pico-color);
        background: none;
        padding: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        max-width: 40ch;
    }

    .meta {
        color: var(--pico-muted-color);
        white-space: nowrap;
        font-size: 0.75rem;
    }

    .meta.duration {
        font-weight: 500;
        color: var(--pico-color);
    }

    .meta.image {
        font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
        max-width: 16ch;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .exit-code {
        font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
        font-size: 0.6875rem;
        padding: 0.125rem 0.375rem;
        background-color: rgba(76, 175, 80, 0.2);
        color: #4caf50;
        border-radius: 0.25rem;
    }

    .exit-code.error {
        background-color: rgba(244, 67, 54, 0.2);
        color: #f44336;
    }

    .kill-button {
        padding: 0.25rem 0.5rem;
        background-color: #f44336;
        color: white;
        border: none;
        border-radius: 0.25rem;
        cursor: pointer;
        font-size: 0.75rem;
        line-height: 1;
        transition: background-color 0.15s ease;
    }

    .kill-button:hover:not(:disabled) {
        background-color: #d32f2f;
    }

    .kill-button:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    @media (max-width: 768px) {
        .status-bar {
            flex-wrap: wrap;
            gap: 0.5rem;
            padding: 0.5rem;
        }

        .status-left {
            width: 100%;
        }

        .status-right {
            width: 100%;
            flex-wrap: wrap;
            gap: 0.5rem;
        }

        .command {
            max-width: 30ch;
        }

        .meta.image {
            display: none;
        }
    }
</style>
