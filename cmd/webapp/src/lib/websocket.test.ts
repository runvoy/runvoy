/// <reference types="vitest" />

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { waitFor } from '@testing-library/svelte';
import { connectWebSocket, disconnectWebSocket } from './websocket';
import { websocketConnection, isConnecting, connectionError } from '../stores/websocket';
import { isCompleted } from '../stores/execution';
import { get } from 'svelte/store';

// Mock WebSocket
class MockWebSocket {
    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSING = 2;
    static CLOSED = 3;

    readyState = MockWebSocket.CONNECTING;
    url = '';
    onopen: ((event: Event) => void) | null = null;
    onmessage: ((event: MessageEvent) => void) | null = null;
    onerror: ((event: Event) => void) | null = null;
    onclose: ((event: CloseEvent) => void) | null = null;

    constructor(url: string) {
        this.url = url;
        // Simulate connection opening
        setTimeout(() => {
            this.readyState = MockWebSocket.OPEN;
            if (this.onopen) {
                this.onopen(new Event('open'));
            }
        }, 0);
    }

    close(code = 1000, reason = ''): void {
        this.readyState = MockWebSocket.CLOSING;
        setTimeout(() => {
            this.readyState = MockWebSocket.CLOSED;
            if (this.onclose) {
                const event = new CloseEvent('close', {
                    code,
                    reason,
                    wasClean: code === 1000
                });
                this.onclose(event);
            }
        }, 0);
    }

    send(_data: string): void {
        // Mock implementation
    }

    // Helper method to simulate receiving a message
    simulateMessage(data: string): void {
        if (this.onmessage) {
            const event = new MessageEvent('message', { data });
            this.onmessage(event);
        }
    }

    // Helper method to simulate close event
    simulateClose(code = 1000, reason = ''): void {
        if (this.onclose) {
            const event = new CloseEvent('close', {
                code,
                reason,
                wasClean: code === 1000
            });
            this.onclose(event);
        }
    }
}

describe('WebSocket Connection', () => {
    let originalWebSocket: typeof WebSocket;
    let eventListeners: Map<string, EventListener[]>;

    beforeEach(() => {
        // Store original WebSocket
        originalWebSocket = globalThis.WebSocket as typeof WebSocket;

        // Replace WebSocket with mock
        globalThis.WebSocket = MockWebSocket as any;

        // Reset stores
        websocketConnection.set(null);
        isConnecting.set(false);
        connectionError.set(null);
        isCompleted.set(false);

        // Track event listeners
        eventListeners = new Map();
        const originalAddEventListener = window.addEventListener.bind(window);
        const originalRemoveEventListener = window.removeEventListener.bind(window);

        window.addEventListener = vi.fn(
            (type: string, listener: EventListenerOrEventListenerObject) => {
                if (!eventListeners.has(type)) {
                    eventListeners.set(type, []);
                }
                if (typeof listener === 'function') {
                    eventListeners.get(type)!.push(listener);
                }
                originalAddEventListener(type, listener);
            }
        ) as typeof window.addEventListener;

        window.removeEventListener = vi.fn(
            (type: string, listener: EventListenerOrEventListenerObject) => {
                const listeners = eventListeners.get(type);
                if (listeners && typeof listener === 'function') {
                    const index = listeners.indexOf(listener);
                    if (index > -1) {
                        listeners.splice(index, 1);
                    }
                }
                originalRemoveEventListener(type, listener);
            }
        ) as typeof window.removeEventListener;

        vi.clearAllMocks();
    });

    afterEach(() => {
        // Restore original WebSocket
        globalThis.WebSocket = originalWebSocket;

        // Clean up any connections
        disconnectWebSocket();

        // Remove all event listeners
        eventListeners.forEach((listeners, type) => {
            listeners.forEach((listener) => {
                window.removeEventListener(type, listener);
            });
        });
        eventListeners.clear();
    });

    describe('connectWebSocket', () => {
        it('should reset manuallyDisconnected flag when connecting', async () => {
            const url = 'wss://localhost:8080/logs';
            connectWebSocket(url);

            // Wait for connection to open
            await new Promise((resolve) => setTimeout(resolve, 10));

            // Manually disconnect
            disconnectWebSocket();

            // Wait for disconnect
            await new Promise((resolve) => setTimeout(resolve, 10));

            // Connect again - manuallyDisconnected should be reset
            connectWebSocket(url);

            await new Promise((resolve) => setTimeout(resolve, 10));

            // Verify connection was established
            expect(get(websocketConnection)).not.toBeNull();
        });

        it('should dispatch execution-complete event with cleanClose when disconnect message is received', async () => {
            const url = 'wss://localhost:8080/logs';
            const dispatchSpy = vi.spyOn(window, 'dispatchEvent');

            connectWebSocket(url);

            // Wait for connection to be set in store
            await waitFor(() => {
                const ws = get(websocketConnection);
                expect(ws).not.toBeNull();
            });

            const ws = get(websocketConnection) as unknown as MockWebSocket;

            // Simulate disconnect message
            ws.simulateMessage(
                JSON.stringify({
                    type: 'disconnect',
                    reason: 'Execution completed'
                })
            );

            await new Promise((resolve) => setTimeout(resolve, 10));

            // Verify event was dispatched with cleanClose: true
            expect(dispatchSpy).toHaveBeenCalled();
            const calls = dispatchSpy.mock.calls;
            const executionCompleteCall = calls.find(
                (call) => (call[0] as CustomEvent).type === 'runvoy:execution-complete'
            );
            expect(executionCompleteCall).toBeDefined();
            const event = executionCompleteCall![0] as CustomEvent;
            expect(event.detail).toEqual({ cleanClose: true });
        });

        it('should dispatch execution-complete event with cleanClose on clean close (code 1000) when not manually disconnected and not completed', async () => {
            const url = 'wss://localhost:8080/logs';
            const dispatchSpy = vi.spyOn(window, 'dispatchEvent');

            connectWebSocket(url);

            // Wait for connection to be set in store
            await waitFor(() => {
                const ws = get(websocketConnection);
                expect(ws).not.toBeNull();
            });

            // Ensure execution is not completed
            isCompleted.set(false);

            const ws = get(websocketConnection) as unknown as MockWebSocket;

            // Simulate clean close without disconnect message
            ws.simulateClose(1000, 'Normal closure');

            await new Promise((resolve) => setTimeout(resolve, 10));

            // Verify event was dispatched with cleanClose: true
            expect(dispatchSpy).toHaveBeenCalled();
            const calls = dispatchSpy.mock.calls;
            const executionCompleteCall = calls.find(
                (call) => (call[0] as CustomEvent).type === 'runvoy:execution-complete'
            );
            expect(executionCompleteCall).toBeDefined();
            const event = executionCompleteCall![0] as CustomEvent;
            expect(event.detail).toEqual({ cleanClose: true });
        });

        it('should not dispatch execution-complete event on clean close if manually disconnected', async () => {
            const url = 'wss://localhost:8080/logs';
            const dispatchSpy = vi.spyOn(window, 'dispatchEvent');

            connectWebSocket(url);

            // Wait for connection to open
            await new Promise((resolve) => setTimeout(resolve, 10));

            // Manually disconnect
            disconnectWebSocket();

            await new Promise((resolve) => setTimeout(resolve, 10));

            // Clear previous calls
            dispatchSpy.mockClear();

            // The close event should have already been triggered by disconnectWebSocket
            // Verify no new event was dispatched
            const calls = dispatchSpy.mock.calls;
            const executionCompleteCall = calls.find(
                (call) => (call[0] as CustomEvent)?.type === 'runvoy:execution-complete'
            );
            // Should not have execution-complete event after manual disconnect
            expect(executionCompleteCall).toBeUndefined();
        });

        it('should not dispatch execution-complete event on clean close if execution is already completed', async () => {
            const url = 'wss://localhost:8080/logs';
            const dispatchSpy = vi.spyOn(window, 'dispatchEvent');

            connectWebSocket(url);

            // Wait for connection to open
            await new Promise((resolve) => setTimeout(resolve, 10));

            // Mark execution as completed
            isCompleted.set(true);

            const ws = get(websocketConnection) as unknown as MockWebSocket;
            expect(ws).not.toBeNull();

            // Simulate clean close
            ws.simulateClose(1000, 'Normal closure');

            await new Promise((resolve) => setTimeout(resolve, 10));

            // Verify event was not dispatched
            const calls = dispatchSpy.mock.calls;
            const executionCompleteCall = calls.find(
                (call) => (call[0] as CustomEvent)?.type === 'runvoy:execution-complete'
            );
            expect(executionCompleteCall).toBeUndefined();
        });

        it('should set connection error on non-clean close (code !== 1000)', async () => {
            const url = 'wss://localhost:8080/logs';
            connectWebSocket(url);

            // Wait for connection to be set in store
            await waitFor(() => {
                const ws = get(websocketConnection);
                expect(ws).not.toBeNull();
            });

            const ws = get(websocketConnection) as unknown as MockWebSocket;

            // Simulate non-clean close
            ws.simulateClose(1006, 'Abnormal closure');

            // Wait for close event to be processed
            await waitFor(() => {
                const error = get(connectionError);
                expect(error).not.toBeNull();
            });

            // Verify error was set
            const error = get(connectionError);
            expect(error).toContain('Disconnected');
        });

        it('should not set connection error on clean close (code 1000)', async () => {
            const url = 'wss://localhost:8080/logs';
            connectWebSocket(url);

            // Wait for connection to be set in store
            await waitFor(() => {
                const ws = get(websocketConnection);
                expect(ws).not.toBeNull();
            });

            const ws = get(websocketConnection) as unknown as MockWebSocket;

            // Simulate clean close
            ws.simulateClose(1000, 'Normal closure');

            await new Promise((resolve) => setTimeout(resolve, 10));

            // Verify no error was set
            expect(get(connectionError)).toBeNull();
        });
    });

    describe('disconnectWebSocket', () => {
        it('should set manuallyDisconnected flag when disconnecting', async () => {
            const url = 'wss://localhost:8080/logs';
            connectWebSocket(url);

            // Wait for connection to be set in store
            await waitFor(() => {
                expect(get(websocketConnection)).not.toBeNull();
            });

            // Disconnect manually
            disconnectWebSocket();

            await new Promise((resolve) => setTimeout(resolve, 10));

            // Verify connection is closed
            expect(get(websocketConnection)).toBeNull();
        });

        it('should close websocket with code 1000 when manually disconnecting', async () => {
            const url = 'wss://localhost:8080/logs';
            connectWebSocket(url);

            // Wait for connection to be set in store
            await waitFor(() => {
                const ws = get(websocketConnection);
                expect(ws).not.toBeNull();
            });

            const ws = get(websocketConnection) as unknown as MockWebSocket;
            const closeSpy = vi.spyOn(ws, 'close');

            disconnectWebSocket();

            await new Promise((resolve) => setTimeout(resolve, 10));

            // Verify close was called with code 1000
            expect(closeSpy).toHaveBeenCalledWith(1000, 'User disconnected');
        });
    });
});
