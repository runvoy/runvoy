import { browser } from '$app/environment';
import { redirect } from '@sveltejs/kit';
import { hydrateConfigStores } from '../stores/config';
import type { LayoutLoad } from './$types';

export const prerender = false;

export const load: LayoutLoad = ({ url, data }) => {
    const hydrated = hydrateConfigStores(data?.initialConfig);

    let endpoint = hydrated.endpoint;
    let apiKey = hydrated.apiKey;

    if (browser) {
        // Prefer latest browser values if available
        endpoint = hydrated.endpoint ?? endpoint;
        apiKey = hydrated.apiKey ?? apiKey;
    }

    const hasEndpoint = Boolean(endpoint);
    const hasApiKey = Boolean(apiKey);

    if (!hasEndpoint && url.pathname !== '/settings') {
        throw redirect(307, '/settings');
    }

    if (hasEndpoint && !hasApiKey && !['/claim', '/settings'].includes(url.pathname)) {
        throw redirect(307, '/claim');
    }

    return {
        hasEndpoint,
        hasApiKey
    };
};
