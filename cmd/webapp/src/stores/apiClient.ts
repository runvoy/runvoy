import { derived, type Readable } from 'svelte/store';
import { browser } from '$app/environment';
import { apiEndpoint, apiKey } from './config';
import APIClient from '../lib/api';

/**
 * Singleton API client store - automatically recreates when config changes.
 * Lives in memory across client-side navigations.
 */
export const apiClient: Readable<APIClient | null> = derived(
    [apiEndpoint, apiKey],
    ([$endpoint, $apiKey]) => {
        if (!browser || !$endpoint) {
            return null;
        }

        try {
            const url = new URL($endpoint);
            const normalized = url.toString().replace(/\/$/, '');
            return new APIClient(normalized, $apiKey ?? '', fetch);
        } catch {
            return null;
        }
    }
);

/**
 * Whether an API endpoint is configured
 */
export const hasEndpoint: Readable<boolean> = derived(apiEndpoint, ($endpoint) =>
    Boolean($endpoint?.trim())
);

/**
 * Whether an API key is configured
 */
export const hasApiKey: Readable<boolean> = derived(apiKey, ($key) => Boolean($key?.trim()));

/**
 * Whether both endpoint and API key are configured
 */
export const isFullyConfigured: Readable<boolean> = derived(
    [hasEndpoint, hasApiKey],
    ([$hasEndpoint, $hasApiKey]) => $hasEndpoint && $hasApiKey
);
