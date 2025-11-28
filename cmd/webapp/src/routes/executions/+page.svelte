<script lang="ts">
    import ListExecutionsView from '../../views/ListExecutionsView.svelte';
    import type { PageData } from './$types';

    interface Props {
        data: PageData;
    }

    const { data }: Props = $props();
    const { apiClient } = data;
</script>

<ListExecutionsView {apiClient} />
