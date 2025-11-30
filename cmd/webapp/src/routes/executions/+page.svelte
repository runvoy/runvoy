<script lang="ts">
    import ListExecutionsView from '../../views/ListExecutionsView.svelte';
    import { apiClient } from '../../stores/apiClient';
</script>

<ListExecutionsView apiClient={$apiClient} />
