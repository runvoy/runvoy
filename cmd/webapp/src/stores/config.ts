/**
 * Configuration store with localStorage persistence
 */
import { writable, type Writable } from 'svelte/store';

const COOKIE_MAX_AGE = 60 * 60 * 24 * 30; // 30 days

function getCookieValue(key: string): string | null {
    if (typeof document === 'undefined') {
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
    if (typeof document === 'undefined') {
        return;
    }

    if (!value) {
        document.cookie = `${key}=; Max-Age=0; Path=/; SameSite=Lax`;
        return;
    }

    document.cookie = `${key}=${encodeURIComponent(value)}; Max-Age=${COOKIE_MAX_AGE}; Path=/; SameSite=Lax`;
}

/**
 * Create a store that syncs with localStorage
 */
function createLocalStorageStore<T>(key: string, initialValue: T): Writable<T> {
    // Try to load from localStorage
    const stored = typeof window !== 'undefined' ? localStorage.getItem(key) : null;
    const cookieValue = getCookieValue(key);
    const initial: T = (stored ?? cookieValue ?? initialValue) as T;

    const store = writable<T>(initial);

    // Subscribe to changes and update localStorage
    if (typeof window !== 'undefined') {
        store.subscribe((value: T) => {
            if (value === null || value === undefined) {
                localStorage.removeItem(key);
                persistCookie(key, null);
            } else {
                const stringValue = String(value);
                localStorage.setItem(key, stringValue);
                persistCookie(key, stringValue);
            }
        });
    }

    return store;
}

export const apiEndpoint = createLocalStorageStore<string | null>('runvoy_endpoint', null);
export const apiKey = createLocalStorageStore<string | null>('runvoy_api_key', null);
