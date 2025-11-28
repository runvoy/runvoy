<script lang="ts">
    import ClaimView from '../../views/ClaimView.svelte';
    import { apiEndpoint, apiKey } from '../../stores/config';
    import APIClient from '../../lib/api';

    let apiClient: APIClient | null = $state(null);
    const isConfigured = $derived(Boolean(apiClient));

    $effect(() => {
        apiClient = $apiEndpoint && $apiKey ? new APIClient($apiEndpoint, $apiKey) : null;
    });
</script>

<ClaimView {apiClient} {isConfigured} />
