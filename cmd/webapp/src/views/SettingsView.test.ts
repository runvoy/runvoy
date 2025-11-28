/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/svelte';
import SettingsView from './SettingsView.svelte';
import type APIClient from '../lib/api';
import type { HealthResponse } from '../types/api';
import { apiEndpoint, apiKey } from '../stores/config';
import { get } from 'svelte/store';

describe('SettingsView', () => {
    let mockApiClient: Partial<APIClient>;

    beforeEach(() => {
        mockApiClient = {
            getHealth: vi.fn()
        };
        apiEndpoint.set(null);
        apiKey.set(null);
    });

    afterEach(() => {
        vi.clearAllMocks();
        apiEndpoint.set(null);
        apiKey.set(null);
    });

    it('displays backend provider and region when available', async () => {
        const mockHealth: HealthResponse = {
            status: 'OK',
            version: '1.2.3',
            provider: 'aws',
            region: 'us-west-2'
        };

        vi.mocked(mockApiClient.getHealth as any).mockResolvedValue(mockHealth);

        render(SettingsView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        await waitFor(() => {
            expect(screen.getByText(mockHealth.provider)).toBeInTheDocument();
            expect(screen.getByText(mockHealth.region!)).toBeInTheDocument();
        });
    });

    it('saves configuration from the settings form', async () => {
        render(SettingsView, {
            props: {
                apiClient: null,
                isConfigured: false
            }
        });

        const endpointField = screen.getByPlaceholderText('https://api.runvoy.example.com');
        const apiKeyField = screen.getByPlaceholderText('Enter API key (or claim one later)');

        await fireEvent.input(endpointField, {
            target: { value: 'https://api.example.com' }
        });

        await fireEvent.input(apiKeyField, {
            target: { value: 'super-secret' }
        });

        await fireEvent.click(screen.getByText('Save configuration'));

        await waitFor(() => {
            expect(get(apiEndpoint)).toBe('https://api.example.com');
            expect(get(apiKey)).toBe('super-secret');
            expect(screen.getByText('Configuration saved')).toBeInTheDocument();
        });
    });

    it('fetches backend health after saving configuration', async () => {
        const mockHealth: HealthResponse = {
            status: 'OK',
            version: '1.0.0',
            provider: 'mock'
        };

        vi.mocked(mockApiClient.getHealth as any).mockResolvedValue(mockHealth);

        render(SettingsView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        await waitFor(() => {
            expect(mockApiClient.getHealth).toHaveBeenCalled();
        });

        const initialCallCount = vi.mocked(mockApiClient.getHealth as any).mock.calls.length;

        const endpointField = screen.getByPlaceholderText('https://api.runvoy.example.com');
        const apiKeyField = screen.getByPlaceholderText('Enter API key (or claim one later)');

        await fireEvent.input(endpointField, {
            target: { value: 'https://api.example.com' }
        });

        await fireEvent.input(apiKeyField, {
            target: { value: 'updated-key' }
        });

        await fireEvent.click(screen.getByText('Save configuration'));

        await waitFor(() => {
            expect(mockApiClient.getHealth).toHaveBeenCalledTimes(initialCallCount + 1);
        });
    });

    it('validates the endpoint before saving', async () => {
        render(SettingsView, {
            props: {
                apiClient: null,
                isConfigured: false
            }
        });

        await fireEvent.click(screen.getByText('Save configuration'));

        expect(await screen.findByText('Please enter an endpoint URL')).toBeInTheDocument();

        const endpointField = screen.getByPlaceholderText('https://api.runvoy.example.com');

        await fireEvent.input(endpointField, {
            target: { value: 'not-a-url' }
        });

        await fireEvent.click(screen.getByText('Save configuration'));

        expect(await screen.findByText('Invalid URL format')).toBeInTheDocument();
    });

    it('shows fallback when region is missing', async () => {
        const mockHealth: HealthResponse = {
            status: 'OK',
            version: '1.2.3',
            provider: 'local'
        };

        vi.mocked(mockApiClient.getHealth as any).mockResolvedValue(mockHealth);

        render(SettingsView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        await waitFor(() => {
            expect(screen.getByText(mockHealth.provider)).toBeInTheDocument();
            expect(screen.getByText('Unknown')).toBeInTheDocument();
        });
    });
});
