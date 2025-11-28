import { redirect } from '@sveltejs/kit';
import { hydrateConfigStores } from '../stores/config';
import type { LayoutLoad } from './$types';

export const prerender = false;

export const load: LayoutLoad = ({ url }) => {
    const { endpoint, apiKey } = hydrateConfigStores();

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
