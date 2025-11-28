<script lang="ts">
    import RunView from '../views/RunView.svelte';
    import { apiEndpoint, apiKey } from '../stores/config';
    import APIClient from '../lib/api';

    const apiClient = $derived.by(() => {
        return $apiEndpoint && $apiKey ? new APIClient($apiEndpoint, $apiKey) : null;
    });
</script>

<RunView {apiClient} />
