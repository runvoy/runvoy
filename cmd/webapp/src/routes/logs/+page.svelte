<script lang="ts">
    import { page } from '$app/stores';
    import LogsView from '../../views/LogsView.svelte';
    import type { PageData } from './$types';
    import { switchExecution } from '../../lib/executionState';

    interface Props {
        data: PageData;
    }

    const { data }: Props = $props();
    const { apiClient } = data;

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
