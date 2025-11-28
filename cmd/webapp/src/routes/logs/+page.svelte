<script lang="ts">
    import { page } from '$app/stores';
    import LogsView from '../../views/LogsView.svelte';
    import { apiEndpoint, apiKey } from '../../stores/config';
    import APIClient from '../../lib/api';
    import { switchExecution } from '../../lib/executionState';

    let apiClient: APIClient | null = $state(null);

    $effect(() => {
        apiClient = $apiEndpoint && $apiKey ? new APIClient($apiEndpoint, $apiKey) : null;
    });

    $effect(() => {
        const execId =
            $page.url.searchParams.get('execution_id') ||
            $page.url.searchParams.get('executionId') ||
            $page.url.searchParams.get('executionID');

        if (execId) {
            switchExecution(execId, { updateHistory: false });
        }
    });
</script>

<LogsView {apiClient} />
