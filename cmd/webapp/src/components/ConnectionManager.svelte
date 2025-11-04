<script>
    import { apiEndpoint, apiKey } from '../stores/config.js';

    let showModal = false;
    let endpointInput = $apiEndpoint || '';
    let keyInput = '';
    let errorMessage = '';

    // Show masked key if already set
    $: displayKey = $apiKey ? '••••••••' : '';

    function openModal() {
        showModal = true;
        endpointInput = $apiEndpoint || '';
        keyInput = '';
        errorMessage = '';
    }

    function closeModal() {
        showModal = false;
        errorMessage = '';
    }

    function saveCredentials() {
        const endpoint = endpointInput.trim();
        const key = keyInput;

        if (!endpoint) {
            errorMessage = 'Please enter an endpoint URL';
            return;
        }

        if (!key || key === '••••••••') {
            errorMessage = 'Please enter a valid API key';
            return;
        }

        // Validate URL format
        try {
            new URL(endpoint);
        } catch {
            errorMessage = 'Invalid URL format';
            return;
        }

        // Save to stores (automatically persists to localStorage)
        apiEndpoint.set(endpoint);
        apiKey.set(key);

        closeModal();

        // Trigger a custom event so parent can reload data
        window.dispatchEvent(new CustomEvent('credentials-updated'));
    }

    function handleModalClick(event) {
        // Close modal if clicking the backdrop
        if (event.target.classList.contains('modal-backdrop')) {
            closeModal();
        }
    }
</script>

<button on:click={openModal} class="config-button">
    ⚙️ Configure API
</button>

{#if showModal}
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="modal-backdrop" on:click={handleModalClick}>
        <div class="modal-content">
            <h3>API Configuration</h3>

            {#if errorMessage}
                <div class="error-message" role="alert">
                    {errorMessage}
                </div>
            {/if}

            <form on:submit|preventDefault={saveCredentials}>
                <label for="endpoint-input">
                    API Endpoint:
                    <input
                        id="endpoint-input"
                        type="text"
                        bind:value={endpointInput}
                        placeholder="https://api.runvoy.example.com"
                        required
                    />
                    <small>The base URL of your runvoy API server</small>
                </label>

                <label for="api-key-input">
                    API Key:
                    <input
                        id="api-key-input"
                        type="password"
                        bind:value={keyInput}
                        placeholder={displayKey || 'Enter API key'}
                        required
                    />
                    <small>Your runvoy API key for authentication</small>
                </label>

                <div class="button-group">
                    <button type="submit">Save</button>
                    <button type="button" on:click={closeModal} class="secondary">Cancel</button>
                </div>
            </form>

            <div class="current-config">
                <h4>Current Configuration</h4>
                <p><strong>Endpoint:</strong> {$apiEndpoint || 'Not configured'}</p>
                <p><strong>API Key:</strong> {displayKey || 'Not configured'}</p>
            </div>
        </div>
    </div>
{/if}

<style>
    .config-button {
        position: fixed;
        top: 1rem;
        right: 1rem;
        z-index: 100;
    }

    .modal-backdrop {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background: rgba(0, 0, 0, 0.5);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 1000;
    }

    .modal-content {
        background: var(--pico-background-color);
        padding: 2rem;
        border-radius: var(--pico-border-radius);
        max-width: 500px;
        width: 90%;
        max-height: 80vh;
        overflow-y: auto;
        box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
    }

    .modal-content h3 {
        margin-top: 0;
        margin-bottom: 1.5rem;
    }

    .error-message {
        background: #f44336;
        color: white;
        padding: 0.75rem;
        border-radius: var(--pico-border-radius);
        margin-bottom: 1rem;
    }

    .button-group {
        display: flex;
        gap: 1rem;
        margin-top: 1.5rem;
    }

    .button-group button {
        flex: 1;
    }

    .current-config {
        margin-top: 2rem;
        padding-top: 1.5rem;
        border-top: 1px solid var(--pico-border-color);
    }

    .current-config h4 {
        margin-top: 0;
        margin-bottom: 1rem;
        font-size: 0.9rem;
        text-transform: uppercase;
        color: var(--pico-muted-color);
    }

    .current-config p {
        margin: 0.5rem 0;
        font-size: 0.9rem;
        word-break: break-all;
    }

    label small {
        display: block;
        margin-top: 0.25rem;
        color: var(--pico-muted-color);
    }
</style>
