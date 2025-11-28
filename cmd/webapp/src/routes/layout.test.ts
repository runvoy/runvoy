/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import type { Redirect } from '@sveltejs/kit';

import Layout from './+layout.svelte';
import { load } from './+layout';
import { apiEndpoint, apiKey, setApiEndpoint, setApiKey } from '../stores/config';

vi.mock('$app/environment', () => ({
    version: 'test-version',
    browser: true
}));

describe('layout load', () => {
    beforeEach(() => {
        localStorage.clear();
        setApiEndpoint(null);
        setApiKey(null);
    });

    it('redirects to settings when no endpoint is configured', () => {
        try {
            load({
                url: new URL('http://localhost:5173/logs'),
                data: { initialConfig: { endpoint: null, apiKey: null } }
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
                url: new URL('http://localhost:5173/logs'),
                data: { initialConfig: { endpoint: null, apiKey: null } }
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
            url: new URL('http://localhost:5173/'),
            data: { initialConfig: { endpoint: null, apiKey: null } }
        } as any) as { hasEndpoint: boolean; hasApiKey: boolean };

        expect(result.hasEndpoint).toBe(true);
        expect(result.hasApiKey).toBe(true);
    });

    it('reads from localStorage', () => {
        localStorage.setItem('runvoy_endpoint', 'https://api.local.test');
        localStorage.setItem('runvoy_api_key', 'local-key');

        const result = load({
            url: new URL('http://localhost:5173/'),
            data: { initialConfig: { endpoint: null, apiKey: null } }
        } as any) as { hasEndpoint: boolean; hasApiKey: boolean };

        expect(result.hasEndpoint).toBe(true);
        expect(result.hasApiKey).toBe(true);
    });

    it('handles missing server data in a SPA/static environment', () => {
        localStorage.setItem('runvoy_endpoint', 'https://api.local.test');
        localStorage.setItem('runvoy_api_key', 'local-key');

        const result = load({
            url: new URL('http://localhost:5173/'),
            data: undefined
        } as any) as { hasEndpoint: boolean; hasApiKey: boolean };

        expect(result.hasEndpoint).toBe(true);
        expect(result.hasApiKey).toBe(true);
    });

    it('honors initial config returned from the server', () => {
        const result = load({
            url: new URL('http://localhost:5173/'),
            data: { initialConfig: { endpoint: 'https://api.server.test', apiKey: 'server-key' } }
        } as any) as { hasEndpoint: boolean; hasApiKey: boolean };

        expect(result.hasEndpoint).toBe(true);
        expect(result.hasApiKey).toBe(true);
    });
});

describe('navigation state', () => {
    beforeEach(() => {
        setApiEndpoint(null);
        setApiKey(null);
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
        setApiEndpoint('https://api.example.test');
        setApiKey('abc123');

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
