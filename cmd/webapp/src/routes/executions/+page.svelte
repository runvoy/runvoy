<script lang="ts">
    import ListExecutionsView from '../../views/ListExecutionsView.svelte';
    import { apiEndpoint, apiKey } from '../../stores/config';
    import APIClient from '../../lib/api';

    let apiClient: APIClient | null = null;
    let isConfigured = false;

    $: apiClient = $apiEndpoint && $apiKey ? new APIClient($apiEndpoint, $apiKey) : null;
    $: isConfigured = Boolean(apiClient);
</script>

<ListExecutionsView {apiClient} {isConfigured} />
