<script lang="ts">
    import { onMount } from 'svelte';
    import type { ComponentType } from 'svelte';
    import { apiEndpoint, apiKey } from '../stores/config';
    import { activeView, VIEWS } from '../stores/ui';
    import APIClient from '../lib/api';
    import { switchExecution } from '../lib/executionState';
    import ConnectionManager from '../components/ConnectionManager.svelte';
    import ViewSwitcher from '../components/ViewSwitcher.svelte';
    import RunView from '../views/RunView.svelte';
    import LogsView from '../views/LogsView.svelte';
    import ClaimView from '../views/ClaimView.svelte';
    import SettingsView from '../views/SettingsView.svelte';

    import '../styles/global.css';

    interface NavView {
        id: string;
        label: string;
        disabled?: boolean;
    }

    const appVersion = import.meta.env.VITE_RUNVOY_VERSION || '';
    let apiClient: APIClient | null = null;
    let isConfigured = false;

    const views: NavView[] = [
        { id: VIEWS.RUN, label: 'Run Command' },
        { id: VIEWS.CLAIM, label: 'Claim Key' },
        { id: VIEWS.LOGS, label: 'Logs' },
        { id: VIEWS.SETTINGS, label: 'Settings' }
    ];
    let navViews: NavView[] = views;

    const viewComponents: Record<string, ComponentType> = {
        [VIEWS.RUN]: RunView,
        [VIEWS.CLAIM]: ClaimView,
        [VIEWS.LOGS]: LogsView,
        [VIEWS.SETTINGS]: SettingsView
    };

    onMount(() => {
        if (typeof window === 'undefined') {
            return;
        }

        const urlParams = new URLSearchParams(window.location.search);
        const execId = urlParams.get('execution_id') || urlParams.get('executionId');

        if (execId) {
            switchExecution(execId, { updateHistory: false });
            activeView.set(VIEWS.LOGS);
        }
    });

    $: apiClient = $apiEndpoint && $apiKey ? new APIClient($apiEndpoint, $apiKey) : null;

    $: isConfigured = Boolean(apiClient);

    $: navViews = views.map((view) =>
        view.id === VIEWS.LOGS ? { ...view, disabled: !isConfigured } : view
    );

    $: if (!isConfigured) {
        activeView.set(VIEWS.RUN);
    }

    $: currentComponent = viewComponents[$activeView] || RunView;

    $: componentProps =
        $activeView === VIEWS.RUN || $activeView === VIEWS.LOGS ? { apiClient, isConfigured } : {};
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
        <svelte:component this={currentComponent} {...componentProps} />
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
