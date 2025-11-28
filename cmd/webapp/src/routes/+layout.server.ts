import type { LayoutServerLoad } from './$types';

const ENDPOINT_KEY = 'runvoy_endpoint';
const API_KEY_KEY = 'runvoy_api_key';

const parseCookieValue = <T>(value: string | undefined): T | null => {
    if (!value) {
        return null;
    }

    try {
        return JSON.parse(value) as T;
    } catch (error) {
        console.warn('Failed to parse cookie value', error);
        return (value as unknown as T) ?? null;
    }
};

export const load: LayoutServerLoad = ({ cookies }) => {
    const endpointCookie = cookies.get(ENDPOINT_KEY);
    const apiKeyCookie = cookies.get(API_KEY_KEY);

    return {
        initialConfig: {
            endpoint: parseCookieValue<string | null>(endpointCookie),
            apiKey: parseCookieValue<string | null>(apiKeyCookie)
        }
    };
};
