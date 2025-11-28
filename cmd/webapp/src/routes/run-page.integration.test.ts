/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';

import { load } from './+page';
import RunPage from './+page.svelte';

vi.mock('$app/environment', () => ({
    browser: true,
    version: 'test-version'
}));

vi.mock('$app/navigation', () => ({
    goto: vi.fn()
}));

vi.mock('../lib/executionState', () => ({
    switchExecution: vi.fn()
}));

describe('run page load integration', () => {
    beforeEach(() => {
        localStorage.clear();
        document.cookie = '';
    });

    it('constructs an API client through load and passes it to the view', async () => {
        const mockFetch = vi.fn().mockResolvedValue(
            new Response(
                JSON.stringify({ execution_id: 'exec-123', websocket_url: 'wss://example.test' }),
                { status: 200 }
            )
        );

        const data = await load({
            fetch: mockFetch as any,
            parent: async () => ({
                endpoint: 'https://api.integration.test',
                apiKey: 'abc123',
                hasEndpoint: true,
                hasApiKey: true
            })
        } as any);

        render(RunPage as any, { props: { data } });

        const commandInput = screen.getByLabelText('Command to execute');
        await fireEvent.input(commandInput, { target: { value: 'echo from test' } });

        const submitButton = screen.getByRole('button', { name: /Run command/i });
        await fireEvent.click(submitButton);

        await waitFor(() => {
            expect(mockFetch).toHaveBeenCalled();
        });

        expect(mockFetch.mock.calls[0][0]).toContain('https://api.integration.test');
    });

    it('throws when configuration is missing during load', async () => {
        const loadPromise = load({
            fetch: vi.fn() as any,
            parent: async () => ({
                endpoint: null,
                apiKey: null,
                hasEndpoint: false,
                hasApiKey: false
            })
        } as any);

        await expect(loadPromise).rejects.toThrow(/API configuration is incomplete/);
    });
});
