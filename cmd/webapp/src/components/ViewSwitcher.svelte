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
        gap: 0.5rem;
        flex-wrap: wrap;
        align-items: center;
    }

    button {
        border-radius: var(--pico-border-radius);
        padding: 0.5rem 1rem;
        border: 1px solid var(--pico-border-color);
        background: transparent;
        color: inherit;
        cursor: pointer;
        font-weight: 500;
        font-size: 0.9375rem;
        transition:
            background-color 0.15s ease,
            color 0.15s ease,
            border-color 0.15s ease;
        white-space: nowrap;
    }

    button:hover:not(.disabled):not(.active) {
        border-color: var(--pico-primary);
        color: var(--pico-primary);
        background: var(--pico-primary-background);
    }

    button.active {
        background: var(--pico-primary);
        color: var(--pico-primary-inverse);
        border-color: var(--pico-primary);
    }

    button.disabled {
        opacity: 0.5;
        cursor: not-allowed;
        border-style: dashed;
    }

    button.disabled:hover {
        border-color: var(--pico-border-color);
        color: inherit;
        background: transparent;
    }

    button:focus-visible {
        outline: 2px solid var(--pico-primary);
        outline-offset: 2px;
    }

    @media (max-width: 768px) {
        .view-switcher {
            gap: 0.375rem;
        }

        button {
            padding: 0.45rem 0.875rem;
            font-size: 0.875rem;
        }
    }
</style>
