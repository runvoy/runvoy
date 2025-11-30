<script lang="ts">
    import { ExecutionStatus } from '../lib/constants';
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

    function getStatusColor(status: string): string {
        if (status === ExecutionStatus.SUCCEEDED) {
            return 'success';
        }
        if (status === ExecutionStatus.FAILED || status === ExecutionStatus.STOPPED) {
            return 'danger';
        }
        if (status === ExecutionStatus.RUNNING) {
            return 'info';
        }
        return 'default';
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
        <span class="status-badge status-{getStatusColor(execution.status)}">
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
        padding: 0.375rem 0.75rem;
        border-radius: 0.25rem;
        font-weight: 600;
        font-size: 0.875rem;
    }

    .status-success {
        background-color: var(--pico-color-green-600);
        color: white;
    }

    .status-danger {
        background-color: var(--pico-color-red-600);
        color: white;
    }

    .status-info {
        background-color: var(--pico-color-blue-600);
        color: white;
    }

    .status-default {
        background-color: var(--pico-muted-color);
        color: white;
    }

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
