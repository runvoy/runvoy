<script lang="ts">
    import ConnectionManager from '../components/ConnectionManager.svelte';
    import ViewSwitcher from '../components/ViewSwitcher.svelte';
    import { apiEndpoint, apiKey } from '../stores/config';
    import { VIEWS } from '../stores/ui';

    import '../styles/global.css';

    interface NavView {
        id: string;
        label: string;
        disabled?: boolean;
    }

    const appVersion = import.meta.env.VITE_RUNVOY_VERSION || '';
    let isConfigured = false;

    const views: NavView[] = [
        { id: VIEWS.RUN, label: 'Run Command' },
        { id: VIEWS.LIST, label: 'Executions' },
        { id: VIEWS.CLAIM, label: 'Claim Key' },
        { id: VIEWS.LOGS, label: 'Logs' },
        { id: VIEWS.SETTINGS, label: 'Settings' }
    ];
    let navViews: NavView[] = views;

    $: isConfigured = Boolean($apiEndpoint && $apiKey);

    $: navViews = views.map((view) =>
        view.id === VIEWS.LOGS || view.id === VIEWS.LIST
            ? { ...view, disabled: !isConfigured }
            : view
    );
</script>

<main class="container">
    <header class="app-header">
        <div class="header-content">
            <div class="header-title">
                <h1>
                    🚀 runvoy
                    {#if appVersion}
                        <span class="version">{appVersion}</span>
                    {/if}
                </h1>
                <p class="subtitle">
                    <a href="https://github.com/runvoy/runvoy" target="_blank" rel="noopener">
                        View on GitHub
                    </a>
                </p>
            </div>
            <div class="header-actions">
                <ConnectionManager />
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
        display: grid;
        grid-template-columns: 1fr auto;
        gap: 1.5rem;
        align-items: start;
        margin-bottom: 1.5rem;
    }

    .header-title {
        min-width: 0;
    }

    .header-title h1 {
        margin-bottom: 0.25rem;
        font-size: 1.75rem;
    }

    .version {
        color: var(--pico-muted-color);
        font-size: 0.75rem;
        font-weight: normal;
        margin-left: 0.5rem;
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

    .header-actions {
        display: flex;
        align-items: flex-start;
    }

    .header-nav {
        margin-top: 1rem;
    }

    .content-area {
        min-height: 400px;
    }

    @media (max-width: 768px) {
        .header-content {
            grid-template-columns: 1fr;
            gap: 1rem;
        }

        .header-actions {
            width: 100%;
        }

        .header-title h1 {
            font-size: 1.5rem;
        }
    }
</style>
