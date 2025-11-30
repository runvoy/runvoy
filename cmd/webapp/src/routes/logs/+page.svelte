<script lang="ts">
    import { page } from '$app/state';
    import LogsView from '../../views/LogsView.svelte';
    import { apiClient } from '../../stores/apiClient';

    // Derive execution ID from URL - this is the single source of truth
    const currentExecutionId = $derived(
        page.url.searchParams.get('execution_id') ||
            page.url.searchParams.get('executionId') ||
            page.url.searchParams.get('executionID') ||
            null
    );
</script>

<LogsView apiClient={$apiClient} {currentExecutionId} />
