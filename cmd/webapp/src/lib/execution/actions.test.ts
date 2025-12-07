import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { createExecutionKiller } from './actions';
import type APIClient from '../api';

function createMockApiClient(): APIClient {
    return {
        killExecution: vi.fn()
    } as unknown as APIClient;
}

describe('createExecutionKiller', () => {
    let mockApiClient: APIClient;
    let killer: ReturnType<typeof createExecutionKiller>;

    beforeEach(() => {
        mockApiClient = createMockApiClient();
        killer = createExecutionKiller(mockApiClient);
    });

    describe('initial state', () => {
        it('should start with clean state', () => {
            const state = get(killer.state);
            expect(state.isKilling).toBe(false);
            expect(state.killInitiated).toBe(false);
            expect(state.error).toBeNull();
        });
    });

    describe('kill', () => {
        it('should set isKilling during request', async () => {
            let resolveKill!: (value?: unknown) => void;
            vi.mocked(mockApiClient.killExecution).mockImplementation(
                (_executionId: string) => new Promise<any>((r) => (resolveKill = r))
            );

            const killPromise = killer.kill('exec-123');

            expect(get(killer.state).isKilling).toBe(true);

            resolveKill();
            await killPromise;

            expect(get(killer.state).isKilling).toBe(false);
        });

        it('should set killInitiated on success', async () => {
            vi.mocked(mockApiClient.killExecution).mockResolvedValue({
                execution_id: 'exec-123',
                status: 'TERMINATING'
            });

            const result = await killer.kill('exec-123');

            expect(result).toBe(true);
            expect(get(killer.state).killInitiated).toBe(true);
            expect(get(killer.state).error).toBeNull();
        });

        it('should call API with correct execution ID', async () => {
            vi.mocked(mockApiClient.killExecution).mockResolvedValue({
                execution_id: 'exec-456',
                status: 'TERMINATING'
            });

            await killer.kill('exec-456');

            expect(mockApiClient.killExecution).toHaveBeenCalledWith('exec-456');
        });

        it('should set error on failure', async () => {
            vi.mocked(mockApiClient.killExecution).mockRejectedValue(new Error('Network error'));

            const result = await killer.kill('exec-123');

            expect(result).toBe(false);
            expect(get(killer.state).killInitiated).toBe(false);
            expect(get(killer.state).error).toBe('Network error');
        });

        it('should use details.error if available', async () => {
            const apiError = new Error('HTTP 403') as any;
            apiError.details = { error: 'Forbidden: insufficient permissions' };
            vi.mocked(mockApiClient.killExecution).mockRejectedValue(apiError);

            await killer.kill('exec-123');

            expect(get(killer.state).error).toBe('Forbidden: insufficient permissions');
        });

        it('should clear previous error on new attempt', async () => {
            vi.mocked(mockApiClient.killExecution).mockRejectedValueOnce(new Error('First error'));
            await killer.kill('exec-123');
            expect(get(killer.state).error).toBe('First error');

            vi.mocked(mockApiClient.killExecution).mockResolvedValueOnce({
                execution_id: 'exec-123',
                status: 'TERMINATING'
            });
            await killer.kill('exec-123');
            expect(get(killer.state).error).toBeNull();
        });

        it('should return false for empty executionId', async () => {
            const result = await killer.kill('');

            expect(result).toBe(false);
            expect(mockApiClient.killExecution).not.toHaveBeenCalled();
        });

        it('should use default error message when none provided', async () => {
            vi.mocked(mockApiClient.killExecution).mockRejectedValue({});

            await killer.kill('exec-123');

            expect(get(killer.state).error).toBe('Failed to stop execution');
        });
    });

    describe('reset', () => {
        it('should reset state after successful kill', async () => {
            vi.mocked(mockApiClient.killExecution).mockResolvedValue({
                execution_id: 'exec-123',
                status: 'TERMINATING'
            });
            await killer.kill('exec-123');

            expect(get(killer.state).killInitiated).toBe(true);

            killer.reset();

            const state = get(killer.state);
            expect(state.isKilling).toBe(false);
            expect(state.killInitiated).toBe(false);
            expect(state.error).toBeNull();
        });

        it('should reset state after failed kill', async () => {
            vi.mocked(mockApiClient.killExecution).mockRejectedValue(new Error('Failed'));
            await killer.kill('exec-123');

            expect(get(killer.state).error).toBe('Failed');

            killer.reset();

            expect(get(killer.state).error).toBeNull();
        });
    });

    describe('multiple instances', () => {
        it('should maintain independent state across instances', async () => {
            const killer2 = createExecutionKiller(mockApiClient);

            vi.mocked(mockApiClient.killExecution).mockResolvedValue({
                execution_id: 'exec-123',
                status: 'TERMINATING'
            });
            await killer.kill('exec-123');

            expect(get(killer.state).killInitiated).toBe(true);
            expect(get(killer2.state).killInitiated).toBe(false);
        });
    });
});
