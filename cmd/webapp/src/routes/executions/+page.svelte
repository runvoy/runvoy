<script lang="ts">
    import ListExecutionsView from '../../views/ListExecutionsView.svelte';
    import { apiEndpoint, apiKey } from '../../stores/config';
    import APIClient from '../../lib/api';

    const apiClient = $derived.by(() => {
        return $apiEndpoint && $apiKey ? new APIClient($apiEndpoint, $apiKey) : null;
    });
</script>

<ListExecutionsView {apiClient} />
