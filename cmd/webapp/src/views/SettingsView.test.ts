/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import SettingsView from './SettingsView.svelte';
import type APIClient from '../lib/api';
import type { HealthResponse } from '../types/api';

describe('SettingsView', () => {
    let mockApiClient: Partial<APIClient>;

    beforeEach(() => {
        mockApiClient = {
            getHealth: vi.fn()
        };
    });

    afterEach(() => {
        vi.clearAllMocks();
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
