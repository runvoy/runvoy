import { writable } from 'svelte/store';

// Create stores that sync with localStorage
function createLocalStorageStore(key, initialValue) {
    // Try to load from localStorage
    const stored = typeof window !== 'undefined' ? localStorage.getItem(key) : null;
    const initial = stored ? stored : initialValue;

    const store = writable(initial);

    // Subscribe to changes and update localStorage
    if (typeof window !== 'undefined') {
        store.subscribe(value => {
            if (value === null || value === undefined) {
                localStorage.removeItem(key);
            } else {
                localStorage.setItem(key, value);
            }
        });
    }

    return store;
}

export const apiEndpoint = createLocalStorageStore('runvoy_endpoint', null);
export const apiKey = createLocalStorageStore('runvoy_api_key', null);
