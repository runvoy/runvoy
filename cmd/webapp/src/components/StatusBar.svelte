<script lang="ts">
    import { FrontendStatus, isKillableStatus } from '../lib/constants';

    interface Props {
        status: string | null;
        startedAt: string | number | null;
        completedAt: string | null;
        exitCode: number | null;
        killInitiated?: boolean;
        onKill?: (() => void) | null;
    }

    const {
        status = null,
        startedAt = null,
        completedAt = null,
        exitCode = null,
        killInitiated = false,
        onKill = null
    }: Props = $props();

    let isKilling = $state(false);

    const statusClass = $derived(status ? status.toLowerCase() : 'loading');
    const formattedStartedAt = $derived.by(() => {
        if (!startedAt) {
            return 'N/A';
        }

        const dateValue = typeof startedAt === 'number' ? startedAt : Date.parse(startedAt);
        const date = new Date(dateValue);

        if (Number.isNaN(date.getTime())) {
            return 'N/A';
        }

        return date.toLocaleString();
    });

    const formattedCompletedAt = $derived.by(() => {
        if (!completedAt) {
            return null;
        }

        const dateValue = typeof completedAt === 'number' ? completedAt : Date.parse(completedAt);
        const date = new Date(dateValue);

        if (Number.isNaN(date.getTime())) {
            return null;
        }

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
    <div class="status-item">
        <strong>Status:</strong>
        <span class="status-badge {statusClass}">{status || FrontendStatus.LOADING}</span>
    </div>
    <div class="status-item">
        <strong>Started:</strong>
        <span>{formattedStartedAt}</span>
    </div>
    {#if formattedCompletedAt}
        <div class="status-item">
            <strong>Ended:</strong>
            <span>{formattedCompletedAt}</span>
        </div>
    {/if}
    {#if exitCode !== null}
        <div class="status-item">
            <strong>Exit Code:</strong>
            <code class="exit-code">{exitCode}</code>
        </div>
    {/if}
    {#if onKill && isKillableStatus(status)}
        <div class="status-item actions">
            <button
                class="kill-button"
                onclick={handleKill}
                disabled={!canKill}
                title="Kill this execution"
            >
                {isKilling ? '⏹️ Killing...' : '⏹️ Kill'}
            </button>
        </div>
    {/if}
</div>

<style>
    .status-bar {
        display: flex;
        flex-wrap: wrap;
        gap: 1.5rem;
        align-items: center;
        padding: 1rem;
        background-color: var(--pico-card-background-color);
        border: 1px solid var(--pico-card-border-color);
        border-radius: var(--pico-border-radius);
        margin-bottom: 1rem;
    }

    .status-item {
        display: flex;
        align-items: center;
        gap: 0.5rem;
    }

    .status-badge {
        padding: 0.25em 0.75em;
        border-radius: 1em;
        font-weight: bold;
        font-size: 0.8em;
        text-transform: uppercase;
        color: #fff;
    }

    .status-badge.loading {
        background-color: #78909c;
    } /* Blue Grey */
    .status-badge.starting {
        background-color: #ffc107;
    } /* Amber */
    .status-badge.running {
        background-color: #2196f3;
    } /* Blue */
    .status-badge.succeeded {
        background-color: #4caf50;
    } /* Green */
    .status-badge.failed {
        background-color: #f44336;
    } /* Red */
    .status-badge.stopped {
        background-color: #ff9800;
    } /* Orange */
    .status-badge.terminating {
        background-color: #9c27b0;
    } /* Purple */

    .exit-code {
        font-family: 'Monaco', 'Courier New', monospace;
        font-size: 0.9em;
        padding: 0.2em 0.4em;
        background-color: var(--pico-code-background-color);
        border-radius: 0.25rem;
    }

    .status-item.actions {
        margin-left: auto;
    }

    .kill-button {
        padding: 0.5rem 1rem;
        background-color: #f44336;
        color: white;
        border: none;
        border-radius: var(--pico-border-radius);
        cursor: pointer;
        font-weight: 600;
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
            flex-direction: column;
            align-items: flex-start;
            gap: 1rem;
            padding: 0.875rem;
        }

        .status-item {
            flex-direction: column;
            align-items: flex-start;
            gap: 0.25rem;
            width: 100%;
        }

        .status-badge {
            font-size: 0.75em;
        }
    }
</style>
