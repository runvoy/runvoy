/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import type { Redirect } from '@sveltejs/kit';

import Layout from './+layout.svelte';
import { load } from './+layout';
import { setApiEndpoint, setApiKey } from '../stores/config';
import APIClient from '../lib/api';

vi.mock('$app/environment', () => ({
    version: 'test-version',
    browser: true
}));

// Create a simple mock store inside hoisted to avoid initialization order issues
const mockPageStore = vi.hoisted(() => {
    // Create a simple mock store that implements the writable interface
    const createMockStore = <T>(initial: T) => {
        let value = initial;
        const subscribers = new Set<(value: T) => void>();

        return {
            set: (newValue: T) => {
                value = newValue;
                subscribers.forEach((fn) => fn(value));
            },
            update: (fn: (value: T) => T) => {
                value = fn(value);
                subscribers.forEach((subscriber) => subscriber(value));
            },
            subscribe: (fn: (value: T) => void) => {
                subscribers.add(fn);
                fn(value); // Call immediately with current value
                return () => subscribers.delete(fn);
            }
        };
    };

    return createMockStore({
        url: new URL('http://localhost:5173/')
    });
});

// Mock the $app/stores module
vi.mock('$app/stores', () => {
    return {
        page: mockPageStore
    };
});

describe('layout load', () => {
    const mockFetch = vi.fn();

    beforeEach(() => {
        localStorage.clear();
        setApiEndpoint(null);
        setApiKey(null);
        mockFetch.mockClear();
    });

    it('redirects to settings when no endpoint is configured', async () => {
        try {
            await load({
                url: new URL('http://localhost:5173/logs'),
                fetch: mockFetch
            } as any);
        } catch (error) {
            const redirectError = error as Redirect;
            expect(redirectError.location).toBe('/settings');
            expect(redirectError.status).toBe(307);
            return;
        }

        throw new Error('Expected redirect to settings');
    });

    it('redirects to claim when API key is missing', async () => {
        localStorage.setItem('runvoy_endpoint', 'https://api.example.test');

        try {
            await load({
                url: new URL('http://localhost:5173/logs'),
                fetch: mockFetch
            } as any);
        } catch (error) {
            const redirectError = error as Redirect;
            expect(redirectError.location).toBe('/claim');
            expect(redirectError.status).toBe(307);
            return;
        }

        throw new Error('Expected redirect to claim');
    });

    it('returns persisted flags when configuration exists', async () => {
        localStorage.setItem('runvoy_endpoint', 'https://api.example.test');
        localStorage.setItem('runvoy_api_key', 'secret-key');

        const result = (await load({
            url: new URL('http://localhost:5173/'),
            fetch: mockFetch
        } as any)) as {
            hasEndpoint: boolean;
            hasApiKey: boolean;
            endpoint: string | null;
            apiKey: string | null;
            apiClient: APIClient | null;
        };

        expect(result.hasEndpoint).toBe(true);
        expect(result.hasApiKey).toBe(true);
        expect(result.endpoint).toBe('https://api.example.test');
        expect(result.apiKey).toBe('secret-key');
        expect(result.apiClient).toBeInstanceOf(APIClient);
    });

    it('reads from localStorage', async () => {
        localStorage.setItem('runvoy_endpoint', 'https://api.local.test');
        localStorage.setItem('runvoy_api_key', 'local-key');

        const result = (await load({
            url: new URL('http://localhost:5173/'),
            fetch: mockFetch
        } as any)) as {
            hasEndpoint: boolean;
            hasApiKey: boolean;
            endpoint: string | null;
            apiKey: string | null;
            apiClient: APIClient | null;
        };

        expect(result.hasEndpoint).toBe(true);
        expect(result.hasApiKey).toBe(true);
        expect(result.endpoint).toBe('https://api.local.test');
        expect(result.apiKey).toBe('local-key');
        expect(result.apiClient).toBeInstanceOf(APIClient);
    });

    it('loads without any server data in a SPA/static environment', async () => {
        localStorage.setItem('runvoy_endpoint', 'https://api.local.test');
        localStorage.setItem('runvoy_api_key', 'local-key');

        const result = (await load({
            url: new URL('http://localhost:5173/'),
            fetch: mockFetch
        } as any)) as {
            hasEndpoint: boolean;
            hasApiKey: boolean;
            endpoint: string | null;
            apiKey: string | null;
            apiClient: APIClient | null;
        };

        expect(result.hasEndpoint).toBe(true);
        expect(result.hasApiKey).toBe(true);
        expect(result.endpoint).toBe('https://api.local.test');
        expect(result.apiKey).toBe('local-key');
        expect(result.apiClient).toBeInstanceOf(APIClient);
    });

    it('hydrates configuration from cookies when available', async () => {
        const cookies = new Map([
            ['runvoy_endpoint', encodeURIComponent(JSON.stringify('https://cookie.example'))],
            ['runvoy_api_key', encodeURIComponent(JSON.stringify('cookie-key'))]
        ]);

        const result = (await load({
            url: new URL('http://localhost:5173/'),
            fetch: mockFetch,
            cookies: {
                get: (key: string) => cookies.get(key) || '',
                set: vi.fn(),
                delete: vi.fn(),
                serialize: vi.fn()
            }
        } as any)) as {
            hasEndpoint: boolean;
            hasApiKey: boolean;
            endpoint: string | null;
            apiKey: string | null;
            apiClient: APIClient | null;
        };

        expect(result.hasEndpoint).toBe(true);
        expect(result.hasApiKey).toBe(true);
        expect(result.endpoint).toBe('https://cookie.example');
        expect(result.apiKey).toBe('cookie-key');
        expect(result.apiClient).toBeInstanceOf(APIClient);
    });
});

describe('navigation state', () => {
    beforeEach(() => {
        setApiEndpoint(null);
        setApiKey(null);
        // Reset the page store to root path before each test
        mockPageStore.set({
            url: new URL('http://localhost:5173/')
        });
    });

    it('disables non-settings views when endpoint is missing', () => {
        render(Layout as any, {
            props: {
                data: {
                    hasEndpoint: false,
                    hasApiKey: false,
                    endpoint: null,
                    apiKey: null,
                    apiClient: null
                }
            }
        });

        expect(screen.getByText('Run Command')).toHaveClass('disabled');
        expect(screen.getByText('Executions')).toHaveClass('disabled');
        expect(screen.getByText('Claim Key')).toHaveClass('disabled');
        expect(screen.getByText('Logs')).toHaveClass('disabled');
        expect(screen.getByText('Settings')).not.toHaveClass('disabled');
    });

    it('enables claim but disables logs/list when API key is missing', () => {
        const mockFetch = vi.fn();
        const apiClient = new APIClient('https://api.example.test', '', mockFetch);

        render(Layout as any, {
            props: {
                data: {
                    hasEndpoint: true,
                    hasApiKey: false,
                    endpoint: 'https://api.example.test',
                    apiKey: null,
                    apiClient
                }
            }
        });

        expect(screen.getByText('Claim Key')).not.toHaveClass('disabled');
        expect(screen.getByText('Run Command')).not.toHaveClass('disabled');
        expect(screen.getByText('Executions')).toHaveClass('disabled');
        expect(screen.getByText('Logs')).toHaveClass('disabled');
    });

    it('enables all views when fully configured', () => {
        setApiEndpoint('https://api.example.test');
        setApiKey('abc123');
        const mockFetch = vi.fn();
        const apiClient = new APIClient('https://api.example.test', 'abc123', mockFetch);

        render(Layout as any, {
            props: {
                data: {
                    hasEndpoint: true,
                    hasApiKey: true,
                    endpoint: 'https://api.example.test',
                    apiKey: 'abc123',
                    apiClient
                }
            }
        });

        expect(screen.getByText('Run Command')).not.toHaveClass('disabled');
        expect(screen.getByText('Executions')).not.toHaveClass('disabled');
        expect(screen.getByText('Claim Key')).not.toHaveClass('disabled');
        expect(screen.getByText('Logs')).not.toHaveClass('disabled');
        expect(screen.getByText('Settings')).not.toHaveClass('disabled');
    });
});
