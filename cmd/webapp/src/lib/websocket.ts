/**
 * WebSocket connection handler for real-time log streaming
 */
import { get } from 'svelte/store';
import {
    websocketConnection,
    isConnecting,
    connectionError,
    isConnected
} from '../stores/websocket';
import { logEvents } from '../stores/logs';
import { isCompleted, executionStatus } from '../stores/execution';
import { ExecutionStatus, FrontendStatus } from '../lib/constants';
import type { LogEvent, WebSocketLogMessage } from '../types/logs';

let socket: WebSocket | null = null;
let manuallyDisconnected = false;

/**
 * Connects to the WebSocket server
 * @param url - The WebSocket URL to connect to
 */
export function connectWebSocket(url: string): void {
    if (socket && socket.readyState === WebSocket.OPEN) {
        return;
    }

    manuallyDisconnected = false;
    isConnecting.set(true);
    connectionError.set(null);
    isConnected.set(false);

    try {
        socket = new WebSocket(url);
    } catch (error) {
        const reason = error instanceof Error ? error.message : 'Invalid URL?';
        connectionError.set(`Failed to create WebSocket connection. ${reason}`);
        isConnecting.set(false);
        return;
    }

    websocketConnection.set(socket);

    socket.onopen = (): void => {
        isConnecting.set(false);
        connectionError.set(null);
        isConnected.set(true);
    };

    socket.onmessage = (event: MessageEvent): void => {
        try {
            const message: WebSocketLogMessage = JSON.parse(event.data);

            // Handle disconnect messages
            if (message.type === 'disconnect') {
                isCompleted.set(true);
                if (typeof window !== 'undefined') {
                    // Pass cleanClose flag to indicate websocket closed cleanly (no need to fetch logs)
                    window.dispatchEvent(
                        new CustomEvent('runvoy:execution-complete', {
                            detail: { cleanClose: true }
                        })
                    );
                }
                // Close the connection gracefully
                if (socket && socket.readyState === WebSocket.OPEN) {
                    socket.close(1000, 'Execution completed');
                }
                return;
            }

            // Handle log events (messages with a message property, timestamp, and event_id)
            // event_id is required for Svelte key tracking in the each block
            const eventId = message.event_id;
            if (
                message.message &&
                message.timestamp !== undefined &&
                eventId &&
                typeof eventId === 'string'
            ) {
                // Update status to RUNNING when we receive the first log message
                // (indicates execution is actually running and producing output)
                const currentStatus = get(executionStatus);
                if (
                    currentStatus === ExecutionStatus.STARTING ||
                    currentStatus === FrontendStatus.LOADING
                ) {
                    executionStatus.set(ExecutionStatus.RUNNING);
                }

                logEvents.update((events: LogEvent[]): LogEvent[] => {
                    // Assign a new line number (like CLI does - just append to the end)
                    const nextLine =
                        events.length > 0 ? Math.max(...events.map((e) => e.line)) + 1 : 1;

                    const eventWithLine: LogEvent = {
                        message: message.message as string,
                        timestamp: message.timestamp as number,
                        event_id: eventId,
                        line: nextLine
                    };

                    // Append the new event (no deduplication needed - API contract ensures no duplicates)
                    return [...events, eventWithLine];
                });
            }
        } catch (error) {
            const reason = error instanceof Error ? error.message : 'Unknown error';
            connectionError.set(`Received invalid data from WebSocket server: ${reason}`);
        }
    };

    socket.onerror = (): void => {
        connectionError.set('WebSocket connection failed.');
        isConnecting.set(false);
        isConnected.set(false);
    };

    socket.onclose = (event: CloseEvent): void => {
        isConnecting.set(false);
        isConnected.set(false);
        if (event.code !== 1000) {
            // 1000 is normal closure
            connectionError.set(`Disconnected: ${event.reason || 'Connection lost'}`);
        } else {
            // If websocket closed cleanly (code 1000) but we didn't receive a disconnect message,
            // and it wasn't a manual disconnect, still dispatch the event but mark it as clean close
            // (logs are already complete)
            if (typeof window !== 'undefined' && !get(isCompleted) && !manuallyDisconnected) {
                window.dispatchEvent(
                    new CustomEvent('runvoy:execution-complete', {
                        detail: { cleanClose: true }
                    })
                );
            }
        }
        websocketConnection.set(null);
    };
}

/**
 * Disconnects the WebSocket connection
 */
export function disconnectWebSocket(): void {
    if (socket) {
        manuallyDisconnected = true;
        socket.close(1000, 'User disconnected');
        socket = null;
        websocketConnection.set(null);
        isConnected.set(false);
    }
}
