<script lang="ts">
    import { browser } from '$app/environment';
    import { page } from '$app/stores';
    import LogsView from '../../views/LogsView.svelte';
    import { apiEndpoint, apiKey } from '../../stores/config';
    import { createApiClientFromConfig } from '../../lib/apiConfig';
    import type { PageData } from './$types';

    interface Props {
        data: PageData;
    }

    const { data }: Props = $props();

    // Derive execution ID from URL - this is the single source of truth
    const currentExecutionId = $derived(
        $page.url.searchParams.get('execution_id') ||
            $page.url.searchParams.get('executionId') ||
            $page.url.searchParams.get('executionID') ||
            null
    );

    // Use server-side apiClient if available, otherwise create from stores on client
    const apiClient = $derived(
        data.apiClient ??
            (browser
                ? createApiClientFromConfig(
                      {
                          endpoint: $apiEndpoint,
                          apiKey: $apiKey
                      },
                      fetch
                  )
                : null)
    );
</script>

<LogsView {apiClient} {currentExecutionId} />
