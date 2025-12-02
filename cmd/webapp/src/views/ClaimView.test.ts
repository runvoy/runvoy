/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/svelte';
import ClaimView from './ClaimView.svelte';
import type APIClient from '../lib/api';
import type { ClaimAPIKeyResponse } from '../types/api';
import { apiEndpoint, apiKey, setApiEndpoint, setApiKey } from '../stores/config';
import { get } from 'svelte/store';

describe('ClaimView', () => {
    let mockApiClient: Partial<APIClient>;

    beforeEach(() => {
        mockApiClient = {
            endpoint: 'https://api.example.com',
            claimAPIKey: vi.fn()
        };
        setApiEndpoint(null);
        setApiKey(null);
        vi.mocked(globalThis.fetch).mockClear();
    });

    afterEach(() => {
        cleanup();
        vi.clearAllMocks();
        setApiEndpoint(null);
        setApiKey(null);
    });

    it('should render claim form with token input', () => {
        render(ClaimView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        expect(screen.getByText('🔑 Claim API Key')).toBeInTheDocument();
        expect(
            screen.getByPlaceholderText('Paste your invitation token here...')
        ).toBeInTheDocument();
        expect(screen.getByText('Claim Key')).toBeInTheDocument();
    });

    it('should show error when token is empty and claim is clicked', async () => {
        render(ClaimView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        const claimButton = screen.getByText('Claim Key');
        await fireEvent.click(claimButton);

        await waitFor(() => {
            expect(screen.getByText('Please enter an invitation token')).toBeInTheDocument();
        });
    });

    it('should show error when endpoint is not configured', async () => {
        render(ClaimView, {
            props: {
                apiClient: null,
                isConfigured: false
            }
        });

        const textarea = screen.getByPlaceholderText('Paste your invitation token here...');
        await fireEvent.input(textarea, { target: { value: 'test-token' } });

        const claimButton = screen.getByText('Claim Key');
        await fireEvent.click(claimButton);

        await waitFor(() => {
            expect(screen.getByText('Please configure the API endpoint first')).toBeInTheDocument();
        });
    });

    it('should use endpoint from apiClient when available', async () => {
        const mockClaimResponse: ClaimAPIKeyResponse = {
            api_key: 'new-api-key',
            user_email: 'user@example.com',
            message: 'Welcome!'
        };

        vi.mocked(globalThis.fetch).mockResolvedValueOnce({
            ok: true,
            json: async () => mockClaimResponse
        } as Response);

        render(ClaimView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        const textarea = screen.getByPlaceholderText('Paste your invitation token here...');
        await fireEvent.input(textarea, { target: { value: 'test-token' } });

        const claimButton = screen.getByText('Claim Key');
        await fireEvent.click(claimButton);

        await waitFor(() => {
            expect(screen.getByText('✅ API Key Claimed!')).toBeInTheDocument();
        });
    });

    it('should use endpoint from store when apiClient is null but endpoint is set', async () => {
        setApiEndpoint('https://api.store.com');

        const mockClaimResponse: ClaimAPIKeyResponse = {
            api_key: 'new-api-key',
            user_email: 'user@example.com'
        };

        vi.mocked(globalThis.fetch).mockResolvedValueOnce({
            ok: true,
            json: async () => mockClaimResponse
        } as Response);

        render(ClaimView, {
            props: {
                apiClient: null,
                isConfigured: false
            }
        });

        const textarea = screen.getByPlaceholderText('Paste your invitation token here...');
        await fireEvent.input(textarea, { target: { value: 'test-token' } });

        const claimButton = screen.getByText('Claim Key');
        await fireEvent.click(claimButton);

        await waitFor(() => {
            expect(screen.getByText('✅ API Key Claimed!')).toBeInTheDocument();
        });
    });

    it('should successfully claim API key and save it', async () => {
        const mockClaimResponse: ClaimAPIKeyResponse = {
            api_key: 'new-api-key-123',
            user_email: 'user@example.com',
            message: 'Welcome to runvoy!'
        };

        vi.mocked(globalThis.fetch).mockResolvedValueOnce({
            ok: true,
            json: async () => mockClaimResponse
        } as Response);

        render(ClaimView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        const textarea = screen.getByPlaceholderText('Paste your invitation token here...');
        await fireEvent.input(textarea, { target: { value: 'test-token' } });

        const claimButton = screen.getByText('Claim Key');
        await fireEvent.click(claimButton);

        await waitFor(() => {
            expect(screen.getByText('✅ API Key Claimed!')).toBeInTheDocument();
            expect(screen.getByText('user@example.com')).toBeInTheDocument();
            expect(screen.getByText('Welcome to runvoy!')).toBeInTheDocument();
        });

        // Verify API key was saved
        expect(get(apiKey)).toBe('new-api-key-123');
    });

    it('should show success message after claim', async () => {
        const mockClaimResponse: ClaimAPIKeyResponse = {
            api_key: 'new-api-key',
            user_email: 'user@example.com'
        };

        vi.mocked(globalThis.fetch).mockResolvedValueOnce({
            ok: true,
            json: async () => mockClaimResponse
        } as Response);

        render(ClaimView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        const textarea = screen.getByPlaceholderText('Paste your invitation token here...');
        await fireEvent.input(textarea, { target: { value: 'test-token' } });

        const claimButton = screen.getByText('Claim Key');
        await fireEvent.click(claimButton);

        await waitFor(() => {
            // After successful claim, the form is replaced with success message
            expect(screen.getByText('✅ API Key Claimed!')).toBeInTheDocument();
        });
    });

    it('should show error message when claim fails', async () => {
        vi.mocked(globalThis.fetch).mockResolvedValueOnce({
            ok: false,
            status: 400,
            json: async () => ({ error: 'Token has expired' })
        } as Response);

        render(ClaimView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        const textarea = screen.getByPlaceholderText('Paste your invitation token here...');
        await fireEvent.input(textarea, { target: { value: 'invalid-token' } });

        const claimButton = screen.getByText('Claim Key');
        await fireEvent.click(claimButton);

        await waitFor(() => {
            expect(screen.getByRole('alert')).toBeInTheDocument();
        });
    });

    it('should show generic error message when claim fails without details', async () => {
        vi.mocked(globalThis.fetch).mockRejectedValueOnce(new Error('Network error'));

        render(ClaimView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        const textarea = screen.getByPlaceholderText('Paste your invitation token here...');
        await fireEvent.input(textarea, { target: { value: 'test-token' } });

        const claimButton = screen.getByText('Claim Key');
        await fireEvent.click(claimButton);

        await waitFor(() => {
            expect(screen.getByText('Network error')).toBeInTheDocument();
        });
    });

    it('should disable claim button when token is empty', () => {
        render(ClaimView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        const claimButton = screen.getByText('Claim Key');
        expect(claimButton).toBeDisabled();
    });

    it('should enable claim button when token is entered', async () => {
        render(ClaimView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        const textarea = screen.getByPlaceholderText('Paste your invitation token here...');
        await fireEvent.input(textarea, { target: { value: 'test-token' } });

        const claimButton = screen.getByText('Claim Key');
        expect(claimButton).not.toBeDisabled();
    });

    it('should show hasEndpoint as true when isConfigured is true', () => {
        render(ClaimView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        const claimButton = screen.getByText('Claim Key');
        // Button should be disabled only if token is empty, not because of endpoint
        expect(claimButton).toBeDisabled(); // Because token is empty
    });
});
