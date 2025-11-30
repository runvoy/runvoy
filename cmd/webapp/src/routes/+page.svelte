<script lang="ts">
    import RunView from '../views/RunView.svelte';
    import { apiClient } from '../stores/apiClient';
</script>

<RunView apiClient={$apiClient} />
