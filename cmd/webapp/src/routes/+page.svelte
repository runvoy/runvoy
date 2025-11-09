<script>
    import { onMount } from 'svelte';
    import { apiEndpoint, apiKey } from '../stores/config.js';
    import { get } from 'svelte/store';
    import { activeView, VIEWS } from '../stores/ui.js';
    import APIClient from '../lib/api.js';
    import { switchExecution } from '../lib/executionState.js';
    import ConnectionManager from '../components/ConnectionManager.svelte';
    import ViewSwitcher from '../components/ViewSwitcher.svelte';
    import RunView from '../views/RunView.svelte';
    import LogsView from '../views/LogsView.svelte';
    import { executionId } from '../stores/execution.js';

    import '../styles/global.css';

    let apiClient = null;
    let isConfigured = false;

    const views = [
        { id: VIEWS.RUN, label: 'Run Command' },
        { id: VIEWS.LOGS, label: 'Logs' }
    ];
    let navViews = views;

    onMount(() => {
        if (typeof window === 'undefined') {
            return;
        }

        const urlParams = new URLSearchParams(window.location.search);
        const execId = urlParams.get('execution_id') || urlParams.get('executionId');

        if (execId) {
            switchExecution(execId, { updateHistory: false });
            activeView.set(VIEWS.LOGS);
        } else {
            document.title = 'runvoy Logs';
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

    $: {
        if (isConfigured && get(executionId) && $activeView === VIEWS.RUN) {
            activeView.set(VIEWS.LOGS);
        }
    }
</script>

<ConnectionManager />

<main class="container">
    <header>
        <h1>runvoy Console</h1>
        <p class="subtitle">
            <a href="https://github.com/runvoy/runvoy" target="_blank" rel="noopener">
                View on GitHub
            </a>
        </p>
    </header>

    <ViewSwitcher views={navViews} />

    {#if $activeView === VIEWS.RUN}
        <RunView {apiClient} {isConfigured} />
    {:else if $activeView === VIEWS.LOGS}
        <LogsView {apiClient} {isConfigured} />
    {/if}
</main>

<style>
    main {
        padding: 2rem;
        padding-top: 4rem; /* Account for fixed config button */
    }

    header {
        margin-bottom: 2rem;
    }

    h1 {
        margin-bottom: 0.5rem;
    }

    .subtitle {
        margin-top: 0;
        color: var(--pico-muted-color);
    }

    .subtitle a {
        color: var(--pico-muted-color);
        text-decoration: none;
    }

    .subtitle a:hover {
        text-decoration: underline;
    }
</style>
