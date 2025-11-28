<script lang="ts">
    import { apiEndpoint, setApiKey } from '../stores/config';
    import type { ClaimAPIKeyResponse } from '../types/api';
    import APIClient from '../lib/api';

    interface Props {
        apiClient: APIClient | null;
        isConfigured?: boolean;
    }

    const { apiClient = null, isConfigured = false }: Props = $props();

    let token = $state('');
    let isLoading = $state(false);
    let error = $state('');
    let success = $state(false);
    let claimResult: ClaimAPIKeyResponse | null = $state(null);

    // Check if endpoint is available (claim only needs endpoint, not API key)
    // If isConfigured is true, we definitely have endpoint; otherwise check directly
    const hasEndpoint = $derived(isConfigured || Boolean(apiClient?.endpoint || $apiEndpoint));

    async function handleClaim(): Promise<void> {
        error = '';
        success = false;
        claimResult = null;

        if (!token.trim()) {
            error = 'Please enter an invitation token';
            return;
        }

        const endpoint = apiClient?.endpoint || $apiEndpoint;
        if (!endpoint) {
            error = 'Please configure the API endpoint first';
            return;
        }

        isLoading = true;
        try {
            // Create a temporary client without auth for the claim endpoint
            const tempClient = new APIClient(endpoint, '');
            const result = await tempClient.claimAPIKey(token.trim());
            claimResult = result;
            success = true;

            // Auto-save the API key
            setApiKey(result.api_key);

            // Clear the token field
            token = '';
        } catch (err: any) {
            error = err.details?.error || err.message || 'Failed to claim API key';
        } finally {
            isLoading = false;
        }
    }

    function handleKeyPress(event: KeyboardEvent): void {
        if (event.key === 'Enter' && !isLoading) {
            handleClaim();
        }
    }
</script>

<article class="claim-card">
    <header>
        <h2>🔑 Claim API Key</h2>
        <p>Use your invitation token to claim a new API key</p>
    </header>

    {#if success && claimResult}
        <div class="success-message" role="status">
            <h3>✅ API Key Claimed!</h3>
            <p>
                <strong>Email:</strong>
                {claimResult.user_email}
            </p>
            {#if claimResult.message}
                <p><strong>Message:</strong> {claimResult.message}</p>
            {/if}
            <p class="note">Your API key has been automatically saved to the configuration.</p>
        </div>
    {:else}
        <div class="form-container">
            {#if error}
                <div class="error-message" role="alert">
                    {error}
                </div>
            {/if}

            <label for="token-input">
                <strong>Invitation Token:</strong>
                <textarea
                    id="token-input"
                    bind:value={token}
                    onkeypress={handleKeyPress}
                    placeholder="Paste your invitation token here..."
                    rows="4"
                    disabled={isLoading}
                ></textarea>
            </label>

            <div class="actions">
                <button onclick={handleClaim} disabled={isLoading || !token.trim() || !hasEndpoint}>
                    {isLoading ? 'Claiming...' : 'Claim Key'}
                </button>
            </div>

            <div class="info-box">
                <p>
                    <strong>ℹ️ How it works:</strong> Paste the invitation token you received from your
                    administrator. We'll exchange it for an API key that will be saved to your local browser
                    storage.
                </p>
            </div>
        </div>
    {/if}
</article>

<style>
    .claim-card {
        background: var(--pico-card-background-color);
        border: 1px solid var(--pico-card-border-color);
        border-radius: var(--pico-border-radius);
        padding: 2rem;
    }

    header {
        margin-bottom: 1.5rem;
    }

    header h2 {
        margin: 0 0 0.5rem 0;
    }

    header p {
        margin: 0;
        color: var(--pico-muted-color);
    }

    .form-container {
        display: flex;
        flex-direction: column;
        gap: 1.5rem;
    }

    label {
        display: flex;
        flex-direction: column;
        gap: 0.5rem;
    }

    label strong {
        font-weight: 600;
    }

    textarea {
        font-family: 'Monaco', 'Courier New', monospace;
        font-size: 0.9rem;
        padding: 0.75rem;
        border: 1px solid var(--pico-border-color);
        border-radius: var(--pico-border-radius);
        background: var(--pico-form-element-background-color);
        color: inherit;
    }

    textarea:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .error-message {
        background-color: #ffebee;
        border: 1px solid #f44336;
        color: #c62828;
        padding: 1rem;
        border-radius: var(--pico-border-radius);
    }

    .success-message {
        background-color: #e8f5e9;
        border: 1px solid #4caf50;
        color: #2e7d32;
        padding: 1rem;
        border-radius: var(--pico-border-radius);
    }

    .success-message h3 {
        margin-top: 0;
        margin-bottom: 0.5rem;
    }

    .success-message p {
        margin: 0.5rem 0;
    }

    .success-message .note {
        font-size: 0.9rem;
        color: #1b5e20;
        margin-top: 1rem;
        padding-top: 1rem;
        border-top: 1px solid #4caf50;
    }

    .actions {
        display: flex;
        gap: 1rem;
    }

    button {
        flex: 1;
        padding: 0.75rem 1rem;
        font-weight: 600;
    }

    button:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .info-box {
        background-color: var(--pico-card-background-color);
        border: 1px solid var(--pico-border-color);
        border-radius: var(--pico-border-radius);
        padding: 1rem;
        font-size: 0.9rem;
    }

    .info-box p {
        margin: 0;
        color: var(--pico-muted-color);
    }

    @media (max-width: 768px) {
        .claim-card {
            padding: 1.5rem;
        }

        textarea {
            font-size: 0.85rem;
        }
    }
</style>
