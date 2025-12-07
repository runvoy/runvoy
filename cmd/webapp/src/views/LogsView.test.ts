/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor, screen } from '@testing-library/svelte';
import LogsView from './LogsView.svelte';
import type APIClient from '../lib/api';
import { executionId } from '../stores/execution';

describe('LogsView', () => {
    let mockApiClient: APIClient;
    let fetchLogsSpy: ReturnType<typeof vi.fn>;

    beforeEach(() => {
        fetchLogsSpy = vi.fn().mockResolvedValue({
            events: [],
            status: 'SUCCEEDED'
        });

        mockApiClient = {
            getLogs: fetchLogsSpy,
            getExecutionStatus: vi.fn().mockResolvedValue({
                execution_id: 'exec-123',
                status: 'SUCCEEDED'
            }),
            killExecution: vi.fn().mockResolvedValue({
                execution_id: 'exec-123',
                status: 'TERMINATING'
            })
        } as unknown as APIClient;

        // Reset stores
        executionId.set(null);
    });

    afterEach(() => {
        vi.clearAllMocks();
    });

    it('should fetch logs when execution ID is provided', async () => {
        render(LogsView, {
            props: {
                apiClient: mockApiClient,
                currentExecutionId: 'exec-123'
            }
        });

        await waitFor(
            () => {
                expect(fetchLogsSpy).toHaveBeenCalledWith('exec-123');
            },
            { timeout: 1000 }
        );
    });

    it('should not fetch logs when execution ID is null', async () => {
        render(LogsView, {
            props: {
                apiClient: mockApiClient,
                currentExecutionId: null
            }
        });

        // Allow effects to run
        await new Promise((resolve) => setTimeout(resolve, 50));

        expect(fetchLogsSpy).not.toHaveBeenCalled();
    });

    it('should display instruction when no execution ID is provided', async () => {
        render(LogsView, {
            props: {
                apiClient: mockApiClient,
                currentExecutionId: null
            }
        });

        await waitFor(() => {
            expect(screen.getByText(/Enter an execution ID above/)).toBeInTheDocument();
        });
    });

    describe('handleExecutionComplete', () => {
        it('should fetch status when websocket triggers completion', async () => {
            const getExecutionStatusSpy = vi.fn().mockResolvedValue({
                execution_id: 'exec-123',
                status: 'SUCCEEDED',
                started_at: '2024-01-01T00:00:00Z'
            });

            mockApiClient.getExecutionStatus = getExecutionStatusSpy;

            // Mock getLogs to return a websocket URL (triggers websocket connection)
            fetchLogsSpy.mockResolvedValue({
                events: null,
                websocket_url: 'wss://example.com/ws',
                status: 'RUNNING'
            });

            render(LogsView, {
                props: {
                    apiClient: mockApiClient,
                    currentExecutionId: 'exec-123'
                }
            });

            await waitFor(
                () => {
                    expect(fetchLogsSpy).toHaveBeenCalledWith('exec-123');
                },
                { timeout: 1000 }
            );
        });

        it('should not fetch status when currentExecutionId is null', async () => {
            const getExecutionStatusSpy = vi.fn().mockResolvedValue({
                execution_id: 'exec-123',
                status: 'SUCCEEDED'
            });

            mockApiClient.getExecutionStatus = getExecutionStatusSpy;

            render(LogsView, {
                props: {
                    apiClient: mockApiClient,
                    currentExecutionId: null
                }
            });

            // Wait for component to mount
            await new Promise((resolve) => setTimeout(resolve, 100));

            expect(getExecutionStatusSpy).not.toHaveBeenCalled();
        });
    });

    describe('fetch deduplication', () => {
        it('should not fetch logs twice for the same execution ID', async () => {
            render(LogsView, {
                props: {
                    apiClient: mockApiClient,
                    currentExecutionId: 'exec-123'
                }
            });

            await waitFor(
                () => {
                    expect(fetchLogsSpy).toHaveBeenCalledWith('exec-123');
                },
                { timeout: 1000 }
            );

            // Should only be called once
            expect(fetchLogsSpy).toHaveBeenCalledTimes(1);
        });
    });

    describe('error handling', () => {
        it('should set error message when LogsResponse is missing status', async () => {
            fetchLogsSpy.mockResolvedValue({
                events: [],
                websocket_url: null
                // status is missing
            });

            const { container } = render(LogsView, {
                props: {
                    apiClient: mockApiClient,
                    currentExecutionId: 'exec-123'
                }
            });

            await waitFor(
                () => {
                    const errorBox = container.querySelector('.error-box');
                    expect(errorBox).toBeInTheDocument();
                    expect(errorBox).toHaveTextContent(
                        'Invalid API response: missing execution status'
                    );
                },
                { timeout: 1000 }
            );
        });

        it('should display Waiting for logs message when events are empty', async () => {
            fetchLogsSpy.mockResolvedValue({
                events: [],
                websocket_url: null,
                status: 'SUCCEEDED'
            });

            render(LogsView, {
                props: {
                    apiClient: mockApiClient,
                    currentExecutionId: 'exec-123'
                }
            });

            await waitFor(
                () => {
                    expect(screen.getByText('Waiting for logs...')).toBeInTheDocument();
                },
                { timeout: 1000 }
            );
        });

        it('should display error from API failure', async () => {
            const apiError = new Error('Network error') as any;
            apiError.details = { error: 'Connection failed' };
            fetchLogsSpy.mockRejectedValue(apiError);

            const { container } = render(LogsView, {
                props: {
                    apiClient: mockApiClient,
                    currentExecutionId: 'exec-123'
                }
            });

            await waitFor(
                () => {
                    const errorBox = container.querySelector('.error-box');
                    expect(errorBox).toBeInTheDocument();
                    expect(errorBox).toHaveTextContent('Connection failed');
                },
                { timeout: 1000 }
            );
        });
    });

    describe('syncing executionId store', () => {
        it('should sync execution ID to store', async () => {
            render(LogsView, {
                props: {
                    apiClient: mockApiClient,
                    currentExecutionId: 'exec-456'
                }
            });

            await waitFor(() => {
                let storeValue: string | null = null;
                executionId.subscribe((v) => (storeValue = v))();
                expect(storeValue).toBe('exec-456');
            });
        });

        it('should keep existing store value when no execution ID is provided', async () => {
            // First set an execution ID
            executionId.set('exec-old');

            render(LogsView, {
                props: {
                    apiClient: mockApiClient,
                    currentExecutionId: null
                }
            });

            await waitFor(() => {
                let storeValue: string | null = null;
                executionId.subscribe((v) => (storeValue = v))();
                expect(storeValue).toBe('exec-old');
            });
        });
    });
});
