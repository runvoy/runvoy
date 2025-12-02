/// <reference types="vitest" />

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { waitFor } from '@testing-library/svelte';
import { connectWebSocket, disconnectWebSocket, type WebSocketCallbacks } from './websocket';
import {
    websocketConnection,
    isConnecting,
    connectionError,
    isConnected
} from '../stores/websocket';
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

function createMockCallbacks(): WebSocketCallbacks {
    return {
        onLogEvent: vi.fn(),
        onExecutionComplete: vi.fn(),
        onStatusRunning: vi.fn(),
        onError: vi.fn()
    };
}

describe('WebSocket Connection', () => {
    let originalWebSocket: typeof WebSocket;
    let mockCallbacks: WebSocketCallbacks;

    beforeEach(() => {
        // Store original WebSocket
        originalWebSocket = globalThis.WebSocket as typeof WebSocket;

        // Replace WebSocket with mock
        globalThis.WebSocket = MockWebSocket as any;

        // Reset stores
        websocketConnection.set(null);
        isConnecting.set(false);
        connectionError.set(null);
        isConnected.set(false);

        // Create fresh mock callbacks
        mockCallbacks = createMockCallbacks();

        vi.clearAllMocks();
    });

    afterEach(() => {
        // Restore original WebSocket
        globalThis.WebSocket = originalWebSocket;

        // Clean up any connections
        disconnectWebSocket();
    });

    describe('connectWebSocket', () => {
        it('should reset manuallyDisconnected flag when connecting', async () => {
            const url = 'wss://localhost:8080/logs';
            connectWebSocket(url, mockCallbacks);

            // Wait for connection to open
            await new Promise((resolve) => setTimeout(resolve, 10));

            // Manually disconnect
            disconnectWebSocket();

            // Wait for disconnect
            await new Promise((resolve) => setTimeout(resolve, 10));

            // Connect again - manuallyDisconnected should be reset
            connectWebSocket(url, mockCallbacks);

            await new Promise((resolve) => setTimeout(resolve, 10));

            // Verify connection was established
            expect(get(websocketConnection)).not.toBeNull();
        });

        it('should call onExecutionComplete when disconnect message is received', async () => {
            const url = 'wss://localhost:8080/logs';

            connectWebSocket(url, mockCallbacks);

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

            // Verify callback was called
            expect(mockCallbacks.onExecutionComplete).toHaveBeenCalledTimes(1);
        });

        it('should call onExecutionComplete on clean close (code 1000) when not manually disconnected', async () => {
            const url = 'wss://localhost:8080/logs';

            connectWebSocket(url, mockCallbacks);

            // Wait for connection to be set in store
            await waitFor(() => {
                const ws = get(websocketConnection);
                expect(ws).not.toBeNull();
            });

            const ws = get(websocketConnection) as unknown as MockWebSocket;

            // Simulate clean close without disconnect message
            ws.simulateClose(1000, 'Normal closure');

            await new Promise((resolve) => setTimeout(resolve, 10));

            // Verify callback was called
            expect(mockCallbacks.onExecutionComplete).toHaveBeenCalledTimes(1);
        });

        it('should not call onExecutionComplete on clean close if manually disconnected', async () => {
            const url = 'wss://localhost:8080/logs';

            connectWebSocket(url, mockCallbacks);

            // Wait for connection to open
            await new Promise((resolve) => setTimeout(resolve, 10));

            // Manually disconnect
            disconnectWebSocket();

            await new Promise((resolve) => setTimeout(resolve, 10));

            // Verify callback was NOT called
            expect(mockCallbacks.onExecutionComplete).not.toHaveBeenCalled();
        });

        it('should set connection error on non-clean close (code !== 1000)', async () => {
            const url = 'wss://localhost:8080/logs';
            connectWebSocket(url, mockCallbacks);

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
            expect(mockCallbacks.onError).toHaveBeenCalled();
        });

        it('should not set connection error on clean close (code 1000)', async () => {
            const url = 'wss://localhost:8080/logs';
            connectWebSocket(url, mockCallbacks);

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
            connectWebSocket(url, mockCallbacks);

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
            connectWebSocket(url, mockCallbacks);

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

        it('should handle disconnectWebSocket when socket is null', () => {
            // Should not throw when no socket exists
            expect(() => disconnectWebSocket()).not.toThrow();
        });
    });

    describe('WebSocket message handling', () => {
        it('should call onLogEvent callback with log event', async () => {
            const url = 'wss://localhost:8080/logs';
            connectWebSocket(url, mockCallbacks);

            await waitFor(() => {
                const ws = get(websocketConnection);
                expect(ws).not.toBeNull();
            });

            const ws = get(websocketConnection) as unknown as MockWebSocket;

            const logMessage = {
                message: 'Test log message',
                timestamp: 1234567890,
                event_id: 'event-1'
            };

            ws.simulateMessage(JSON.stringify(logMessage));

            await waitFor(() => {
                expect(mockCallbacks.onLogEvent).toHaveBeenCalledTimes(1);
                expect(mockCallbacks.onLogEvent).toHaveBeenCalledWith({
                    message: 'Test log message',
                    timestamp: 1234567890,
                    event_id: 'event-1',
                    line: 0 // Line is assigned as 0, view will assign actual line number
                });
            });
        });

        it('should call onStatusRunning callback when receiving first log message', async () => {
            const url = 'wss://localhost:8080/logs';
            connectWebSocket(url, mockCallbacks);

            await waitFor(() => {
                const ws = get(websocketConnection);
                expect(ws).not.toBeNull();
            });

            const ws = get(websocketConnection) as unknown as MockWebSocket;

            const logMessage = {
                message: 'First log',
                timestamp: 1234567890,
                event_id: 'event-1'
            };

            ws.simulateMessage(JSON.stringify(logMessage));

            await waitFor(() => {
                expect(mockCallbacks.onStatusRunning).toHaveBeenCalledTimes(1);
            });
        });

        it('should call onStatusRunning for each log message (view handles dedup)', async () => {
            const url = 'wss://localhost:8080/logs';
            connectWebSocket(url, mockCallbacks);

            await waitFor(() => {
                const ws = get(websocketConnection);
                expect(ws).not.toBeNull();
            });

            const ws = get(websocketConnection) as unknown as MockWebSocket;

            // Send multiple log messages
            ws.simulateMessage(
                JSON.stringify({
                    message: 'First log',
                    timestamp: 1234567890,
                    event_id: 'event-1'
                })
            );

            ws.simulateMessage(
                JSON.stringify({
                    message: 'Second log',
                    timestamp: 1234567891,
                    event_id: 'event-2'
                })
            );

            ws.simulateMessage(
                JSON.stringify({
                    message: 'Third log',
                    timestamp: 1234567892,
                    event_id: 'event-3'
                })
            );

            await waitFor(() => {
                expect(mockCallbacks.onLogEvent).toHaveBeenCalledTimes(3);
                expect(mockCallbacks.onStatusRunning).toHaveBeenCalledTimes(3);
            });
        });

        it('should call onError callback for invalid JSON in message', async () => {
            const url = 'wss://localhost:8080/logs';
            connectWebSocket(url, mockCallbacks);

            await waitFor(() => {
                const ws = get(websocketConnection);
                expect(ws).not.toBeNull();
            });

            const ws = get(websocketConnection) as unknown as MockWebSocket;

            // Send invalid JSON
            ws.simulateMessage('invalid json');

            await waitFor(() => {
                const error = get(connectionError);
                expect(error).toContain('Received invalid data from WebSocket server');
                expect(mockCallbacks.onError).toHaveBeenCalled();
            });
        });

        it('should not call onLogEvent for messages without required fields', async () => {
            const url = 'wss://localhost:8080/logs';
            connectWebSocket(url, mockCallbacks);

            await waitFor(() => {
                const ws = get(websocketConnection);
                expect(ws).not.toBeNull();
            });

            const ws = get(websocketConnection) as unknown as MockWebSocket;

            // Send message without event_id
            ws.simulateMessage(
                JSON.stringify({
                    message: 'Test log',
                    timestamp: 1234567890
                })
            );

            // Should not call onLogEvent
            await new Promise((resolve) => setTimeout(resolve, 10));
            expect(mockCallbacks.onLogEvent).not.toHaveBeenCalled();
        });

        it('should not call onLogEvent for messages with non-string event_id', async () => {
            const url = 'wss://localhost:8080/logs';
            connectWebSocket(url, mockCallbacks);

            await waitFor(() => {
                const ws = get(websocketConnection);
                expect(ws).not.toBeNull();
            });

            const ws = get(websocketConnection) as unknown as MockWebSocket;

            // Send message with numeric event_id
            ws.simulateMessage(
                JSON.stringify({
                    message: 'Test log',
                    timestamp: 1234567890,
                    event_id: 123
                })
            );

            // Should not call onLogEvent
            await new Promise((resolve) => setTimeout(resolve, 10));
            expect(mockCallbacks.onLogEvent).not.toHaveBeenCalled();
        });
    });

    describe('WebSocket error handling', () => {
        it('should handle WebSocket constructor error', () => {
            // Mock WebSocket constructor to throw
            const originalWebSocket = globalThis.WebSocket;
            globalThis.WebSocket = class {
                constructor() {
                    throw new Error('Invalid URL');
                }
            } as any;

            connectWebSocket('invalid-url', mockCallbacks);

            // Should set error
            expect(get(connectionError)).toContain('Failed to create WebSocket connection');
            expect(get(isConnecting)).toBe(false);
            expect(mockCallbacks.onError).toHaveBeenCalled();

            // Restore
            globalThis.WebSocket = originalWebSocket;
        });

        it('should handle onerror event', async () => {
            const url = 'wss://localhost:8080/logs';
            connectWebSocket(url, mockCallbacks);

            await waitFor(() => {
                const ws = get(websocketConnection);
                expect(ws).not.toBeNull();
            });

            const ws = get(websocketConnection) as unknown as MockWebSocket;

            // Simulate error
            if (ws.onerror) {
                ws.onerror(new Event('error'));
            }

            await waitFor(() => {
                expect(get(connectionError)).toBe('WebSocket connection failed.');
                expect(get(isConnecting)).toBe(false);
                expect(get(isConnected)).toBe(false);
                expect(mockCallbacks.onError).toHaveBeenCalledWith('WebSocket connection failed.');
            });
        });

        it('should handle close event with reason', async () => {
            const url = 'wss://localhost:8080/logs';
            connectWebSocket(url, mockCallbacks);

            await waitFor(() => {
                const ws = get(websocketConnection);
                expect(ws).not.toBeNull();
            });

            const ws = get(websocketConnection) as unknown as MockWebSocket;

            // Simulate close with reason
            ws.simulateClose(1006, 'Connection lost');

            await waitFor(() => {
                const error = get(connectionError);
                expect(error).toContain('Disconnected');
                expect(error).toContain('Connection lost');
            });
        });
    });

    describe('WebSocket connection state', () => {
        it('should not create new connection if already open', async () => {
            const url = 'wss://localhost:8080/logs';
            connectWebSocket(url, mockCallbacks);

            // Wait for connection to be established
            await waitFor(() => {
                expect(get(isConnected)).toBe(true);
            });

            // Try to connect again - should return early since already connected
            connectWebSocket(url, mockCallbacks);

            await new Promise((resolve) => setTimeout(resolve, 10));

            // Should still be connected
            expect(get(isConnected)).toBe(true);
        });

        it('should set connection state correctly on open', async () => {
            const url = 'wss://localhost:8080/logs';
            connectWebSocket(url, mockCallbacks);

            await waitFor(() => {
                expect(get(isConnecting)).toBe(false);
                expect(get(isConnected)).toBe(true);
                expect(get(connectionError)).toBeNull();
            });
        });
    });
});
