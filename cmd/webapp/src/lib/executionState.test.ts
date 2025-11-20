import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { switchExecution, clearExecution } from './executionState';
import { get } from 'svelte/store';
import * as executionStore from '../stores/execution';
import * as logsStore from '../stores/logs';
import * as websocketStore from '../stores/websocket';
import * as uiStore from '../stores/ui';
import * as websocketLib from './websocket';

// Mock the websocket lib
vi.mock('./websocket', () => ({
    disconnectWebSocket: vi.fn()
}));

describe('Execution State Management', () => {
    let mockWebSocket: Partial<WebSocket>;

    beforeEach(() => {
        // Reset all stores
        executionStore.executionId.set(null);
        executionStore.executionStatus.set(null);
        executionStore.isCompleted.set(false);
        executionStore.startedAt.set(null);
        logsStore.logEvents.set([]);
        logsStore.logsRetryCount.set(0);
        websocketStore.websocketConnection.set(null);
        websocketStore.cachedWebSocketURL.set(null);
        websocketStore.isConnecting.set(false);
        websocketStore.connectionError.set(null);

        // Create mock WebSocket
        mockWebSocket = {
            close: vi.fn(),
            readyState: 1
        };

        // Reset mocks
        vi.clearAllMocks();
    });

    afterEach(() => {
        vi.clearAllMocks();
    });

    describe('switchExecution', () => {
        it('should switch to a new execution ID', () => {
            switchExecution('exec-123', { updateHistory: false });

            expect(get(executionStore.executionId)).toBe('exec-123');
        });

        it('should trim whitespace from execution ID', () => {
            switchExecution('  exec-456  ', { updateHistory: false });

            expect(get(executionStore.executionId)).toBe('exec-456');
        });

        it('should close existing WebSocket before switching', () => {
            websocketStore.websocketConnection.set(mockWebSocket as WebSocket);

            switchExecution('exec-new', { updateHistory: false });

            expect(mockWebSocket.close).toHaveBeenCalled();
            expect(get(websocketStore.websocketConnection)).toBeNull();
        });

        it('should reset execution data on switch', () => {
            // Set up some data
            logsStore.logEvents.set([{ message: 'test', timestamp: 1000 }]);
            logsStore.logsRetryCount.set(2);
            websocketStore.cachedWebSocketURL.set('wss://example.com');

            switchExecution('exec-new', { updateHistory: false });

            expect(get(logsStore.logEvents)).toEqual([]);
            expect(get(logsStore.logsRetryCount)).toBe(0);
            expect(get(websocketStore.cachedWebSocketURL)).toBeNull();
        });

        it('should call disconnectWebSocket', () => {
            switchExecution('exec-123', { updateHistory: false });

            expect(websocketLib.disconnectWebSocket).toHaveBeenCalled();
        });

        it('should update document title', () => {
            const originalTitle = document.title;

            switchExecution('exec-123', { updateHistory: false });

            expect(document.title).toBe('runvoy Logs - exec-123');
        });

        it('should not switch to same execution ID', () => {
            switchExecution('exec-123', { updateHistory: false });
            const initialTitle = document.title;

            // Try to switch to same ID
            vi.clearAllMocks();
            websocketStore.websocketConnection.set(mockWebSocket as WebSocket);

            switchExecution('exec-123', { updateHistory: false });

            // WebSocket should not be closed (already same ID)
            expect(mockWebSocket.close).not.toHaveBeenCalled();
            expect(document.title).toBe(initialTitle);
        });

        it('should clear execution when passed empty string', () => {
            switchExecution('exec-123', { updateHistory: false });
            switchExecution('', { updateHistory: false });

            expect(get(executionStore.executionId)).toBeNull();
            expect(document.title).toBe('runvoy Logs');
        });

        it('should clear execution when passed whitespace', () => {
            switchExecution('exec-123', { updateHistory: false });
            switchExecution('   ', { updateHistory: false });

            expect(get(executionStore.executionId)).toBeNull();
        });

        it('should set active view to LOGS', () => {
            switchExecution('exec-123', { updateHistory: false });

            expect(get(uiStore.activeView)).toBe(uiStore.VIEWS.LOGS);
        });

        it('should handle updateHistory flag', () => {
            const pushStateSpy = vi.spyOn(window.history, 'pushState');

            switchExecution('exec-123', { updateHistory: true });

            expect(pushStateSpy).toHaveBeenCalled();

            pushStateSpy.mockRestore();
        });

        it('should not update history when updateHistory is false', () => {
            const pushStateSpy = vi.spyOn(window.history, 'pushState');

            switchExecution('exec-123', { updateHistory: false });

            expect(pushStateSpy).not.toHaveBeenCalled();

            pushStateSpy.mockRestore();
        });

        it('should preserve other query params when updating history', () => {
            // Mock window.location
            const originalLocation = window.location;
            delete (window as any).location;
            (window as any).location = {
                search: '?filter=completed',
                pathname: '/app',
                href: 'http://localhost/app?filter=completed'
            };

            const pushStateSpy = vi.spyOn(window.history, 'pushState');

            switchExecution('exec-123', { updateHistory: true });

            expect(pushStateSpy).toHaveBeenCalledWith(
                { executionId: 'exec-123' },
                '',
                expect.stringContaining('execution_id=exec-123')
            );

            pushStateSpy.mockRestore();
        });
    });

    describe('clearExecution', () => {
        it('should clear execution ID', () => {
            executionStore.executionId.set('exec-123');

            clearExecution({ updateHistory: false });

            expect(get(executionStore.executionId)).toBeNull();
        });

        it('should close existing WebSocket', () => {
            websocketStore.websocketConnection.set(mockWebSocket as WebSocket);

            clearExecution({ updateHistory: false });

            expect(mockWebSocket.close).toHaveBeenCalled();
            expect(get(websocketStore.websocketConnection)).toBeNull();
        });

        it('should reset execution data', () => {
            logsStore.logEvents.set([{ message: 'test', timestamp: 1000 }]);
            logsStore.logsRetryCount.set(3);
            websocketStore.cachedWebSocketURL.set('wss://example.com');

            clearExecution({ updateHistory: false });

            expect(get(logsStore.logEvents)).toEqual([]);
            expect(get(logsStore.logsRetryCount)).toBe(0);
            expect(get(websocketStore.cachedWebSocketURL)).toBeNull();
        });

        it('should call disconnectWebSocket', () => {
            clearExecution({ updateHistory: false });

            expect(websocketLib.disconnectWebSocket).toHaveBeenCalled();
        });

        it('should reset document title', () => {
            document.title = 'runvoy Logs - exec-123';

            clearExecution({ updateHistory: false });

            expect(document.title).toBe('runvoy Logs');
        });

        it('should handle updateHistory flag', () => {
            const pushStateSpy = vi.spyOn(window.history, 'pushState');

            clearExecution({ updateHistory: true });

            expect(pushStateSpy).toHaveBeenCalled();

            pushStateSpy.mockRestore();
        });

        it('should not update history when updateHistory is false', () => {
            const pushStateSpy = vi.spyOn(window.history, 'pushState');

            clearExecution({ updateHistory: false });

            expect(pushStateSpy).not.toHaveBeenCalled();

            pushStateSpy.mockRestore();
        });

        it('should remove execution_id from URL params when clearing', () => {
            const pushStateSpy = vi.spyOn(window.history, 'pushState');

            clearExecution({ updateHistory: true });

            expect(pushStateSpy).toHaveBeenCalled();
            const [, , url] = pushStateSpy.mock.calls[0] as any[];
            expect(url).not.toContain('execution_id');

            pushStateSpy.mockRestore();
        });

        it('should preserve other query params when clearing history', () => {
            // This test ensures that clearing execution doesn't remove other params
            const pushStateSpy = vi.spyOn(window.history, 'pushState');

            clearExecution({ updateHistory: true });

            expect(pushStateSpy).toHaveBeenCalled();

            pushStateSpy.mockRestore();
        });

        it('should handle clearing when no execution is active', () => {
            expect(get(executionStore.executionId)).toBeNull();

            // Should not throw
            expect(() => clearExecution({ updateHistory: false })).not.toThrow();
        });
    });

    describe('execution flow', () => {
        it('should switch and then clear execution', () => {
            switchExecution('exec-123', { updateHistory: false });
            expect(get(executionStore.executionId)).toBe('exec-123');

            clearExecution({ updateHistory: false });
            expect(get(executionStore.executionId)).toBeNull();
        });

        it('should handle multiple switches', () => {
            switchExecution('exec-1', { updateHistory: false });
            expect(get(executionStore.executionId)).toBe('exec-1');

            switchExecution('exec-2', { updateHistory: false });
            expect(get(executionStore.executionId)).toBe('exec-2');

            switchExecution('exec-3', { updateHistory: false });
            expect(get(executionStore.executionId)).toBe('exec-3');
        });

        it('should close WebSocket on each switch', () => {
            websocketStore.websocketConnection.set(mockWebSocket as WebSocket);
            switchExecution('exec-1', { updateHistory: false });
            expect(mockWebSocket.close).toHaveBeenCalledTimes(1);

            const mockWebSocket2 = { close: vi.fn(), readyState: 1 } as any;
            websocketStore.websocketConnection.set(mockWebSocket2);
            switchExecution('exec-2', { updateHistory: false });
            expect(mockWebSocket2.close).toHaveBeenCalledTimes(1);
        });
    });
});
