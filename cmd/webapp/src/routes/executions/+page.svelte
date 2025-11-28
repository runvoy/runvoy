<script lang="ts">
    import ListExecutionsView from '../../views/ListExecutionsView.svelte';
    import { apiEndpoint, apiKey } from '../../stores/config';
    import APIClient from '../../lib/api';

    let apiClient: APIClient | null = $state(null);

    $effect(() => {
        apiClient = $apiEndpoint && $apiKey ? new APIClient($apiEndpoint, $apiKey) : null;
    });
</script>

<ListExecutionsView {apiClient} />
