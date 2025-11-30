<script lang="ts">
    import ClaimView from '../../views/ClaimView.svelte';
    import { apiClient, hasEndpoint } from '../../stores/apiClient';
</script>

<ClaimView apiClient={$apiClient} isConfigured={$hasEndpoint} />
