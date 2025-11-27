/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';
import { tick } from 'svelte';
import LogsView from './LogsView.svelte';
import type APIClient from '../lib/api';
import { executionId } from '../stores/execution';
import { cachedWebSocketURL } from '../stores/websocket';

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
    });

    afterEach(() => {
        vi.clearAllMocks();
    });

    describe('handleExecutionComplete', () => {
        it('should not fetch logs when cleanClose is true', async () => {
            render(LogsView, {
                props: {
                    apiClient: mockApiClient as APIClient,
                    isConfigured: true
                }
            });

            // Set execution ID
            executionId.set('exec-123');

            // Wait for component to mount and set up event listener
            await waitFor(
                () => {
                    expect(executionId).toBeDefined();
                },
                { timeout: 1000 }
            );

            // Wait a bit more for onMount to complete
            await new Promise((resolve) => setTimeout(resolve, 100));

            // Clear any initial calls
            fetchLogsSpy.mockClear();

            // Simulate execution-complete event with cleanClose: true
            const event = new CustomEvent('runvoy:execution-complete', {
                detail: { cleanClose: true }
            });
            window.dispatchEvent(event);

            // Wait a bit to ensure any async operations complete
            await new Promise((resolve) => setTimeout(resolve, 200));

            // Verify fetchLogs was not called
            expect(fetchLogsSpy).not.toHaveBeenCalled();
        });

        it.skip('should fetch logs when cleanClose is false', async () => {
            // Set execution ID first so subscription will set currentExecutionId when component mounts
            executionId.set('exec-123');

            render(LogsView, {
                props: {
                    apiClient: mockApiClient as APIClient,
                    isConfigured: true
                }
            });

            // Wait for component to mount and set up event listener
            await waitFor(
                () => {
                    expect(executionId).toBeDefined();
                },
                { timeout: 1000 }
            );

            // Wait a bit more for onMount to complete and subscription to fire
            await new Promise((resolve) => setTimeout(resolve, 150));

            // Set websocketURL so fetchLogs isn't called from subscription
            cachedWebSocketURL.set('wss://example.com/logs');
            await tick();
            await new Promise((resolve) => setTimeout(resolve, 50));

            // Clear any initial calls
            fetchLogsSpy.mockClear();

            // Simulate execution-complete event with cleanClose: false
            const event = new CustomEvent('runvoy:execution-complete', {
                detail: { cleanClose: false }
            });
            window.dispatchEvent(event);
            await tick();
            await new Promise((resolve) => setTimeout(resolve, 100));

            // Wait for fetchLogs to be called
            await waitFor(
                () => {
                    expect(fetchLogsSpy).toHaveBeenCalledWith('exec-123');
                },
                { timeout: 2000 }
            );
        });

        it.skip('should fetch logs when cleanClose is missing', async () => {
            // Set execution ID first so subscription will set currentExecutionId when component mounts
            executionId.set('exec-123');

            render(LogsView, {
                props: {
                    apiClient: mockApiClient as APIClient,
                    isConfigured: true
                }
            });

            // Wait for component to mount and set up event listener
            await waitFor(
                () => {
                    expect(executionId).toBeDefined();
                },
                { timeout: 1000 }
            );

            // Wait a bit more for onMount to complete and subscription to fire
            await new Promise((resolve) => setTimeout(resolve, 150));

            // Set websocketURL so fetchLogs isn't called from subscription
            cachedWebSocketURL.set('wss://example.com/logs');
            await tick();
            await new Promise((resolve) => setTimeout(resolve, 50));

            // Clear any initial calls
            fetchLogsSpy.mockClear();

            // Simulate execution-complete event without cleanClose
            const event = new CustomEvent('runvoy:execution-complete');
            window.dispatchEvent(event);
            await tick();
            await new Promise((resolve) => setTimeout(resolve, 100));

            // Wait for fetchLogs to be called
            await waitFor(
                () => {
                    expect(fetchLogsSpy).toHaveBeenCalledWith('exec-123');
                },
                { timeout: 2000 }
            );
        });

        it('should not fetch logs when apiClient is null', async () => {
            render(LogsView, {
                props: {
                    apiClient: null,
                    isConfigured: true
                }
            });

            // Set execution ID
            executionId.set('exec-123');

            // Wait for component to initialize
            await waitFor(() => {
                expect(executionId).toBeDefined();
            });

            // Clear any initial calls
            fetchLogsSpy.mockClear();

            // Simulate execution-complete event
            const event = new CustomEvent('runvoy:execution-complete', {
                detail: { cleanClose: false }
            });
            window.dispatchEvent(event);

            // Wait a bit to ensure any async operations complete
            await waitFor(
                () => {
                    // Verify fetchLogs was not called (no apiClient)
                    expect(fetchLogsSpy).not.toHaveBeenCalled();
                },
                { timeout: 100 }
            );
        });

        it('should not fetch logs when currentExecutionId is null', async () => {
            render(LogsView, {
                props: {
                    apiClient: mockApiClient as APIClient,
                    isConfigured: true
                }
            });

            // Don't set execution ID (keep it null)

            // Wait for component to initialize
            await waitFor(() => {
                expect(executionId).toBeDefined();
            });

            // Clear any initial calls
            fetchLogsSpy.mockClear();

            // Simulate execution-complete event
            const event = new CustomEvent('runvoy:execution-complete', {
                detail: { cleanClose: false }
            });
            window.dispatchEvent(event);

            // Wait a bit to ensure any async operations complete
            await waitFor(
                () => {
                    // Verify fetchLogs was not called (no execution ID)
                    expect(fetchLogsSpy).not.toHaveBeenCalled();
                },
                { timeout: 100 }
            );
        });

        it('should handle cleanClose: true correctly even when execution ID is set', async () => {
            render(LogsView, {
                props: {
                    apiClient: mockApiClient as APIClient,
                    isConfigured: true
                }
            });

            // Set execution ID
            executionId.set('exec-456');

            // Wait for component to mount and set up event listener
            await waitFor(
                () => {
                    expect(executionId).toBeDefined();
                },
                { timeout: 1000 }
            );

            // Wait a bit more for onMount to complete
            await new Promise((resolve) => setTimeout(resolve, 100));

            // Clear any initial calls
            fetchLogsSpy.mockClear();

            // Simulate execution-complete event with cleanClose: true
            const event = new CustomEvent('runvoy:execution-complete', {
                detail: { cleanClose: true }
            });
            window.dispatchEvent(event);

            // Wait a bit to ensure any async operations complete
            await new Promise((resolve) => setTimeout(resolve, 200));

            // Verify fetchLogs was not called despite having execution ID
            expect(fetchLogsSpy).not.toHaveBeenCalled();
        });
    });
});
