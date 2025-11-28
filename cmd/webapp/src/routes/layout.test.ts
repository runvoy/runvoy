/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import type { Redirect } from '@sveltejs/kit';

import Layout from './+layout.svelte';
import { load } from './+layout';

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

const apiEndpoint = vi.hoisted(() => createMockStore<string | null>(null));
const apiKey = vi.hoisted(() => createMockStore<string | null>(null));

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

describe('layout load', () => {
    beforeEach(() => {
        localStorage.clear();
    });

    it('redirects to settings when no endpoint is configured', () => {
        try {
            load({
                url: new URL('http://localhost:5173/logs')
            } as any);
        } catch (error) {
            const redirectError = error as Redirect;
            expect(redirectError.location).toBe('/settings');
            expect(redirectError.status).toBe(307);
            return;
        }

        throw new Error('Expected redirect to settings');
    });

    it('redirects to claim when API key is missing', () => {
        localStorage.setItem('runvoy_endpoint', 'https://api.example.test');

        try {
            load({
                url: new URL('http://localhost:5173/logs')
            } as any);
        } catch (error) {
            const redirectError = error as Redirect;
            expect(redirectError.location).toBe('/claim');
            expect(redirectError.status).toBe(307);
            return;
        }

        throw new Error('Expected redirect to claim');
    });

    it('returns persisted flags when configuration exists', () => {
        localStorage.setItem('runvoy_endpoint', 'https://api.example.test');
        localStorage.setItem('runvoy_api_key', 'secret-key');

        const result = load({
            url: new URL('http://localhost:5173/')
        } as any) as { hasEndpoint: boolean; hasApiKey: boolean };

        expect(result.hasEndpoint).toBe(true);
        expect(result.hasApiKey).toBe(true);
    });

    it('reads from localStorage', () => {
        localStorage.setItem('runvoy_endpoint', 'https://api.local.test');
        localStorage.setItem('runvoy_api_key', 'local-key');

        const result = load({
            url: new URL('http://localhost:5173/')
        } as any) as { hasEndpoint: boolean; hasApiKey: boolean };

        expect(result.hasEndpoint).toBe(true);
        expect(result.hasApiKey).toBe(true);
    });
});

describe('navigation state', () => {
    beforeEach(() => {
        page.set({ url: new URL('http://localhost:5173/') });
        apiEndpoint.set(null);
        apiKey.set(null);
    });

    it('disables non-settings views when endpoint is missing', () => {
        render(Layout as any, {
            props: { data: { hasEndpoint: false, hasApiKey: false } }
        });

        expect(screen.getByText('Run Command')).toHaveClass('disabled');
        expect(screen.getByText('Executions')).toHaveClass('disabled');
        expect(screen.getByText('Claim Key')).toHaveClass('disabled');
        expect(screen.getByText('Logs')).toHaveClass('disabled');
        expect(screen.getByText('Settings')).not.toHaveClass('disabled');
    });

    it('enables claim but disables logs/list when API key is missing', () => {
        render(Layout as any, {
            props: { data: { hasEndpoint: true, hasApiKey: false } }
        });

        expect(screen.getByText('Claim Key')).not.toHaveClass('disabled');
        expect(screen.getByText('Run Command')).not.toHaveClass('disabled');
        expect(screen.getByText('Executions')).toHaveClass('disabled');
        expect(screen.getByText('Logs')).toHaveClass('disabled');
    });

    it('enables all views when fully configured', () => {
        apiEndpoint.set('https://api.example.test');
        apiKey.set('abc123');

        render(Layout as any, {
            props: { data: { hasEndpoint: true, hasApiKey: true } }
        });

        expect(screen.getByText('Run Command')).not.toHaveClass('disabled');
        expect(screen.getByText('Executions')).not.toHaveClass('disabled');
        expect(screen.getByText('Claim Key')).not.toHaveClass('disabled');
        expect(screen.getByText('Logs')).not.toHaveClass('disabled');
        expect(screen.getByText('Settings')).not.toHaveClass('disabled');
    });
});
