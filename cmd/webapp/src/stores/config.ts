import { browser } from '$app/environment';
import { derived, writable, type Readable, type Writable } from 'svelte/store';

/**
 * Configuration store with localStorage persistence
 */

const COOKIE_MAX_AGE = 60 * 60 * 24 * 30; // 30 days

type PersistedStore<T> = {
    readable: Readable<T>;
    set: Writable<T>['set'];
    update: Writable<T>['update'];
    hydrate: (initialValue?: T) => T;
};

type ConfigSnapshot = {
    endpoint?: string | null;
    apiKey?: string | null;
};

function getCookieValue(key: string): string | null {
    if (!browser) {
        return null;
    }

    const cookies = document.cookie.split(';').map((cookie) => cookie.trim());

    for (const cookie of cookies) {
        if (cookie.startsWith(`${key}=`)) {
            return decodeURIComponent(cookie.slice(key.length + 1));
        }
    }

    return null;
}

function persistCookie(key: string, value: string | null): void {
    if (!browser) {
        return;
    }

    if (!value) {
        document.cookie = `${key}=; Max-Age=0; Path=/; SameSite=Lax`;
        return;
    }

    document.cookie = `${key}=${encodeURIComponent(value)}; Max-Age=${COOKIE_MAX_AGE}; Path=/; SameSite=Lax`;
}

function parseJSON<T>(value: string | null): T | null {
    if (!value) {
        return null;
    }

    try {
        return JSON.parse(value) as T;
    } catch (error) {
        console.warn('Failed to parse stored value', error);
        return value as unknown as T;
    }
}

function serializeJSON(value: unknown): string | null {
    if (value === null || value === undefined) {
        return null;
    }

    try {
        return JSON.stringify(value);
    } catch (error) {
        console.warn('Failed to serialize value', error);
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

    const hydrate = (initialValue?: T): T => {
        const stored = browser ? parseJSON<T>(localStorage.getItem(key)) : null;
        const cookieValue = browser ? parseJSON<T>(getCookieValue(key)) : null;
        const nextValue = browser
            ? ((stored ?? cookieValue ?? initialValue ?? defaultValue) as T)
            : ((initialValue ?? currentValue ?? defaultValue) as T);

        if (!hydrated || browser) {
            writableStore.set(nextValue);
        }

        if (browser && !persistedSubscription) {
            persistedSubscription = writableStore.subscribe((value: T) => {
                const serialized = serializeJSON(value);

                if (serialized === null) {
                    localStorage.removeItem(key);
                    persistCookie(key, null);
                } else {
                    localStorage.setItem(key, serialized);
                    persistCookie(key, serialized);
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

export function hydrateConfigStores(initial?: ConfigSnapshot) {
    const endpoint = apiEndpointStore.hydrate(initial?.endpoint);
    const apiKey = apiKeyStore.hydrate(initial?.apiKey);

    return { endpoint, apiKey };
}
