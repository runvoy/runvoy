<script lang="ts">
    import { page } from '$app/state';

    interface View {
        id: string;
        label: string;
        disabled?: boolean;
    }

    interface Props {
        views: View[];
    }

    const { views = [] }: Props = $props();

    // Map view IDs to routes
    const viewRoutes: Record<string, string> = {
        run: '/',
        logs: '/logs',
        list: '/executions',
        claim: '/claim',
        settings: '/settings'
    };

    function getViewRoute(viewId: string): string {
        return viewRoutes[viewId] || '/';
    }

    function isActive(viewId: string, pathname: string): boolean {
        const route = viewRoutes[viewId];
        if (!route) return false;

        // Check if current path matches the route
        if (route === '/') {
            return pathname === '/';
        }
        return pathname.startsWith(route);
    }

    const currentPathname = $derived(page.url.pathname);
</script>

<nav class="view-switcher" aria-label="View selection">
    {#each views as view (view.id)}
        {@const active = isActive(view.id, currentPathname)}
        <a
            href={getViewRoute(view.id)}
            class:active
            class:disabled={view.disabled}
            aria-disabled={view.disabled}
            aria-current={active ? 'page' : undefined}
            onclick={(e) => {
                if (view.disabled) {
                    e.preventDefault();
                }
            }}
        >
            {view.label}
        </a>
    {/each}
</nav>

<style>
    .view-switcher {
        display: flex;
        gap: 0.5rem;
        flex-wrap: wrap;
        align-items: center;
    }

    a {
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
        text-decoration: none;
        display: inline-block;
    }

    a:hover:not(.disabled):not(.active) {
        border-color: var(--pico-primary);
        color: var(--pico-primary);
        background: var(--pico-primary-background);
    }

    a.active {
        background: var(--pico-primary);
        color: var(--pico-primary-inverse);
        border-color: var(--pico-primary);
    }

    a.disabled {
        opacity: 0.5;
        cursor: not-allowed;
        border-style: dashed;
        pointer-events: none;
    }

    a.disabled:hover {
        border-color: var(--pico-border-color);
        color: inherit;
        background: transparent;
    }

    a:focus-visible {
        outline: 2px solid var(--pico-primary);
        outline-offset: 2px;
    }

    @media (max-width: 768px) {
        .view-switcher {
            gap: 0.375rem;
        }

        a {
            padding: 0.45rem 0.875rem;
            font-size: 0.875rem;
        }
    }
</style>
