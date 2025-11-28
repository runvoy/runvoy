import { describe, it, expect, beforeEach, vi } from 'vitest';
import { get } from 'svelte/store';
import {
    apiEndpoint,
    apiKey,
    hydrateConfigStores,
    setApiEndpoint,
    setApiKey,
    updateApiEndpoint
} from './config';

vi.mock('$app/environment', () => ({
    browser: true
}));

describe('config persistence helper', () => {
    beforeEach(() => {
        localStorage.clear();
        document.cookie = '';
        setApiEndpoint(null);
        setApiKey(null);
    });

    it('hydrates stores from initial snapshot without browser storage', () => {
        hydrateConfigStores({ endpoint: 'https://cookie.example', apiKey: 'cookie-key' });

        expect(get(apiEndpoint)).toBe('https://cookie.example');
        expect(get(apiKey)).toBe('cookie-key');
    });

    it('serializes values as JSON when persisting', () => {
        setApiEndpoint('https://api.runvoy.test');
        setApiKey('my-key');

        expect(localStorage.getItem('runvoy_endpoint')).toBe(
            JSON.stringify('https://api.runvoy.test')
        );
        expect(localStorage.getItem('runvoy_api_key')).toBe(JSON.stringify('my-key'));
    });

    it('removes persisted entries when cleared', () => {
        setApiEndpoint('https://api.runvoy.test');
        setApiKey('persist-me');

        setApiEndpoint(null);
        setApiKey(null);

        expect(localStorage.getItem('runvoy_endpoint')).toBeNull();
        expect(localStorage.getItem('runvoy_api_key')).toBeNull();
    });

    it('updates values using updater functions', () => {
        hydrateConfigStores({ endpoint: 'https://api.initial' });
        updateApiEndpoint((current) => (current ? `${current}/v1` : null));

        expect(get(apiEndpoint)).toBe('https://api.initial/v1');
    });
});
