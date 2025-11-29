<script lang="ts">
    import { page } from '$app/stores';
    import LogsView from '../../views/LogsView.svelte';
    import type { PageData } from './$types';

    interface Props {
        data: PageData;
    }

    const { data }: Props = $props();
    const { apiClient } = data;

    // Derive execution ID from URL - this is the single source of truth
    const currentExecutionId = $derived(
        $page.url.searchParams.get('execution_id') ||
            $page.url.searchParams.get('executionId') ||
            $page.url.searchParams.get('executionID') ||
            null
    );
</script>

<LogsView {apiClient} {currentExecutionId} />
