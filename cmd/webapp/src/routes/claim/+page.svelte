<script lang="ts">
    import ClaimView from '../../views/ClaimView.svelte';
    import { apiEndpoint, apiKey } from '../../stores/config';
    import APIClient from '../../lib/api';

    const apiClient = $derived.by(() => {
        return $apiEndpoint && $apiKey ? new APIClient($apiEndpoint, $apiKey) : null;
    });
    const isConfigured = $derived(Boolean(apiClient));
</script>

<ClaimView {apiClient} {isConfigured} />
