/**
 * Configuration store with localStorage persistence
 */
import { writable, type Writable } from 'svelte/store';

/**
 * Create a store that syncs with localStorage
 */
function createLocalStorageStore<T>(key: string, initialValue: T): Writable<T> {
    // Try to load from localStorage
    const stored = typeof window !== 'undefined' ? localStorage.getItem(key) : null;
    const initial: T = stored ? (stored as T) : initialValue;

    const store = writable<T>(initial);

    // Subscribe to changes and update localStorage
    if (typeof window !== 'undefined') {
        store.subscribe((value: T) => {
            if (value === null || value === undefined) {
                localStorage.removeItem(key);
            } else {
                localStorage.setItem(key, String(value));
            }
        });
    }

    return store;
}

export const apiEndpoint = createLocalStorageStore<string | null>('runvoy_endpoint', null);
export const apiKey = createLocalStorageStore<string | null>('runvoy_api_key', null);
