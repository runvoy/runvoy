<script lang="ts">
    import { goto } from '$app/navigation';
    import type APIClient from '../lib/api';
    import type { Execution, ApiError } from '../types/api';
    import ExecutionRow from '../components/ExecutionRow.svelte';

    interface Props {
        apiClient: APIClient | null;
    }

    const { apiClient = null }: Props = $props();

    let executions: Execution[] = $state([]);
    let isLoading = $state(false);
    let errorMessage = $state('');
    let hasAttemptedLoad = $state(false);

    async function loadExecutions(allowRefresh = false): Promise<void> {
        if (!apiClient) {
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
        const executionId = execution.execution_id;
        if (executionId) {
            goto(`/logs?executionID=${encodeURIComponent(executionId)}`);
        }
    }

    $effect(() => {
        if (apiClient) {
            loadExecutions();
        }
    });
</script>

<article class="list-card">
    <header>
        <h2>Execution History</h2>
        <button onclick={() => loadExecutions(true)} disabled={isLoading} class="secondary">
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
                        <th>Ended</th>
                        <th>Exit Code</th>
                        <th>Action</th>
                    </tr>
                </thead>
                <tbody>
                    {#each executions as execution (execution.execution_id)}
                        <ExecutionRow {execution} onView={handleViewExecution} />
                    {/each}
                </tbody>
            </table>
        </div>
    {/if}
</article>

<style>
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

    :global(td) {
        padding: 0.875rem 1rem;
        border-bottom: 1px solid var(--pico-border-color);
    }

    :global(tbody tr:hover) {
        background-color: var(--pico-form-element-background-color);
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
        :global(td) {
            padding: 0.75rem 0.5rem;
        }
    }
</style>
