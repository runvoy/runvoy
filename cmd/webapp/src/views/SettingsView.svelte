<script lang="ts">
    import { version } from '$app/environment';
    import { apiEndpoint, apiKey } from '../stores/config';
    import APIClient from '../lib/api';
    import type { HealthResponse } from '../types/api';

    interface Props {
        apiClient: APIClient | null;
        isConfigured?: boolean;
    }

    const { apiClient = null, isConfigured = false }: Props = $props();

    const MASKED_API_KEY_PLACEHOLDER = '••••••••';

    let endpointInput = $state($apiEndpoint || '');
    let keyInput = $state('');
    let formError = $state('');
    let formSuccess = $state('');
    let showApiKey = $state(false);
    let backendHealth: HealthResponse | null = $state(null);
    let healthError: string | null = $state(null);
    let loadingHealth = $state(false);

    const displayKey = $derived($apiKey ? MASKED_API_KEY_PLACEHOLDER : '');

    async function fetchBackendHealth(): Promise<void> {
        if (!apiClient) {
            return;
        }

        loadingHealth = true;
        healthError = null;

        try {
            backendHealth = await apiClient.getHealth();
        } catch (error) {
            healthError = error instanceof Error ? error.message : 'Failed to fetch backend health';
        } finally {
            loadingHealth = false;
        }
    }

    $effect.root(() => {
        syncFormFromStore();
        fetchBackendHealth();
    });

    function toggleApiKeyVisibility(): void {
        showApiKey = !showApiKey;
    }

    function clearConfiguration(): void {
        if (
            confirm(
                'Are you sure you want to clear your configuration? You will need to configure the API again.'
            )
        ) {
            apiEndpoint.set(null);
            apiKey.set(null);
            showApiKey = false;
            backendHealth = null;
            healthError = null;
            formSuccess = '';
            formError = '';
            syncFormFromStore();
        }
    }

    function syncFormFromStore(): void {
        endpointInput = $apiEndpoint || '';
        keyInput = '';
    }

    async function saveConfiguration(event: { preventDefault: () => void }): Promise<void> {
        event.preventDefault();
        formError = '';
        formSuccess = '';

        const endpoint = endpointInput.trim();

        if (!endpoint) {
            formError = 'Please enter an endpoint URL';
            return;
        }

        try {
            new URL(endpoint);
        } catch {
            formError = 'Invalid URL format';
            return;
        }

        apiEndpoint.set(endpoint);

        if (keyInput && keyInput !== MASKED_API_KEY_PLACEHOLDER) {
            apiKey.set(keyInput.trim());
        }

        formSuccess = 'Configuration saved';
        keyInput = '';

        window.dispatchEvent(new CustomEvent('credentials-updated'));

        if (apiClient) {
            await fetchBackendHealth();
        }
    }

    function copyToClipboard(text: string | null): void {
        if (!text) return;
        navigator.clipboard.writeText(text).then(() => {
            // Could show a toast notification here
            alert('Copied to clipboard!');
        });
    }

    // Re-fetch health when API configuration changes
    $effect(() => {
        if (apiClient && isConfigured) {
            fetchBackendHealth();
        }
    });
</script>

<article class="settings-card">
    <header>
        <h2>⚙️ Settings & About</h2>
    </header>

    <section class="settings-section">
        <h3>API Configuration</h3>

        {#if formError}
            <div class="alert error" role="alert">{formError}</div>
        {/if}

        {#if formSuccess}
            <div class="alert success" role="status">{formSuccess}</div>
        {/if}

        <form class="config-form" onsubmit={saveConfiguration} novalidate>
            <label for="endpoint-input">
                API Endpoint
                <input
                    id="endpoint-input"
                    type="url"
                    bind:value={endpointInput}
                    placeholder="https://api.runvoy.example.com"
                />
                <small>The base URL of your runvoy API server.</small>
            </label>

            <label for="api-key-input">
                API Key (optional)
                <input
                    id="api-key-input"
                    type="password"
                    bind:value={keyInput}
                    placeholder={displayKey || 'Enter API key (or claim one later)'}
                />
                <small>
                    You can provide your API key now or claim one later with an invitation token.
                </small>
            </label>

            <div class="form-actions">
                <button type="submit">Save configuration</button>
                <button type="button" class="secondary" onclick={syncFormFromStore}>Reset</button>
            </div>
        </form>
    </section>

    <section class="settings-section">
        <h3>Application Information</h3>
        <div class="info-group">
            <div class="label">Application:</div>
            <span class="value">runvoy Webapp</span>
        </div>
        <div class="info-group">
            <div class="label">Webapp Version:</div>
            <span class="value">{version || 'unknown'}</span>
        </div>
        <div class="info-group">
            <div class="label">Backend Version:</div>
            {#if loadingHealth}
                <span class="value muted">Loading...</span>
            {:else if healthError}
                <span class="value error" title={healthError}> Failed to fetch </span>
            {:else if backendHealth}
                <span class="value">{backendHealth.version}</span>
                <span class="status-badge configured" title="Backend is healthy">
                    {backendHealth.status}
                </span>
            {:else}
                <span class="value empty">Not connected</span>
            {/if}
        </div>

        <div class="info-group">
            <div class="label">Backend Provider:</div>
            {#if loadingHealth}
                <span class="value muted">Loading...</span>
            {:else if healthError}
                <span class="value error" title={healthError}> Failed to fetch </span>
            {:else if backendHealth?.provider}
                <span class="value">{backendHealth.provider}</span>
            {:else}
                <span class="value empty">Not connected</span>
            {/if}
        </div>

        <div class="info-group">
            <div class="label">Backend Region:</div>
            {#if loadingHealth}
                <span class="value muted">Loading...</span>
            {:else if healthError}
                <span class="value error" title={healthError}> Failed to fetch </span>
            {:else if backendHealth?.region}
                <span class="value">{backendHealth.region}</span>
            {:else if backendHealth}
                <span class="value muted">Unknown</span>
            {:else}
                <span class="value empty">Not connected</span>
            {/if}
        </div>
    </section>

    <section class="settings-section">
        <h3>Current Configuration</h3>

        <div class="info-group">
            <div class="label">API Endpoint:</div>
            {#if $apiEndpoint}
                <div class="value-with-action">
                    <code class="endpoint">{$apiEndpoint}</code>
                    <button
                        class="icon-button"
                        onclick={() => copyToClipboard($apiEndpoint)}
                        title="Copy endpoint"
                    >
                        📋
                    </button>
                </div>
            {:else}
                <span class="value empty">Not configured</span>
            {/if}
        </div>

        <div class="info-group">
            <div class="label">API Key Status:</div>
            {#if $apiKey}
                <div class="key-status">
                    <span class="status-badge configured">✓ Configured</span>
                    <button
                        class="icon-button"
                        onclick={toggleApiKeyVisibility}
                        title={showApiKey ? 'Hide API key' : 'Show API key'}
                    >
                        {showApiKey ? '🙈' : '🔑'}
                    </button>
                </div>
                {#if showApiKey}
                    <div class="api-key-display">
                        <code class="api-key">{$apiKey}</code>
                        <button
                            class="icon-button"
                            onclick={() => copyToClipboard($apiKey)}
                            title="Copy API key"
                        >
                            📋
                        </button>
                    </div>
                {/if}
            {:else}
                <span class="status-badge unconfigured">✗ Not configured</span>
            {/if}
        </div>

        <div class="storage-info">
            <small
                >💾 Your credentials are stored locally in your browser's localStorage and never
                sent to third parties.</small
            >
        </div>
    </section>

    <section class="settings-section">
        <h3>Browser Storage</h3>
        <div class="storage-stats">
            <p>
                <strong>Endpoint:</strong>
                {$apiEndpoint ? 'Stored ✓' : 'Not stored'}
            </p>
            <p>
                <strong>API Key:</strong>
                {$apiKey ? 'Stored ✓' : 'Not stored'}
            </p>
        </div>
        <button class="secondary" onclick={clearConfiguration}> 🗑️ Clear All Configuration </button>
    </section>

    <section class="settings-section">
        <h3>Links</h3>
        <ul class="links-list">
            <li>
                <a href="https://github.com/runvoy/runvoy" target="_blank" rel="noopener">
                    GitHub Repository
                </a>
            </li>
            <li>
                <a href="https://runvoy.site/" target="_blank" rel="noopener"> Documentation </a>
            </li>
        </ul>
    </section>
</article>

<style>
    .settings-card {
        background: var(--pico-card-background-color);
        border: 1px solid var(--pico-card-border-color);
        border-radius: var(--pico-border-radius);
        padding: 2rem;
    }

    header {
        margin-bottom: 2rem;
        border-bottom: 1px solid var(--pico-border-color);
        padding-bottom: 1rem;
    }

    header h2 {
        margin: 0;
    }

    .settings-section {
        margin-bottom: 2rem;
        padding-bottom: 2rem;
        border-bottom: 1px solid var(--pico-border-color);
    }

    .settings-section:last-of-type {
        border-bottom: none;
        margin-bottom: 0;
        padding-bottom: 0;
    }

    .settings-section h3 {
        margin-top: 0;
        margin-bottom: 1rem;
        color: var(--pico-primary);
        font-size: 1.1rem;
    }

    .config-form {
        display: flex;
        flex-direction: column;
        gap: 1rem;
    }

    .config-form label {
        display: flex;
        flex-direction: column;
        gap: 0.5rem;
    }

    .config-form input {
        padding: 0.75rem;
        border: 1px solid var(--pico-border-color);
        border-radius: var(--pico-border-radius);
        background: var(--pico-form-element-background-color);
    }

    .config-form small {
        color: var(--pico-muted-color);
    }

    .form-actions {
        display: flex;
        gap: 1rem;
        flex-wrap: wrap;
    }

    .alert {
        padding: 0.75rem 1rem;
        border-radius: var(--pico-border-radius);
        margin-bottom: 1rem;
    }

    .alert.error {
        background: #ffebee;
        border: 1px solid #f44336;
        color: #c62828;
    }

    .alert.success {
        background: #e8f5e9;
        border: 1px solid #4caf50;
        color: #2e7d32;
    }

    .info-group {
        display: flex;
        align-items: flex-start;
        gap: 1rem;
        margin-bottom: 1rem;
        padding: 0.75rem;
        background: var(--pico-card-background-color);
        border-radius: var(--pico-border-radius);
    }

    .info-group .label {
        flex: 0 0 120px;
        font-weight: 600;
        color: var(--pico-muted-color);
    }

    .value {
        flex: 1;
        word-break: break-all;
    }

    .value.empty {
        color: var(--pico-muted-color);
        font-style: italic;
    }

    .value.muted {
        color: var(--pico-muted-color);
    }

    .value.error {
        color: #c62828;
        cursor: help;
    }

    .value-with-action {
        flex: 1;
        display: flex;
        align-items: center;
        gap: 0.5rem;
    }

    .endpoint {
        flex: 1;
        background: var(--pico-form-element-background-color);
        padding: 0.5rem;
        border-radius: 0.25rem;
        font-size: 0.85rem;
        word-break: break-all;
    }

    .key-status {
        flex: 1;
        display: flex;
        align-items: center;
        gap: 0.75rem;
    }

    .status-badge {
        display: inline-block;
        padding: 0.25rem 0.75rem;
        border-radius: 1rem;
        font-size: 0.85rem;
        font-weight: 600;
    }

    .status-badge.configured {
        background-color: #e8f5e9;
        color: #2e7d32;
    }

    .status-badge.unconfigured {
        background-color: #ffebee;
        color: #c62828;
    }

    .api-key-display {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        margin-top: 0.5rem;
        padding: 0.75rem;
        background: var(--pico-form-element-background-color);
        border-radius: var(--pico-border-radius);
    }

    .api-key {
        flex: 1;
        font-family: 'Monaco', 'Courier New', monospace;
        font-size: 0.8rem;
        word-break: break-all;
    }

    .icon-button {
        border: none;
        background: none;
        cursor: pointer;
        font-size: 1.2rem;
        padding: 0.25rem 0.5rem;
        line-height: 1;
    }

    .icon-button:hover {
        opacity: 0.7;
    }

    .storage-info {
        background: #e3f2fd;
        border: 1px solid #90caf9;
        color: #1565c0;
        padding: 0.75rem;
        border-radius: var(--pico-border-radius);
        margin-top: 1rem;
    }

    .storage-info small {
        display: block;
    }

    .storage-stats {
        background: var(--pico-card-background-color);
        padding: 1rem;
        border-radius: var(--pico-border-radius);
        margin-bottom: 1rem;
    }

    .storage-stats p {
        margin: 0.5rem 0;
        font-size: 0.9rem;
    }

    .links-list {
        list-style: none;
        padding: 0;
        margin: 0;
    }

    .links-list li {
        margin-bottom: 0.5rem;
    }

    .links-list a {
        color: var(--pico-primary);
        text-decoration: none;
        display: inline-flex;
        align-items: center;
        gap: 0.25rem;
    }

    .links-list a:hover {
        text-decoration: underline;
    }

    @media (max-width: 768px) {
        .settings-card {
            padding: 1.5rem;
        }

        .info-group {
            flex-direction: column;
            align-items: flex-start;
        }

        .info-group .label {
            flex: 1 1 auto;
        }
    }
</style>
