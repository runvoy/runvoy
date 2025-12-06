<script lang="ts">
    import { goto } from '$app/navigation';
    import { page } from '$app/state';
    import LogsView from '../../views/LogsView.svelte';
    import { apiClient } from '../../stores/apiClient';
    import { executionId } from '../../stores/execution';

    // Get execution ID from URL - this is the single source of truth
    const urlExecutionId = $derived(
        page.url.searchParams.get('execution_id') ||
            page.url.searchParams.get('executionId') ||
            page.url.searchParams.get('executionID') ||
            null
    );

    // Restore from store if URL has no execution ID
    $effect(() => {
        if (!urlExecutionId && $executionId) {
            goto(`/logs?execution_id=${encodeURIComponent($executionId)}`, {
                replaceState: true
            });
        }
    });

    // Save to store when viewing an execution
    $effect(() => {
        if (urlExecutionId) {
            executionId.set(urlExecutionId);
        }
    });
</script>

{#if $apiClient}
	<LogsView apiClient={$apiClient} currentExecutionId={urlExecutionId} />
{/if}
