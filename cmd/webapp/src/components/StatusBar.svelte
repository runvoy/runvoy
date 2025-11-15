<script>
    import { executionId, executionStatus, startedAt } from '../stores/execution.js';

    const DEFAULT_STATUS = 'LOADING';

    $: statusClass = $executionStatus ? $executionStatus.toLowerCase() : 'loading';
    $: formattedStartedAt = (() => {
        if (!$startedAt) {
            return 'N/A';
        }

        const dateValue = typeof $startedAt === 'number' ? $startedAt : Date.parse($startedAt);
        const date = new Date(dateValue);

        if (Number.isNaN(date.getTime())) {
            return 'N/A';
        }

        return date.toLocaleString();
    })();
</script>

<div class="status-bar">
    <div class="status-item">
        <strong>Status:</strong>
        <span class="status-badge {statusClass}">{$executionStatus || DEFAULT_STATUS}</span>
    </div>
    <div class="status-item">
        <strong>Started:</strong>
        <span>{formattedStartedAt}</span>
    </div>
    <div class="status-item">
        <strong>Execution ID:</strong>
        <code class="execution-id">{$executionId}</code>
    </div>
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

    .execution-id {
        font-size: 0.9em;
        padding: 0.2em 0.4em;
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

        .execution-id {
            font-size: 0.85em;
            word-break: break-all;
        }

        .status-badge {
            font-size: 0.75em;
        }
    }
</style>
