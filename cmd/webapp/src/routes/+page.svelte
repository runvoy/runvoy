<script lang="ts">
    import RunView from '../views/RunView.svelte';
    import type { PageData } from './$types';

    interface Props {
        data: PageData;
    }

    const { data }: Props = $props();
    const { apiClient } = data;
</script>

<RunView {apiClient} />
