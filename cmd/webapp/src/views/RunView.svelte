<script>
    import { activeView, VIEWS } from '../stores/ui.js';
    import { switchExecution } from '../lib/executionState.js';

    export let apiClient = null;
    export let isConfigured = false;

    let command = '';
    let image = '';
    let lock = '';
    let timeout = '';
    let gitRepo = '';
    let gitRef = '';
    let gitPath = '';
    let envRows = [];
    let showAdvanced = false;
    let isSubmitting = false;
    let errorMessage = '';

    let envRowCounter = 0;

    function createEnvRow() {
        envRowCounter += 1;
        return { id: envRowCounter, key: '', value: '' };
    }

    $: if (envRows.length === 0) {
        envRows = [createEnvRow()];
    }

    function addEnvRow() {
        envRows = [...envRows, createEnvRow()];
    }

    function updateEnvRow(id, field, value) {
        envRows = envRows.map((row) => (row.id === id ? { ...row, [field]: value } : row));
    }

    function removeEnvRow(id) {
        if (envRows.length === 1) {
            envRows = [createEnvRow()];
            return;
        }
        envRows = envRows.filter((row) => row.id !== id);
    }

    function buildEnvObject() {
        return envRows.reduce((acc, row) => {
            const key = row.key.trim();
            if (!key) {
                return acc;
            }
            acc[key] = row.value;
            return acc;
        }, {});
    }

    function buildPayload() {
        const payload = {
            command: command.trim()
        };

        if (image.trim()) {
            payload.image = image.trim();
        }

        if (lock.trim()) {
            payload.lock = lock.trim();
        }

        const parsedTimeout = Number.parseInt(timeout, 10);
        if (!Number.isNaN(parsedTimeout) && parsedTimeout > 0) {
            payload.timeout = parsedTimeout;
        }

        const env = buildEnvObject();
        if (Object.keys(env).length > 0) {
            payload.env = env;
        }

        if (gitRepo.trim()) {
            payload.git_repo = gitRepo.trim();
            if (gitRef.trim()) {
                payload.git_ref = gitRef.trim();
            }
            if (gitPath.trim()) {
                payload.git_path = gitPath.trim();
            }
        }

        return payload;
    }

    async function handleSubmit() {
        errorMessage = '';

        if (!isConfigured || !apiClient) {
            errorMessage = 'Configure the API endpoint and key before running commands.';
            return;
        }

        if (!command.trim()) {
            errorMessage = 'Command is required.';
            return;
        }

        isSubmitting = true;
        try {
            const payload = buildPayload();
            const response = await apiClient.runCommand(payload);
            switchExecution(response.execution_id);
            activeView.set(VIEWS.LOGS);
        } catch (error) {
            errorMessage = error.details?.error || error.message || 'Failed to start command';
        } finally {
            isSubmitting = false;
        }
    }
</script>

{#if !isConfigured}
    <article class="info-card">
        <h2>Configure API access to run commands</h2>
        <p>
            Use the <strong>⚙️ Configure API</strong> button to set the endpoint and API key for your runvoy
            backend. Once configured, you can launch commands directly from this view.
        </p>
    </article>
{:else}
    <form on:submit|preventDefault={handleSubmit} class="run-form">
        <fieldset>
            <legend>Command</legend>
            <label for="command-input">
                Command to execute
                <textarea
                    id="command-input"
                    rows="4"
                    placeholder="e.g. make test"
                    bind:value={command}
                    required
                    spellcheck="false"
                ></textarea>
            </label>
        </fieldset>

        <button
            class="link-button"
            type="button"
            on:click={() => (showAdvanced = !showAdvanced)}
            aria-expanded={showAdvanced}
        >
            {showAdvanced ? 'Hide advanced options' : 'Show advanced options'}
        </button>

        {#if showAdvanced}
            <fieldset class="advanced-grid">
                <legend>Execution options</legend>
                <label for="image-input">
                    Docker image
                    <input
                        id="image-input"
                        type="text"
                        placeholder="Optional image override"
                        bind:value={image}
                    />
                </label>

                <label for="lock-input">
                    Lock name
                    <input
                        id="lock-input"
                        type="text"
                        placeholder="Optional lock name"
                        bind:value={lock}
                    />
                </label>

                <label for="timeout-input">
                    Timeout (seconds)
                    <input
                        id="timeout-input"
                        type="number"
                        min="1"
                        inputmode="numeric"
                        placeholder="Optional"
                        bind:value={timeout}
                    />
                </label>
            </fieldset>

            <fieldset class="advanced-grid">
                <legend>Git repository (optional)</legend>
                <label for="git-repo-input">
                    Repository URL
                    <input
                        id="git-repo-input"
                        type="url"
                        placeholder="https://github.com/runvoy/runvoy"
                        bind:value={gitRepo}
                    />
                </label>

                <label for="git-ref-input">
                    Git ref
                    <input
                        id="git-ref-input"
                        type="text"
                        placeholder="main"
                        bind:value={gitRef}
                    />
                </label>

                <label for="git-path-input">
                    Working directory
                    <input
                        id="git-path-input"
                        type="text"
                        placeholder="."
                        bind:value={gitPath}
                    />
                </label>
            </fieldset>

            <fieldset>
                <legend>Environment variables</legend>
                <div class="env-grid">
                    {#each envRows as row (row.id)}
                        <div class="env-row">
                            <input
                                type="text"
                                placeholder="KEY"
                                value={row.key}
                                on:input={(event) => updateEnvRow(row.id, 'key', event.target.value)}
                            />
                            <input
                                type="text"
                                placeholder="value"
                                value={row.value}
                                on:input={(event) => updateEnvRow(row.id, 'value', event.target.value)}
                            />
                            <button
                                type="button"
                                class="icon-button"
                                on:click={() => removeEnvRow(row.id)}
                                aria-label="Remove environment variable"
                            >
                                ✕
                            </button>
                        </div>
                    {/each}
                </div>
                <button type="button" class="secondary" on:click={addEnvRow}>
                    Add environment variable
                </button>
            </fieldset>
        {/if}

        {#if errorMessage}
            <p class="error-text" role="alert">{errorMessage}</p>
        {/if}

        <div class="actions">
            <button type="submit" disabled={isSubmitting}>
                {isSubmitting ? 'Starting...' : 'Run command'}
            </button>
        </div>
    </form>
{/if}

<style>
    .info-card {
        background: var(--pico-card-background-color);
        border: 1px solid var(--pico-card-border-color);
        border-radius: var(--pico-border-radius);
        padding: 1.5rem;
        max-width: 600px;
    }

    .run-form {
        display: flex;
        flex-direction: column;
        gap: 1.5rem;
        max-width: 720px;
    }

    fieldset {
        border: 1px solid var(--pico-border-color);
        border-radius: var(--pico-border-radius);
        padding: 1.25rem;
    }

    legend {
        padding: 0 0.5rem;
        font-weight: 600;
    }

    textarea {
        font-family: 'Monaco', 'Courier New', monospace;
        min-height: 6rem;
    }

    label {
        display: flex;
        flex-direction: column;
        gap: 0.5rem;
        font-weight: 500;
    }

    .advanced-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
        gap: 1rem;
    }

    .env-grid {
        display: flex;
        flex-direction: column;
        gap: 0.75rem;
    }

    .env-row {
        display: grid;
        grid-template-columns: minmax(120px, 2fr) minmax(120px, 2fr) auto;
        gap: 0.5rem;
        align-items: center;
    }

    .actions {
        display: flex;
        justify-content: flex-end;
    }

    .link-button {
        align-self: flex-start;
        border: none;
        background: none;
        color: var(--pico-primary);
        cursor: pointer;
        font-weight: 600;
        padding: 0;
    }

    .link-button:hover {
        text-decoration: underline;
    }

    .icon-button {
        border: none;
        background: none;
        cursor: pointer;
        font-size: 1.1rem;
        line-height: 1;
        padding: 0.2rem 0.4rem;
    }

    .icon-button:hover {
        color: var(--pico-color-red-500);
    }

    .error-text {
        color: var(--pico-color-red-500);
        font-weight: 600;
        margin: 0;
    }
</style>
