import { redirect } from '@sveltejs/kit';
import type { LayoutLoad } from './$types';

const ENDPOINT_KEY = 'runvoy_endpoint';
const API_KEY_KEY = 'runvoy_api_key';

export const prerender = false;

export const load: LayoutLoad = ({ cookies, url }) => {
    let endpoint = cookies.get(ENDPOINT_KEY);
    let apiKey = cookies.get(API_KEY_KEY);

    if (typeof localStorage !== 'undefined') {
        endpoint = endpoint ?? localStorage.getItem(ENDPOINT_KEY);
        apiKey = apiKey ?? localStorage.getItem(API_KEY_KEY);
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
