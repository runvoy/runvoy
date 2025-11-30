import { browser } from '$app/environment';
import { derived, writable, type Readable, type Writable } from 'svelte/store';

/**
 * Configuration store with localStorage persistence
 */

type PersistedStore<T> = {
    readable: Readable<T>;
    set: Writable<T>['set'];
    update: Writable<T>['update'];
    hydrate: () => T;
};

function parsePersistedValue<T>(value: string | null): T | null {
    if (!value) {
        return null;
    }

    try {
        return JSON.parse(value) as T;
    } catch {
        return value as unknown as T;
    }
}

function serializeJSON(value: unknown): string | null {
    if (value === null || value === undefined) {
        return null;
    }

    try {
        return JSON.stringify(value);
    } catch {
        return null;
    }
}

function createPersistentStore<T>(key: string, defaultValue: T): PersistedStore<T> {
    const writableStore = writable<T>(defaultValue);
    const readable = derived(writableStore, (value) => value);

    let persistedSubscription: (() => void) | null = null;
    let hydrated = false;
    let currentValue = defaultValue;

    writableStore.subscribe((value) => {
        currentValue = value;
    });

    const hydrate = (): T => {
        const stored = browser ? parsePersistedValue<T>(localStorage.getItem(key)) : null;
        const nextValue = (stored ?? currentValue ?? defaultValue) as T;

        if (!hydrated || browser) {
            writableStore.set(nextValue);
        }

        if (browser && !persistedSubscription) {
            persistedSubscription = writableStore.subscribe((value: T) => {
                const serialized = serializeJSON(value);

                if (serialized === null) {
                    localStorage.removeItem(key);
                } else {
                    localStorage.setItem(key, serialized);
                }
            });
        }

        hydrated = true;
        return nextValue;
    };

    const set: Writable<T>['set'] = (value) => {
        hydrate();
        writableStore.set(value);
    };

    const update: Writable<T>['update'] = (updater) => {
        hydrate();
        writableStore.update(updater);
    };

    return {
        readable,
        set,
        update,
        hydrate
    };
}

const apiEndpointStore = createPersistentStore<string | null>('runvoy_endpoint', null);
const apiKeyStore = createPersistentStore<string | null>('runvoy_api_key', null);

export const apiEndpoint = apiEndpointStore.readable;
export const apiKey = apiKeyStore.readable;

export const setApiEndpoint = apiEndpointStore.set;
export const updateApiEndpoint = apiEndpointStore.update;
export const setApiKey = apiKeyStore.set;
export const updateApiKey = apiKeyStore.update;

export function hydrateConfigStores() {
    const endpoint = apiEndpointStore.hydrate();
    const apiKey = apiKeyStore.hydrate();

    return { endpoint, apiKey };
}
