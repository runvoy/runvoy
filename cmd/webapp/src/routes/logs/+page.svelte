<script lang="ts">
    import { page } from '$app/stores';
    import { get } from 'svelte/store';
    import LogsView from '../../views/LogsView.svelte';
    import type { PageData } from './$types';
    import { switchExecution } from '../../lib/executionState';
    import { executionId } from '../../stores/execution';

    interface Props {
        data: PageData;
    }

    const { data }: Props = $props();
    const { apiClient } = data;

    let lastProcessedExecId: string | null = $state(null);

    $effect(() => {
        const execId =
            $page.url.searchParams.get('execution_id') ||
            $page.url.searchParams.get('executionId') ||
            $page.url.searchParams.get('executionID');

        // Only switch if the execution ID is different from what we last processed
        // and different from what's currently in the store
        if (execId && execId !== lastProcessedExecId) {
            const currentId = get(executionId);
            if (execId !== currentId) {
                lastProcessedExecId = execId;
                switchExecution(execId, { updateHistory: false });
            } else {
                lastProcessedExecId = execId;
            }
        } else if (!execId) {
            lastProcessedExecId = null;
        }
    });
</script>

<LogsView {apiClient} />
