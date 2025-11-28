<script lang="ts">
    import ClaimView from '../../views/ClaimView.svelte';
    import type { PageData } from './$types';

    interface Props {
        data: PageData;
    }

    const { data }: Props = $props();
    const { apiClient, isConfigured } = data;
</script>

<ClaimView {apiClient} {isConfigured} />
