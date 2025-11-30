/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';

import RunPage from './+page.svelte';
import { setApiEndpoint, setApiKey } from '../stores/config';

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

// Mock the global fetch for API calls
const mockFetch = vi.fn();

describe('run page integration', () => {
    beforeEach(() => {
        localStorage.clear();
        setApiEndpoint(null);
        setApiKey(null);
        mockFetch.mockClear();
        vi.stubGlobal('fetch', mockFetch);
    });

    it('uses the API client from the store to submit commands', async () => {
        mockFetch.mockResolvedValue(
            new Response(
                JSON.stringify({
                    execution_id: 'exec-123',
                    websocket_url: 'wss://example.test'
                }),
                { status: 200 }
            )
        );

        // Set up config stores
        setApiEndpoint('https://api.integration.test');
        setApiKey('abc123');

        render(RunPage as any, { props: {} });

        const commandInput = screen.getByLabelText('Command to execute');
        await fireEvent.input(commandInput, { target: { value: 'echo from test' } });

        const submitButton = screen.getByRole('button', { name: /Run command/i });
        await fireEvent.click(submitButton);

        await waitFor(() => {
            expect(mockFetch).toHaveBeenCalled();
        });

        expect(mockFetch.mock.calls[0][0]).toContain('https://api.integration.test');
    });

    it('renders correctly when configuration is missing', async () => {
        // No config set - apiClient will be null
        render(RunPage as any, { props: {} });

        // The page should still render (though API calls will fail)
        const commandInput = screen.getByLabelText('Command to execute');
        expect(commandInput).toBeInTheDocument();
    });
});
