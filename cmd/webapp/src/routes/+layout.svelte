<script lang="ts">
    import { version } from '$app/environment';
    import { goto } from '$app/navigation';
    import { page } from '$app/stores';
    import ViewSwitcher from '../components/ViewSwitcher.svelte';
    import { apiEndpoint, apiKey } from '../stores/config';
    import { VIEWS } from '../stores/ui';

    import '../styles/global.css';

    interface NavView {
        id: string;
        label: string;
        disabled?: boolean;
    }

    let hasEndpoint = false;
    let hasApiKey = false;

    const views: NavView[] = [
        { id: VIEWS.RUN, label: 'Run Command' },
        { id: VIEWS.LIST, label: 'Executions' },
        { id: VIEWS.CLAIM, label: 'Claim Key' },
        { id: VIEWS.LOGS, label: 'Logs' },
        { id: VIEWS.SETTINGS, label: 'Settings' }
    ];
    let navViews: NavView[] = views;

    $: hasEndpoint = Boolean($apiEndpoint);
    $: hasApiKey = Boolean($apiKey);

    $: navViews = views.map((view) =>
        view.id === VIEWS.LOGS || view.id === VIEWS.LIST ? { ...view, disabled: !hasApiKey } : view
    );

    $: {
        const pathname = $page.url.pathname;
        const isSettingsOrClaim = pathname === '/settings' || pathname === '/claim';

        // Redirect to settings if endpoint is missing (unless already on settings/claim)
        if (!hasEndpoint && !isSettingsOrClaim) {
            goto('/settings');
        }
        // Redirect to settings if API key is missing (unless on settings/claim)
        // This allows claim view to work with just endpoint configured
        else if (!hasApiKey && !isSettingsOrClaim) {
            goto('/settings');
        }
    }
</script>

<main class="container">
    <header class="app-header">
        <div class="header-content">
            <div class="header-title">
                <h1>
                    <img src="/runvoy-avatar.png" alt="runvoy logo" class="avatar" />
                    <div>
                        {#if version}
                            <span class="version">{version}</span>
                        {/if}
                        <p class="subtitle">
                            <a href="https://runvoy.site/" target="_blank" rel="noopener">
                                Documentation
                            </a>
                        </p>
                    </div>
                </h1>
            </div>
        </div>
        <div class="header-nav">
            <ViewSwitcher views={navViews} />
        </div>
    </header>

    <div class="content-area">
        <slot />
    </div>
</main>

<style>
    /* Pico's .container class on main element handles max-width and centering */

    .app-header {
        margin-bottom: 2rem;
        border-bottom: 1px solid var(--pico-border-color);
        padding-bottom: 1.5rem;
    }

    .header-content {
        display: flex;
        gap: 1.5rem;
        align-items: center;
        justify-content: space-between;
        margin-bottom: 1.5rem;
    }

    .header-title h1 {
        margin-bottom: 0.25rem;
        font-size: 1.75rem;
        display: flex;
        align-items: center;
        gap: 0.5rem;
    }

    .avatar {
        height: 3em;
        width: 3em;
        object-fit: contain;
        vertical-align: middle;
    }

    .version {
        color: var(--pico-muted-color);
        font-size: 0.75rem;
        /* font-weight: normal;
        margin-left: 0.5rem; */
    }

    .subtitle {
        margin: 0;
        color: var(--pico-muted-color);
        font-size: 0.875rem;
    }

    .subtitle a {
        color: var(--pico-muted-color);
        text-decoration: none;
    }

    .subtitle a:hover {
        text-decoration: underline;
        color: var(--pico-primary);
    }

    .header-nav {
        margin-top: 1rem;
    }

    .content-area {
        min-height: 400px;
    }

    @media (max-width: 768px) {
        .header-content {
            flex-direction: column;
            align-items: flex-start;
            gap: 1rem;
        }

        .header-title h1 {
            font-size: 1.5rem;
        }
    }
</style>
