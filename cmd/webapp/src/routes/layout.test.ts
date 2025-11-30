/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/svelte';

import Layout from './+layout.svelte';
import { setApiEndpoint, setApiKey } from '../stores/config';

vi.mock('$app/environment', () => ({
    version: 'test-version',
    browser: true
}));

// Create a mock page state object (not a store - $app/state uses reactive objects)
const mockPage = vi.hoisted(() => ({
    url: new URL('http://localhost:5173/')
}));

// Track goto calls - must be hoisted to avoid initialization order issues
const mockGoto = vi.hoisted(() => vi.fn().mockResolvedValue(undefined));

// Mock the $app/state module
vi.mock('$app/state', () => {
    return {
        page: mockPage
    };
});

vi.mock('$app/navigation', () => ({
    goto: mockGoto
}));

// Helper to update mock page URL
function setMockPageUrl(url: string) {
    mockPage.url = new URL(url);
}

describe('navigation state', () => {
    beforeEach(() => {
        localStorage.clear();
        setApiEndpoint(null);
        setApiKey(null);
        mockGoto.mockClear();
        // Reset the page to root path before each test
        setMockPageUrl('http://localhost:5173/');
    });

    it('disables non-settings views when endpoint is missing', () => {
        render(Layout as any, {
            props: {}
        });

        expect(screen.getByText('Run Command')).toHaveClass('disabled');
        expect(screen.getByText('Executions')).toHaveClass('disabled');
        expect(screen.getByText('Claim Key')).toHaveClass('disabled');
        expect(screen.getByText('Logs')).toHaveClass('disabled');
        expect(screen.getByText('Settings')).not.toHaveClass('disabled');
    });

    it('enables claim but disables logs/list when API key is missing', () => {
        setApiEndpoint('https://api.example.test');

        render(Layout as any, {
            props: {}
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
            props: {}
        });

        expect(screen.getByText('Run Command')).not.toHaveClass('disabled');
        expect(screen.getByText('Executions')).not.toHaveClass('disabled');
        expect(screen.getByText('Claim Key')).not.toHaveClass('disabled');
        expect(screen.getByText('Logs')).not.toHaveClass('disabled');
        expect(screen.getByText('Settings')).not.toHaveClass('disabled');
    });
});

describe('redirect behavior', () => {
    beforeEach(() => {
        localStorage.clear();
        setApiEndpoint(null);
        setApiKey(null);
        mockGoto.mockClear();
    });

    it('redirects to settings when no endpoint and not on settings page', async () => {
        setMockPageUrl('http://localhost:5173/logs');

        render(Layout as any, {
            props: {}
        });

        // Wait for effect to run
        await vi.waitFor(() => {
            expect(mockGoto).toHaveBeenCalledWith('/settings', { replaceState: true });
        });
    });

    it('does not redirect when already on settings page', async () => {
        setMockPageUrl('http://localhost:5173/settings');

        render(Layout as any, {
            props: {}
        });

        // Give effect time to run
        await new Promise((resolve) => setTimeout(resolve, 50));

        expect(mockGoto).not.toHaveBeenCalled();
    });

    it('redirects to claim when endpoint exists but no API key', async () => {
        setApiEndpoint('https://api.example.test');
        setMockPageUrl('http://localhost:5173/logs');

        render(Layout as any, {
            props: {}
        });

        await vi.waitFor(() => {
            expect(mockGoto).toHaveBeenCalledWith('/claim', { replaceState: true });
        });
    });

    it('does not redirect when on claim page without API key', async () => {
        setApiEndpoint('https://api.example.test');
        setMockPageUrl('http://localhost:5173/claim');

        render(Layout as any, {
            props: {}
        });

        // Give effect time to run
        await new Promise((resolve) => setTimeout(resolve, 50));

        expect(mockGoto).not.toHaveBeenCalled();
    });

    it('does not redirect when fully configured', async () => {
        setApiEndpoint('https://api.example.test');
        setApiKey('abc123');
        setMockPageUrl('http://localhost:5173/logs');

        render(Layout as any, {
            props: {}
        });

        // Give effect time to run
        await new Promise((resolve) => setTimeout(resolve, 50));

        expect(mockGoto).not.toHaveBeenCalled();
    });
});
