/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';

import Layout from './+layout.svelte';

const mockGoto = vi.hoisted(() => vi.fn());

const createMockStore = vi.hoisted(() => {
    return <T>(initial: T) => {
        let value = initial;
        const subscribers = new Set<(current: T) => void>();

        return {
            set: (newValue: T) => {
                value = newValue;
                subscribers.forEach((fn) => fn(value));
            },
            subscribe: (fn: (current: T) => void) => {
                subscribers.add(fn);
                fn(value);
                return () => subscribers.delete(fn);
            }
        };
    };
});

const page = vi.hoisted(() =>
    createMockStore({
        url: new URL('http://localhost:5173/')
    })
);

const apiEndpoint = vi.hoisted(() => createMockStore(''));
const apiKey = vi.hoisted(() => createMockStore(''));

vi.mock('$app/navigation', () => ({
    goto: mockGoto
}));

vi.mock('$app/environment', () => ({
    version: 'test-version'
}));

vi.mock('$app/stores', () => ({
    page
}));

vi.mock('../stores/config', () => ({
    apiEndpoint,
    apiKey
}));

describe('App layout guardrails', () => {
    beforeEach(() => {
        mockGoto.mockReset();
        apiEndpoint.set('');
        apiKey.set('');
        page.set({ url: new URL('http://localhost:5173/') });
    });

    it('redirects to settings when no endpoint is configured', async () => {
        page.set({ url: new URL('http://localhost:5173/claim') });

        render(Layout as any, { slots: { default: '<p>content</p>' } });

        await waitFor(() => expect(mockGoto).toHaveBeenCalledWith('/settings'));
    });

    it('allows claim view when endpoint exists but API key is missing', async () => {
        page.set({ url: new URL('http://localhost:5173/claim') });
        apiEndpoint.set('https://api.example.test');

        render(Layout as any, { slots: { default: '<p>content</p>' } });

        await waitFor(() => expect(mockGoto).not.toHaveBeenCalled());
    });
});
