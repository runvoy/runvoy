<script lang="ts">
    import SettingsView from '../../views/SettingsView.svelte';
    import { apiEndpoint, apiKey } from '../../stores/config';
    import APIClient from '../../lib/api';

    const apiClient = $derived.by(() => {
        return $apiEndpoint && $apiKey ? new APIClient($apiEndpoint, $apiKey) : null;
    });
    const isConfigured = $derived(Boolean(apiClient));
</script>

<SettingsView {apiClient} {isConfigured} />
