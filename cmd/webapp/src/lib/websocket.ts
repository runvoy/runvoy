/**
 * WebSocket connection handler for real-time log streaming
 */
import {
    websocketConnection,
    isConnecting,
    connectionError,
    isConnected
} from '../stores/websocket';
import type { LogEvent, WebSocketLogMessage } from '../types/logs';

let socket: WebSocket | null = null;
let manuallyDisconnected = false;

export interface WebSocketCallbacks {
    onLogEvent: (event: LogEvent) => void;
    onExecutionComplete: () => void;
    onStatusRunning: () => void;
    onError: (error: string) => void;
}

/**
 * Connects to the WebSocket server
 * @param url - The WebSocket URL to connect to
 * @param callbacks - Callbacks for handling websocket events
 */
export function connectWebSocket(url: string, callbacks: WebSocketCallbacks): void {
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
        const errorMsg = `Failed to create WebSocket connection. ${reason}`;
        connectionError.set(errorMsg);
        callbacks.onError(errorMsg);
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
                callbacks.onExecutionComplete();
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
                // Notify that execution is running (first log message received)
                callbacks.onStatusRunning();

                const logEvent: LogEvent = {
                    message: message.message as string,
                    timestamp: message.timestamp as number,
                    event_id: eventId,
                    line: 0 // Line number will be assigned by the view
                };

                callbacks.onLogEvent(logEvent);
            }
        } catch (error) {
            const reason = error instanceof Error ? error.message : 'Unknown error';
            const errorMsg = `Received invalid data from WebSocket server: ${reason}`;
            connectionError.set(errorMsg);
            callbacks.onError(errorMsg);
        }
    };

    socket.onerror = (): void => {
        const errorMsg = 'WebSocket connection failed.';
        connectionError.set(errorMsg);
        callbacks.onError(errorMsg);
        isConnecting.set(false);
        isConnected.set(false);
    };

    socket.onclose = (event: CloseEvent): void => {
        isConnecting.set(false);
        isConnected.set(false);
        if (event.code !== 1000) {
            // 1000 is normal closure
            const errorMsg = `Disconnected: ${event.reason || 'Connection lost'}`;
            connectionError.set(errorMsg);
            callbacks.onError(errorMsg);
        } else {
            // If websocket closed cleanly (code 1000) but we didn't receive a disconnect message,
            // and it wasn't a manual disconnect, notify execution complete
            if (!manuallyDisconnected) {
                callbacks.onExecutionComplete();
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
