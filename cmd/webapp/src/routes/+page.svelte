<script>
    import { onMount } from 'svelte';
    import { executionId } from '../stores/execution.js';
    import { apiEndpoint, apiKey } from '../stores/config.js';
    import { logEvents, logsRetryCount } from '../stores/logs.js';
    import { cachedWebSocketURL } from '../stores/websocket.js';
    import APIClient from '../lib/api.js';
    import { connectWebSocket, disconnectWebSocket } from '../lib/websocket.js';

    import ExecutionSelector from '../components/ExecutionSelector.svelte';
    import ConnectionManager from '../components/ConnectionManager.svelte';
    import LogViewer from '../components/LogViewer.svelte';
    import WebSocketStatus from '../components/WebSocketStatus.svelte';
    import StatusBar from '../components/StatusBar.svelte';
    import LogControls from '../components/LogControls.svelte';

    import '../styles/global.css';

    let showWelcome = false;
    let apiClient;
    let fetchLogsTimer;
    let errorMessage = '';

    const MAX_LOG_RETRIES = 2;
    const LOG_RETRY_DELAY = 10000; // 10 seconds

    onMount(() => {
        const urlParams = new URLSearchParams(window.location.search);
        const execId = urlParams.get('execution_id') || urlParams.get('executionId');

        if (execId) {
            executionId.set(execId);
        }

        document.title = execId ? `runvoy Logs - ${execId}` : 'runvoy Logs';

        if (!$apiEndpoint || !$apiKey) {
            showWelcome = true;
        }

        window.addEventListener('credentials-updated', () => {
            apiEndpoint.set($apiEndpoint);
            apiKey.set($apiKey);
        });

        return () => {
            clearTimeout(fetchLogsTimer);
            disconnectWebSocket();
        };
    });

    async function fetchLogs() {
        if (!apiClient || !$executionId) return;

        clearTimeout(fetchLogsTimer);
        errorMessage = '';

        try {
            const response = await apiClient.getLogs($executionId);
            const eventsWithLines = (response.events || []).map((event, index) => ({
                ...event,
                line: index + 1,
            }));
            logEvents.set(eventsWithLines);
            cachedWebSocketURL.set(response.websocket_url);
            logsRetryCount.set(0);
        } catch (error) {
            if (error.status === 404 && $logsRetryCount < MAX_LOG_RETRIES) {
                errorMessage = `Execution not found, retrying... (${$logsRetryCount + 1}/${MAX_LOG_RETRIES})`;
                logsRetryCount.update(n => n + 1);
                fetchLogsTimer = setTimeout(fetchLogs, LOG_RETRY_DELAY);
            } else {
                errorMessage = error.details?.error || error.message || 'Failed to fetch logs';
                logEvents.set([]);
            }
        }
    }

    $: apiClient = ($apiEndpoint && $apiKey) ? new APIClient($apiEndpoint, $apiKey) : null;

    $: if (apiClient) {
        showWelcome = false;
    } else {
        showWelcome = true;
    }

    $: if ($executionId && apiClient) {
        disconnectWebSocket();
        fetchLogs();
    }

    $: if ($cachedWebSocketURL) {
        connectWebSocket($cachedWebSocketURL);
    }
</script>

<ConnectionManager />

<main class="container">
    <header>
        <h1>runvoy Log Viewer</h1>
        <p class="subtitle">
            <a href="https://github.com/runvoy/runvoy" target="_blank" rel="noopener">
                View on GitHub
            </a>
        </p>
    </header>

    <ExecutionSelector />

    {#if errorMessage}
        <article class="error-box">
            <p>{errorMessage}</p>
        </article>
    {/if}

    {#if showWelcome}
        <article>
            <header>
                <strong>Welcome to runvoy Log Viewer</strong>
            </header>
            <p>To get started:</p>
            <ol>
                <li>Click the "⚙️ Configure API" button to set your API endpoint and key</li>
                <li>Enter an execution ID in the field above</li>
                <li>View logs and monitor execution status in real-time</li>
            </ol>
            <footer>
                <small>Your credentials are stored locally in your browser and never sent to third parties.</small>
            </footer>
        </article>
    {:else if !$executionId}
        <article>
            <p>Enter an execution ID above or provide <code>?execution_id=&lt;id&gt;</code> in the URL</p>
        </article>
    {:else}
        <section>
            <StatusBar />
            <WebSocketStatus />
            <LogControls />
            <LogViewer />
        </section>
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

    article {
        margin-top: 2rem;
    }

    code {
        background: var(--pico-code-background-color);
        padding: 0.25rem 0.5rem;
        border-radius: 0.25rem;
        font-size: 0.9em;
    }

    .error-box {
        background-color: var(--pico-card-background-color);
        border: 1px solid var(--pico-card-border-color);
        border-left: 4px solid var(--pico-color-red-500);
        padding: 1rem 1.5rem;
        margin-top: 2rem;
        border-radius: var(--pico-border-radius);
    }

    .error-box p {
        margin: 0;
        color: var(--pico-color-red-500);
        font-weight: bold;
    }
</style>

