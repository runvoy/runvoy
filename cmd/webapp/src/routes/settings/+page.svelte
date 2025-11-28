<script lang="ts">
    import SettingsView from '../../views/SettingsView.svelte';
    import type { PageData } from './$types';

    interface Props {
        data: PageData;
    }

    const { data }: Props = $props();
    const { apiClient, isConfigured } = data;
</script>

<SettingsView {apiClient} {isConfigured} />
