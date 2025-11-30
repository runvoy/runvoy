<script lang="ts">
    import SettingsView from '../../views/SettingsView.svelte';
    import { apiClient, hasEndpoint } from '../../stores/apiClient';
</script>

<SettingsView apiClient={$apiClient} isConfigured={$hasEndpoint} />
