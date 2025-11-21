<script lang="ts">
    import { activeView, VIEWS } from '../stores/ui';
    import { switchExecution } from '../lib/executionState';
    import type APIClient from '../lib/api';
    import type { Execution, ApiError } from '../types/api';

    export let apiClient: APIClient | null = null;
    export let isConfigured = false;

    let executions: Execution[] = [];
    let isLoading = false;
    let errorMessage = '';
    let hasAttemptedLoad = false;

    async function loadExecutions(allowRefresh = false): Promise<void> {
        if (!isConfigured || !apiClient) {
            return;
        }

        if (isLoading) {
            return; // Prevent multiple concurrent requests
        }

        // On initial load, check if we've already loaded. On refresh, allow it
        if (!allowRefresh && hasAttemptedLoad) {
            return;
        }

        isLoading = true;
        hasAttemptedLoad = true;
        errorMessage = '';

        try {
            const response = await apiClient.listExecutions();
            // Response is an array of executions
            executions = response || [];
        } catch (error) {
            const err = error as ApiError;
            errorMessage = err.details?.error || err.message || 'Failed to load executions';
            executions = [];
        } finally {
            isLoading = false;
        }
    }

    function handleViewExecution(execution: Execution): void {
        switchExecution(execution.execution_id);
        activeView.set(VIEWS.LOGS);
    }

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
        if (status === 'SUCCEEDED') {
            return 'success';
        }
        if (status === 'FAILED' || status === 'STOPPED') {
            return 'danger';
        }
        if (status === 'RUNNING') {
            return 'info';
        }
        return 'default';
    }

    $: if (apiClient && isConfigured) {
        loadExecutions();
    }
</script>

{#if !isConfigured}
    <article class="info-card">
        <h2>Configure API access to view executions</h2>
        <p>
            Use the <strong>⚙️ Configure API</strong> button to set the endpoint and API key for your
            runvoy backend. Once configured, you can view past executions here.
        </p>
    </article>
{:else}
    <article class="list-card">
        <header>
            <h2>Execution History</h2>
            <button on:click={() => loadExecutions(true)} disabled={isLoading} class="secondary">
                {isLoading ? '⟳ Refreshing...' : '⟳ Refresh'}
            </button>
        </header>

        {#if errorMessage}
            <div class="error-box">
                <p>{errorMessage}</p>
            </div>
        {/if}

        {#if isLoading}
            <p class="loading-text">Loading executions...</p>
        {:else if executions.length === 0}
            <p class="empty-text">No executions found. Start by running a command.</p>
        {:else}
            <div class="table-wrapper">
                <table>
                    <thead>
                        <tr>
                            <th>ID</th>
                            <th>Status</th>
                            <th>Started</th>
                            <th>Completed</th>
                            <th>Exit Code</th>
                            <th>Action</th>
                        </tr>
                    </thead>
                    <tbody>
                        {#each executions as execution (execution.execution_id)}
                            <tr>
                                <td class="execution-id">
                                    <code
                                        >{execution.execution_id
                                            ? execution.execution_id.slice(0, 8) + '...'
                                            : 'N/A'}</code
                                    >
                                </td>
                                <td>
                                    <span
                                        class="status-badge status-{getStatusColor(
                                            execution.status
                                        )}"
                                    >
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
                                        on:click={() => handleViewExecution(execution)}
                                        aria-label="View execution {execution.execution_id}"
                                    >
                                        View
                                    </button>
                                </td>
                            </tr>
                        {/each}
                    </tbody>
                </table>
            </div>
        {/if}
    </article>
{/if}

<style>
    .info-card {
        background: var(--pico-card-background-color);
        border: 1px solid var(--pico-card-border-color);
        border-radius: var(--pico-border-radius);
        padding: 1.5rem;
    }

    .list-card {
        background: var(--pico-card-background-color);
        border: 1px solid var(--pico-card-border-color);
        border-radius: var(--pico-border-radius);
        padding: 2rem;
    }

    header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 1.5rem;
        gap: 1rem;
    }

    header h2 {
        margin: 0;
    }

    header button {
        white-space: nowrap;
    }

    .error-box {
        background-color: var(--pico-card-background-color);
        border: 1px solid var(--pico-card-border-color);
        border-left: 4px solid var(--pico-color-red-500);
        padding: 1rem 1.5rem;
        margin-bottom: 1.5rem;
        border-radius: var(--pico-border-radius);
    }

    .error-box p {
        margin: 0;
        color: var(--pico-color-red-500);
        font-weight: bold;
    }

    .loading-text,
    .empty-text {
        text-align: center;
        color: var(--pico-muted-color);
        padding: 2rem 0;
        margin: 0;
    }

    .table-wrapper {
        overflow-x: auto;
    }

    table {
        width: 100%;
        border-collapse: collapse;
        margin: 0;
    }

    thead {
        background-color: var(--pico-form-element-background-color);
    }

    th {
        text-align: left;
        padding: 1rem;
        font-weight: 600;
        border-bottom: 2px solid var(--pico-border-color);
    }

    td {
        padding: 0.875rem 1rem;
        border-bottom: 1px solid var(--pico-border-color);
    }

    tbody tr:hover {
        background-color: var(--pico-form-element-background-color);
    }

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
        .list-card {
            padding: 1.5rem;
        }

        header {
            flex-direction: column;
            align-items: stretch;
        }

        header button {
            width: 100%;
        }

        .table-wrapper {
            overflow-x: auto;
            -webkit-overflow-scrolling: touch;
        }

        table {
            font-size: 0.875rem;
        }

        th,
        td {
            padding: 0.75rem 0.5rem;
        }

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
