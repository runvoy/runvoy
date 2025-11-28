import { redirect } from '@sveltejs/kit';
import { hydrateConfigStores, parsePersistedValue } from '../stores/config';
import { validateApiConfiguration } from '../lib/apiConfig';
import type { LayoutLoad } from './$types';

export const prerender = false;

export const load: LayoutLoad = ({ url, cookies }) => {
    const safeCookies = cookies ?? {
        get: () => '',
        set: () => {},
        delete: () => {},
        serialize: () => ''
    };

    const endpointCookie = safeCookies.get('runvoy_endpoint');
    const apiKeyCookie = safeCookies.get('runvoy_api_key');

    const { endpoint, apiKey } = hydrateConfigStores({
        endpoint: parsePersistedValue<string>(endpointCookie ? decodeURIComponent(endpointCookie) : null),
        apiKey: parsePersistedValue<string>(apiKeyCookie ? decodeURIComponent(apiKeyCookie) : null)
    });

    const validated = validateApiConfiguration(
        {
            endpoint,
            apiKey
        },
        { requireApiKey: false }
    );

    const hasEndpoint = Boolean(validated?.endpoint);
    const hasApiKey = Boolean(validateApiConfiguration({ endpoint, apiKey }));

    if (!hasEndpoint && url.pathname !== '/settings') {
        throw redirect(307, '/settings');
    }

    if (hasEndpoint && !hasApiKey && !['/claim', '/settings'].includes(url.pathname)) {
        throw redirect(307, '/claim');
    }

    return {
        hasEndpoint,
        hasApiKey,
        endpoint: validated?.endpoint ?? null,
        apiKey: validated?.apiKey ?? null
    };
};
