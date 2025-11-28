<script lang="ts">
    import RunView from '../views/RunView.svelte';
    import { apiEndpoint, apiKey } from '../stores/config';
    import APIClient from '../lib/api';

    let apiClient: APIClient | null = null;

    $: apiClient = $apiEndpoint && $apiKey ? new APIClient($apiEndpoint, $apiKey) : null;
</script>

<RunView {apiClient} />
