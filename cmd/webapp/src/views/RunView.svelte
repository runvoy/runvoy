<script lang="ts">
    import { goto } from '$app/navigation';
    import type APIClient from '../lib/api';
    import type { RunCommandPayload } from '../types/api';
    import type { EnvRow } from '../types/run';
    import { switchExecution } from '../lib/executionState';
    import { cachedWebSocketURL } from '../stores/websocket';

    export let apiClient: APIClient | null = null;

    let command = '';
    let image = '';
    let timeout = '';
    let gitRepo = '';
    let gitRef = '';
    let gitPath = '';
    let envRows: EnvRow[] = [];
    let showAdvanced = false;
    let isSubmitting = false;
    let errorMessage = '';

    let envRowCounter = 0;

    function createEnvRow(): EnvRow {
        envRowCounter += 1;
        return { id: envRowCounter, key: '', value: '' };
    }

    $: if (envRows.length === 0) {
        envRows = [createEnvRow()];
    }

    function addEnvRow(): void {
        envRows = [...envRows, createEnvRow()];
    }

    function updateEnvRow(id: number, field: string, value: string): void {
        envRows = envRows.map((row) => (row.id === id ? { ...row, [field]: value } : row));
    }

    function removeEnvRow(id: number): void {
        if (envRows.length === 1) {
            envRows = [createEnvRow()];
            return;
        }
        envRows = envRows.filter((row) => row.id !== id);
    }

    function buildEnvObject(): Record<string, string> {
        return envRows.reduce(
            (acc, row) => {
                const key = row.key.trim();
                if (!key) {
                    return acc;
                }
                acc[key] = row.value;
                return acc;
            },
            {} as Record<string, string>
        );
    }

    function buildPayload(): RunCommandPayload {
        const payload: RunCommandPayload = {
            command: command.trim()
        };

        if (image.trim()) {
            payload.image = image.trim();
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

    async function handleSubmit(): Promise<void> {
        errorMessage = '';

        if (!apiClient) {
            errorMessage = 'API client is not available.';
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
            const executionId = response.execution_id;
            const websocketURL = response.websocket_url;
            if (executionId) {
                switchExecution(executionId, { updateHistory: false });
                cachedWebSocketURL.set(websocketURL || null);
                goto(`/logs?execution_id=${encodeURIComponent(executionId)}`);
            }
        } catch (error) {
            const err = error as any;
            errorMessage = err.details?.error || err.message || 'Failed to start command';
        } finally {
            isSubmitting = false;
        }
    }
</script>

<article class="run-card">
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
                                    on:input={(event) => {
                                        const target = event.currentTarget;
                                        if (target) {
                                            updateEnvRow(row.id, 'key', target.value);
                                        }
                                    }}
                                />
                                <input
                                    type="text"
                                    placeholder="value"
                                    value={row.value}
                                    on:input={(event) => {
                                        const target = event.currentTarget;
                                        if (target) {
                                            updateEnvRow(row.id, 'value', target.value);
                                        }
                                    }}
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
</article>

<style>
    .run-card {
        background: var(--pico-card-background-color);
        border: 1px solid var(--pico-card-border-color);
        border-radius: var(--pico-border-radius);
        padding: 2rem;
    }

    .run-form {
        display: flex;
        flex-direction: column;
        gap: 1.5rem;
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

    @media (max-width: 768px) {
        .run-card {
            padding: 1.5rem;
        }

        .run-form {
            gap: 1rem;
        }

        fieldset {
            padding: 1rem;
        }

        .advanced-grid {
            grid-template-columns: 1fr;
        }

        .env-row {
            grid-template-columns: 1fr 1fr auto;
            gap: 0.375rem;
        }

        .actions {
            justify-content: stretch;
        }

        .actions button {
            width: 100%;
        }
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
