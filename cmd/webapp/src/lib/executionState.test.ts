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
            switchExecution('exec-123');

            expect(get(executionStore.executionId)).toBe('exec-123');
        });

        it('should trim whitespace from execution ID', () => {
            switchExecution('  exec-456  ');

            expect(get(executionStore.executionId)).toBe('exec-456');
        });

        it('should close existing WebSocket before switching', () => {
            websocketStore.websocketConnection.set(mockWebSocket as WebSocket);

            switchExecution('exec-new');

            expect(mockWebSocket.close).toHaveBeenCalled();
            expect(get(websocketStore.websocketConnection)).toBeNull();
        });

        it('should reset execution data on switch', () => {
            // Set up some data
            logsStore.logEvents.set([
                { message: 'test', timestamp: 1000, event_id: 'event-1', line: 1 }
            ]);
            logsStore.logsRetryCount.set(2);
            websocketStore.cachedWebSocketURL.set('wss://example.com');

            switchExecution('exec-new');

            expect(get(logsStore.logEvents)).toEqual([]);
            expect(get(logsStore.logsRetryCount)).toBe(0);
            expect(get(websocketStore.cachedWebSocketURL)).toBeNull();
        });

        it('should call disconnectWebSocket', () => {
            switchExecution('exec-123');

            expect(websocketLib.disconnectWebSocket).toHaveBeenCalled();
        });

        it('should update document title', () => {
            switchExecution('exec-123');

            expect(document.title).toBe('runvoy Logs - exec-123');
        });

        it('should not switch to same execution ID', () => {
            switchExecution('exec-123');
            const initialTitle = document.title;

            // Try to switch to same ID
            vi.clearAllMocks();
            websocketStore.websocketConnection.set(mockWebSocket as WebSocket);

            switchExecution('exec-123');

            // WebSocket should not be closed (already same ID)
            expect(mockWebSocket.close).not.toHaveBeenCalled();
            expect(document.title).toBe(initialTitle);
        });

        it('should clear execution when passed empty string', () => {
            switchExecution('exec-123');
            switchExecution('');

            expect(get(executionStore.executionId)).toBeNull();
            expect(document.title).toBe('runvoy Logs');
        });

        it('should clear execution when passed whitespace', () => {
            switchExecution('exec-123');
            switchExecution('   ');

            expect(get(executionStore.executionId)).toBeNull();
        });

        it('should set active view to LOGS', () => {
            switchExecution('exec-123');

            expect(get(uiStore.activeView)).toBe(uiStore.VIEWS.LOGS);
        });
    });

    describe('clearExecution', () => {
        it('should clear execution ID', () => {
            executionStore.executionId.set('exec-123');

            clearExecution();

            expect(get(executionStore.executionId)).toBeNull();
        });

        it('should close existing WebSocket', () => {
            websocketStore.websocketConnection.set(mockWebSocket as WebSocket);

            clearExecution();

            expect(mockWebSocket.close).toHaveBeenCalled();
            expect(get(websocketStore.websocketConnection)).toBeNull();
        });

        it('should reset execution data', () => {
            logsStore.logEvents.set([
                { message: 'test', timestamp: 1000, event_id: 'event-1', line: 1 }
            ]);
            logsStore.logsRetryCount.set(3);
            websocketStore.cachedWebSocketURL.set('wss://example.com');

            clearExecution();

            expect(get(logsStore.logEvents)).toEqual([]);
            expect(get(logsStore.logsRetryCount)).toBe(0);
            expect(get(websocketStore.cachedWebSocketURL)).toBeNull();
        });

        it('should call disconnectWebSocket', () => {
            clearExecution();

            expect(websocketLib.disconnectWebSocket).toHaveBeenCalled();
        });

        it('should reset document title', () => {
            document.title = 'runvoy Logs - exec-123';

            clearExecution();

            expect(document.title).toBe('runvoy Logs');
        });

        it('should handle clearing when no execution is active', () => {
            expect(get(executionStore.executionId)).toBeNull();

            // Should not throw
            expect(() => clearExecution()).not.toThrow();
        });
    });

    describe('execution flow', () => {
        it('should switch and then clear execution', () => {
            switchExecution('exec-123');
            expect(get(executionStore.executionId)).toBe('exec-123');

            clearExecution();
            expect(get(executionStore.executionId)).toBeNull();
        });

        it('should handle multiple switches', () => {
            switchExecution('exec-1');
            expect(get(executionStore.executionId)).toBe('exec-1');

            switchExecution('exec-2');
            expect(get(executionStore.executionId)).toBe('exec-2');

            switchExecution('exec-3');
            expect(get(executionStore.executionId)).toBe('exec-3');
        });

        it('should close WebSocket on each switch', () => {
            websocketStore.websocketConnection.set(mockWebSocket as WebSocket);
            switchExecution('exec-1');
            expect(mockWebSocket.close).toHaveBeenCalledTimes(1);

            const mockWebSocket2 = { close: vi.fn(), readyState: 1 } as any;
            websocketStore.websocketConnection.set(mockWebSocket2);
            switchExecution('exec-2');
            expect(mockWebSocket2.close).toHaveBeenCalledTimes(1);
        });
    });
});
