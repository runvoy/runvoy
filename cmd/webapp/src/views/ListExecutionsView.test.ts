/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import ListExecutionsView from './ListExecutionsView.svelte';
import type APIClient from '../lib/api';
import type { Execution } from '../types/api';

describe('ListExecutionsView', () => {
    let mockApiClient: Partial<APIClient>;

    beforeEach(() => {
        mockApiClient = {
            listExecutions: vi.fn()
        };
    });

    afterEach(() => {
        vi.clearAllMocks();
    });

    it('should show config message when not configured', () => {
        render(ListExecutionsView, {
            props: {
                apiClient: null,
                isConfigured: false
            }
        });

        expect(screen.getByText(/Configure API access to view executions/i)).toBeInTheDocument();
        expect(screen.getByText(/⚙️ Configure API/i)).toBeInTheDocument();
    });

    it('should show empty state when no executions exist', async () => {
        vi.mocked(mockApiClient.listExecutions as any).mockResolvedValue([]);

        render(ListExecutionsView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        await waitFor(() => {
            expect(screen.getByText(/No executions found/i)).toBeInTheDocument();
        });
    });

    it('should display list of executions in table', async () => {
        const mockExecutions: Execution[] = [
            {
                execution_id: 'exec-12345678-abcd',
                status: 'SUCCEEDED',
                started_at: '2025-11-21T10:00:00Z',
                completed_at: '2025-11-21T10:01:00Z',
                exit_code: 0
            },
            {
                execution_id: 'exec-87654321-dcba',
                status: 'FAILED',
                started_at: '2025-11-21T09:00:00Z',
                completed_at: '2025-11-21T09:05:00Z',
                exit_code: 1
            }
        ];

        vi.mocked(mockApiClient.listExecutions as any).mockResolvedValue(mockExecutions);

        const { container } = render(ListExecutionsView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        await waitFor(() => {
            // IDs are truncated to first 8 chars + '...' in display
            // 'exec-12345678-abcd' becomes 'exec-123'
            expect(container.textContent).toContain('exec-123');
            expect(container.textContent).toContain('exec-876');
            expect(screen.getAllByText('SUCCEEDED')).toHaveLength(1);
            expect(screen.getAllByText('FAILED')).toHaveLength(1);
            expect(screen.getByText('0')).toBeInTheDocument();
            expect(screen.getByText('1')).toBeInTheDocument();
        });
    });

    it('should format dates correctly', async () => {
        const mockExecutions: Execution[] = [
            {
                execution_id: 'exec-test-001',
                status: 'SUCCEEDED',
                started_at: '2025-11-21T10:30:45Z',
                completed_at: '2025-11-21T10:35:45Z'
            }
        ];

        vi.mocked(mockApiClient.listExecutions as any).mockResolvedValue(mockExecutions);

        const { container } = render(ListExecutionsView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        await waitFor(() => {
            // Check that the date format appears in the container
            expect(container.textContent).toMatch(/2025/);
            // Check that we rendered some table cells
            const cells = container.querySelectorAll('td');
            expect(cells.length).toBeGreaterThan(0);
        });
    });

    it('should show error message on API failure', async () => {
        const errorMessage = 'Failed to fetch executions';
        vi.mocked(mockApiClient.listExecutions as any).mockRejectedValue(new Error(errorMessage));

        render(ListExecutionsView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        await waitFor(() => {
            expect(screen.getByText(errorMessage)).toBeInTheDocument();
        });
    });

    it('should handle API error with details', async () => {
        const error = new Error('API Error') as any;
        error.details = { error: 'Unauthorized access' };

        vi.mocked(mockApiClient.listExecutions as any).mockRejectedValue(error);

        render(ListExecutionsView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        await waitFor(() => {
            expect(screen.getByText('Unauthorized access')).toBeInTheDocument();
        });
    });

    it('should refresh executions on button click', async () => {
        const mockExecutions: Execution[] = [
            {
                execution_id: 'exec-initial-id',
                status: 'SUCCEEDED',
                started_at: '2025-11-21T10:00:00Z'
            }
        ];

        vi.mocked(mockApiClient.listExecutions as any).mockResolvedValue(mockExecutions);

        const { container } = render(ListExecutionsView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        await waitFor(() => {
            // ID is truncated to first 8 chars
            // 'exec-initial-id' becomes 'exec-in'
            expect(container.textContent).toContain('exec-in');
        });

        // Update mock to return different data
        const updatedExecutions: Execution[] = [
            {
                execution_id: 'exec-updated-id',
                status: 'RUNNING',
                started_at: '2025-11-21T11:00:00Z'
            }
        ];

        vi.mocked(mockApiClient.listExecutions as any).mockResolvedValue(updatedExecutions);

        const refreshButton = screen.getByText(/Refresh/);
        fireEvent.click(refreshButton);

        await waitFor(() => {
            // Updated ID is truncated to first 8 chars
            // 'exec-updated-id' becomes 'exec-upd'
            expect(container.textContent).toContain('exec-upd');
        });
    });

    it('should handle execution without exit code', async () => {
        const mockExecutions: Execution[] = [
            {
                execution_id: 'exec-running-123',
                status: 'RUNNING',
                started_at: '2025-11-21T10:00:00Z'
            }
        ];

        vi.mocked(mockApiClient.listExecutions as any).mockResolvedValue(mockExecutions);

        render(ListExecutionsView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        await waitFor(() => {
            const cells = screen.getAllByText('-');
            expect(cells.length).toBeGreaterThan(0); // Should show dash for missing exit code
        });
    });

    it('should display correct status badge colors', async () => {
        const mockExecutions: Execution[] = [
            {
                execution_id: 'exec-success',
                status: 'SUCCEEDED',
                started_at: '2025-11-21T10:00:00Z'
            },
            {
                execution_id: 'exec-failed',
                status: 'FAILED',
                started_at: '2025-11-21T10:00:00Z'
            },
            {
                execution_id: 'exec-running',
                status: 'RUNNING',
                started_at: '2025-11-21T10:00:00Z'
            }
        ];

        vi.mocked(mockApiClient.listExecutions as any).mockResolvedValue(mockExecutions);

        const { container } = render(ListExecutionsView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        await waitFor(() => {
            const badges = container.querySelectorAll('.status-badge');
            expect(badges.length).toBe(3);

            const successBadges = container.querySelectorAll('.status-success');
            const dangerBadges = container.querySelectorAll('.status-danger');
            const infoBadges = container.querySelectorAll('.status-info');

            expect(successBadges.length).toBe(1);
            expect(dangerBadges.length).toBe(1);
            expect(infoBadges.length).toBe(1);
        });
    });

    it('should have View button for each execution', async () => {
        const mockExecutions: Execution[] = [
            {
                execution_id: 'exec-btn-test-1',
                status: 'SUCCEEDED',
                started_at: '2025-11-21T10:00:00Z'
            },
            {
                execution_id: 'exec-btn-test-2',
                status: 'FAILED',
                started_at: '2025-11-21T10:00:00Z'
            }
        ];

        vi.mocked(mockApiClient.listExecutions as any).mockResolvedValue(mockExecutions);

        render(ListExecutionsView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        await waitFor(() => {
            const viewButtons = screen.getAllByText('View');
            expect(viewButtons.length).toBe(2);
        });
    });

    it('should disable refresh button while loading', async () => {
        let resolveListExecutions: () => void;
        const listExecutionsPromise = new Promise<void>((resolve) => {
            resolveListExecutions = resolve;
        });

        vi.mocked(mockApiClient.listExecutions as any).mockImplementation(() => {
            return listExecutionsPromise as any;
        });

        render(ListExecutionsView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        const refreshButton = screen.getByText(/⟳/);
        expect(refreshButton).toBeDisabled();

        resolveListExecutions!();

        await waitFor(() => {
            expect(refreshButton).not.toBeDisabled();
        });
    });

    it('should show loading state message', async () => {
        const slowPromise = new Promise(() => {}); // Never resolves
        vi.mocked(mockApiClient.listExecutions as any).mockReturnValue(slowPromise as any);

        render(ListExecutionsView, {
            props: {
                apiClient: mockApiClient as APIClient,
                isConfigured: true
            }
        });

        expect(screen.getByText(/Loading executions/)).toBeInTheDocument();
    });
});
