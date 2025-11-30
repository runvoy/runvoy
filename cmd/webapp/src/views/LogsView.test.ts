/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';
import { get } from 'svelte/store';
import LogsView from './LogsView.svelte';
import type APIClient from '../lib/api';
import { executionId } from '../stores/execution';
import { cachedWebSocketURL, isConnected, isConnecting } from '../stores/websocket';
import { logEvents } from '../stores/logs';

describe('LogsView', () => {
    let mockApiClient: Partial<APIClient>;
    let fetchLogsSpy: ReturnType<typeof vi.fn> & ((executionId: string) => Promise<any>);

    beforeEach(() => {
        fetchLogsSpy = vi.fn().mockResolvedValue({
            events: [],
            websocket_url: null
        }) as ReturnType<typeof vi.fn> & ((executionId: string) => Promise<any>);

        mockApiClient = {
            getLogs: fetchLogsSpy,
            getExecutionStatus: vi.fn().mockResolvedValue({
                execution_id: 'exec-123',
                status: 'RUNNING'
            })
        };

        // Reset stores
        executionId.set(null);
        cachedWebSocketURL.set(null);
        isConnected.set(false);
        isConnecting.set(false);
        logEvents.set([]);
    });

    afterEach(() => {
        vi.clearAllMocks();
    });

    it('should skip fetching logs when WebSocket is already connected', async () => {
        isConnected.set(true);

        render(LogsView, {
            props: {
                apiClient: mockApiClient as APIClient,
                currentExecutionId: 'exec-123'
            }
        });

        // Allow effects to run
        await new Promise((resolve) => setTimeout(resolve, 50));

        expect(fetchLogsSpy).not.toHaveBeenCalled();
    });

    it('should fetch logs when execution ID is provided and no WebSocket', async () => {
        render(LogsView, {
            props: {
                apiClient: mockApiClient as APIClient,
                currentExecutionId: 'exec-123'
            }
        });

        // Wait for fetchLogs to be called
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
                apiClient: mockApiClient as APIClient,
                currentExecutionId: null
            }
        });

        // Allow effects to run
        await new Promise((resolve) => setTimeout(resolve, 50));

        expect(fetchLogsSpy).not.toHaveBeenCalled();
    });

    it('should not fetch logs when apiClient is null', async () => {
        render(LogsView, {
            props: {
                apiClient: null,
                currentExecutionId: 'exec-123'
            }
        });

        // Allow effects to run
        await new Promise((resolve) => setTimeout(resolve, 50));

        expect(fetchLogsSpy).not.toHaveBeenCalled();
    });

    describe('handleExecutionComplete', () => {
        it('should fetch status when execution-complete event fires', async () => {
            const getExecutionStatusSpy = vi.fn().mockResolvedValue({
                execution_id: 'exec-123',
                status: 'SUCCEEDED',
                started_at: '2024-01-01T00:00:00Z'
            });

            mockApiClient.getExecutionStatus = getExecutionStatusSpy;

            render(LogsView, {
                props: {
                    apiClient: mockApiClient as APIClient,
                    currentExecutionId: 'exec-123'
                }
            });

            // Wait for component to mount
            await new Promise((resolve) => setTimeout(resolve, 100));

            // Simulate execution-complete event
            const event = new CustomEvent('runvoy:execution-complete');
            window.dispatchEvent(event);

            // Wait for status to be fetched
            await waitFor(
                () => {
                    expect(getExecutionStatusSpy).toHaveBeenCalledWith('exec-123');
                },
                { timeout: 1000 }
            );
        });

        it('should not fetch status when apiClient is null', async () => {
            const getExecutionStatusSpy = vi.fn();

            render(LogsView, {
                props: {
                    apiClient: null,
                    currentExecutionId: 'exec-123'
                }
            });

            // Wait for component to mount
            await new Promise((resolve) => setTimeout(resolve, 100));

            // Simulate execution-complete event
            const event = new CustomEvent('runvoy:execution-complete');
            window.dispatchEvent(event);

            // Wait a bit
            await new Promise((resolve) => setTimeout(resolve, 100));

            expect(getExecutionStatusSpy).not.toHaveBeenCalled();
        });

        it('should not fetch status when currentExecutionId is null', async () => {
            const getExecutionStatusSpy = vi.fn().mockResolvedValue({
                execution_id: 'exec-123',
                status: 'SUCCEEDED'
            });

            mockApiClient.getExecutionStatus = getExecutionStatusSpy;

            render(LogsView, {
                props: {
                    apiClient: mockApiClient as APIClient,
                    currentExecutionId: null
                }
            });

            // Wait for component to mount
            await new Promise((resolve) => setTimeout(resolve, 100));

            // Simulate execution-complete event
            const event = new CustomEvent('runvoy:execution-complete');
            window.dispatchEvent(event);

            // Wait a bit
            await new Promise((resolve) => setTimeout(resolve, 100));

            expect(getExecutionStatusSpy).not.toHaveBeenCalled();
        });
    });

    describe('fetch deduplication', () => {
        it('should not fetch logs twice for the same execution ID', async () => {
            render(LogsView, {
                props: {
                    apiClient: mockApiClient as APIClient,
                    currentExecutionId: 'exec-123'
                }
            });

            // Wait for first fetch
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
                    apiClient: mockApiClient as APIClient,
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

        it('should set error message when ExecutionStatusResponse is missing status', async () => {
            const getExecutionStatusSpy = vi.fn().mockResolvedValue({
                execution_id: 'exec-123'
                // status is missing
            });

            mockApiClient.getExecutionStatus = getExecutionStatusSpy;

            const { container } = render(LogsView, {
                props: {
                    apiClient: mockApiClient as APIClient,
                    currentExecutionId: 'exec-123'
                }
            });

            // Wait for component to mount
            await new Promise((resolve) => setTimeout(resolve, 100));

            // Simulate execution-complete event
            const event = new CustomEvent('runvoy:execution-complete');
            window.dispatchEvent(event);

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

        it('should clear logs when LogsResponse is missing status', async () => {
            // Set some initial logs
            logEvents.set([
                { message: 'test log', timestamp: 1234567890, event_id: 'evt-1', line: 1 }
            ]);

            fetchLogsSpy.mockResolvedValue({
                events: [{ message: 'test log', timestamp: 1234567890, event_id: 'evt-1' }],
                websocket_url: null
                // status is missing
            });

            render(LogsView, {
                props: {
                    apiClient: mockApiClient as APIClient,
                    currentExecutionId: 'exec-123'
                }
            });

            await waitFor(
                () => {
                    expect(fetchLogsSpy).toHaveBeenCalled();
                },
                { timeout: 1000 }
            );

            // Logs should be cleared when status is missing
            await waitFor(
                () => {
                    const events = get(logEvents);
                    expect(events).toEqual([]);
                },
                { timeout: 1000 }
            );
        });
    });
});
