<script lang="ts">
    import SettingsView from '../../views/SettingsView.svelte';
    import { apiEndpoint, apiKey } from '../../stores/config';
    import APIClient from '../../lib/api';

    let apiClient: APIClient | null = null;
    let isConfigured = false;

    $: apiClient = $apiEndpoint && $apiKey ? new APIClient($apiEndpoint, $apiKey) : null;
    $: isConfigured = Boolean(apiClient);
</script>

<SettingsView {apiClient} {isConfigured} />
