<script>
    import { activeView } from '../stores/ui.js';

    export let views = [];

    function selectView(view) {
        if (view.disabled) {
            return;
        }
        activeView.set(view.id);
    }
</script>

<nav class="view-switcher" aria-label="View selection">
    {#each views as view (view.id)}
        <button
            type="button"
            class:active={$activeView === view.id}
            class:disabled={view.disabled}
            disabled={view.disabled}
            on:click={() => selectView(view)}
        >
            {view.label}
        </button>
    {/each}
</nav>

<style>
    .view-switcher {
        display: flex;
        gap: 0.75rem;
        margin-bottom: 1.5rem;
        flex-wrap: wrap;
        align-items: center;
    }

    button {
        border-radius: 999px;
        padding: 0.35rem 1.25rem;
        border: 1px solid var(--pico-border-color);
        background: transparent;
        color: inherit;
        cursor: pointer;
        font-weight: 600;
        transition:
            background-color 0.15s ease,
            color 0.15s ease,
            border-color 0.15s ease;
    }

    button:hover {
        border-color: var(--pico-primary);
        color: var(--pico-primary);
    }

    button.active {
        background: var(--pico-primary);
        color: #fff;
        border-color: var(--pico-primary);
    }

    button.disabled {
        opacity: 0.6;
        cursor: not-allowed;
        border-style: dashed;
    }

    button.disabled:hover {
        border-color: var(--pico-border-color);
        color: inherit;
    }

    button:focus-visible {
        outline: 2px solid var(--pico-primary);
        outline-offset: 2px;
    }
</style>
