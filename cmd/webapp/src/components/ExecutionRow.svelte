<script lang="ts">
    import type { Execution } from '../types/api';

    interface Props {
        execution: Execution;
        onView: (execution: Execution) => void;
    }

    const { execution, onView }: Props = $props();

    function formatDate(dateString: string | undefined): string {
        if (!dateString) return '-';
        try {
            const date = new Date(dateString);
            return date.toLocaleString();
        } catch {
            return dateString;
        }
    }

    function getStatusClass(status: string): string {
        if (!status) return 'loading';
        const normalizedStatus = status.toLowerCase();
        return normalizedStatus;
    }

    function formatExecutionId(executionId: string | undefined): string {
        if (!executionId) return 'N/A';
        return executionId.slice(0, 8) + '...';
    }
</script>

<tr>
    <td class="execution-id">
        <code>{formatExecutionId(execution.execution_id)}</code>
    </td>
    <td>
        <span class="status-badge {getStatusClass(execution.status)}">
            {execution.status}
        </span>
    </td>
    <td>{formatDate(execution.started_at)}</td>
    <td>{formatDate(execution.completed_at)}</td>
    <td class="exit-code">
        {execution.exit_code ?? '-'}
    </td>
    <td class="action-cell">
        <button
            class="secondary"
            onclick={() => onView(execution)}
            aria-label="View execution {execution.execution_id}"
        >
            View
        </button>
    </td>
</tr>

<style>
    .execution-id {
        font-family: 'Monaco', 'Courier New', monospace;
        font-size: 0.875rem;
    }

    code {
        background: var(--pico-code-background-color);
        padding: 0.25rem 0.5rem;
        border-radius: 0.25rem;
        font-size: 0.9em;
    }

    .status-badge {
        display: inline-block;
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
        text-align: center;
    }

    .action-cell {
        text-align: right;
    }

    .action-cell button {
        margin: 0;
    }

    @media (max-width: 768px) {
        .status-badge {
            padding: 0.25rem 0.5rem;
            font-size: 0.75rem;
        }

        .execution-id {
            font-size: 0.75rem;
        }

        code {
            font-size: 0.8em;
        }
    }
</style>
