<script lang="ts">
    import { version, browser } from '$app/environment';
    import { page } from '$app/state';
    import { goto } from '$app/navigation';
    import ViewSwitcher from '../components/ViewSwitcher.svelte';
    import { hydrateConfigStores } from '../stores/config';
    import { hasEndpoint, hasApiKey } from '../stores/apiClient';
    import { VIEWS, NAV_VIEWS } from '$lib/constants';
    import type { Snippet } from 'svelte';

    import '../styles/global.css';

    interface Props {
        children?: Snippet;
    }

    const { children }: Props = $props();

    // Hydrate stores once on client mount (reads from localStorage)
    if (browser) {
        hydrateConfigStores();
    }

    // Single centralized redirect logic
    $effect(() => {
        if (!browser) return;

        const path = page.url.pathname;

        if (!$hasEndpoint && path !== '/settings') {
            goto('/settings', { replaceState: true });
            return;
        }

        if ($hasEndpoint && !$hasApiKey && !['/claim', '/settings'].includes(path)) {
            goto('/claim', { replaceState: true });
        }
    });

    const navViews = $derived(
        NAV_VIEWS.map((view) => {
            if (!$hasEndpoint && view.id !== VIEWS.SETTINGS) {
                return { ...view, disabled: true };
            }

            if (view.id === VIEWS.LOGS || view.id === VIEWS.LIST) {
                return { ...view, disabled: !$hasApiKey };
            }

            return view;
        })
    );
</script>

<main class="container">
    <header class="app-bar">
        <div class="brand">
            <img src="/runvoy-avatar.png" alt="runvoy logo" class="avatar" />
            <div class="brand-text">
                <p class="brand-name">runvoy</p>
                <div class="meta">
                    {#if version}
                        <span class="version">{version}</span>
                    {/if}
                    <a class="docs-link" href="https://runvoy.site/" target="_blank" rel="noopener">
                        Documentation
                    </a>
                </div>
            </div>
        </div>
        <div class="header-nav">
            <ViewSwitcher views={navViews} />
        </div>
    </header>

    <div class="content-area">
        {#if children}
            {@render children()}
        {/if}
    </div>
</main>

<style>
    /* Pico's .container class on main element handles max-width and centering */

    .app-bar {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 1rem;
        padding: 0.75rem 0;
        border-bottom: 1px solid var(--pico-border-color);
        position: sticky;
        top: 0;
        background: var(--pico-background-color);
        z-index: 5;
    }

    .brand {
        display: flex;
        align-items: center;
        gap: 0.75rem;
        min-width: 0;
    }

    .brand-text {
        display: flex;
        flex-direction: column;
        gap: 0.25rem;
    }

    .brand-name {
        margin: 0;
        font-weight: 700;
        font-size: 1.05rem;
        letter-spacing: 0.01em;
    }

    .meta {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        flex-wrap: wrap;
    }

    .avatar {
        height: 2.5rem;
        width: 2.5rem;
        object-fit: contain;
    }

    .version {
        color: var(--pico-muted-color);
        font-size: 0.75rem;
        border: 1px solid var(--pico-border-color);
        border-radius: 999px;
        padding: 0.2rem 0.65rem;
        background: var(--pico-primary-background);
    }

    .docs-link {
        color: var(--pico-muted-color);
        text-decoration: none;
        font-size: 0.875rem;
    }

    .docs-link:hover {
        color: var(--pico-primary);
        text-decoration: underline;
    }

    .header-nav {
        display: flex;
        justify-content: flex-end;
        width: min(640px, 100%);
    }

    .content-area {
        min-height: 400px;
        padding-top: 0.5rem;
    }

    @media (max-width: 768px) {
        .app-bar {
            flex-wrap: wrap;
            padding: 0.5rem 0;
            gap: 0.75rem;
        }

        .header-nav {
            width: 100%;
            justify-content: flex-start;
        }
    }
</style>
