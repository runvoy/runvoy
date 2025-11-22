<script lang="ts">
    import ClaimView from '../../views/ClaimView.svelte';
    import { apiEndpoint, apiKey } from '../../stores/config';
    import APIClient from '../../lib/api';

    let apiClient: APIClient | null = null;
    let isConfigured = false;

    $: apiClient = $apiEndpoint && $apiKey ? new APIClient($apiEndpoint, $apiKey) : null;
    $: isConfigured = Boolean(apiClient);
</script>

<ClaimView {apiClient} {isConfigured} />
