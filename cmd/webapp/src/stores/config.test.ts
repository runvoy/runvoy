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

describe('config persistence', () => {
    beforeEach(() => {
        localStorage.clear();
        setApiEndpoint(null);
        setApiKey(null);
    });

    it('hydrates stores from localStorage', () => {
        localStorage.setItem('runvoy_endpoint', JSON.stringify('https://stored.example'));
        localStorage.setItem('runvoy_api_key', JSON.stringify('stored-key'));

        hydrateConfigStores();

        expect(get(apiEndpoint)).toBe('https://stored.example');
        expect(get(apiKey)).toBe('stored-key');
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
        setApiEndpoint('https://api.initial');
        updateApiEndpoint((current) => (current ? `${current}/v1` : null));

        expect(get(apiEndpoint)).toBe('https://api.initial/v1');
    });

    it('returns current values from hydrateConfigStores', () => {
        localStorage.setItem('runvoy_endpoint', JSON.stringify('https://hydrated.example'));
        localStorage.setItem('runvoy_api_key', JSON.stringify('hydrated-key'));

        const result = hydrateConfigStores();

        expect(result.endpoint).toBe('https://hydrated.example');
        expect(result.apiKey).toBe('hydrated-key');
    });
});
