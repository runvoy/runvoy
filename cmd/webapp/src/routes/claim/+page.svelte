<script lang="ts">
    import { browser } from '$app/environment';
    import ClaimView from '../../views/ClaimView.svelte';
    import { apiEndpoint, apiKey } from '../../stores/config';
    import { createApiClientFromConfig } from '../../lib/apiConfig';
    import type { PageData } from './$types';

    interface Props {
        data: PageData;
    }

    const { data }: Props = $props();

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

    // isConfigured should reflect whether we have a valid apiClient
    const isConfigured = $derived(Boolean(apiClient));
</script>

<ClaimView {apiClient} {isConfigured} />
