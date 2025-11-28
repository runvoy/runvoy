<script lang="ts">
    import SettingsView from '../../views/SettingsView.svelte';
    import { apiEndpoint, apiKey } from '../../stores/config';
    import APIClient from '../../lib/api';

    let apiClient: APIClient | null = $state(null);
    const isConfigured = $derived(Boolean(apiClient));

    $effect(() => {
        apiClient = $apiEndpoint && $apiKey ? new APIClient($apiEndpoint, $apiKey) : null;
    });
</script>

<SettingsView {apiClient} {isConfigured} />
