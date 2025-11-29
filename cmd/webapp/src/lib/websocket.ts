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
import { isCompleted } from '../stores/execution';
import type { LogEvent, WebSocketLogMessage } from '../types/logs';

let socket: WebSocket | null = null;
let manuallyDisconnected = false;
// Track seen event IDs to prevent duplicates
let seenEventIds: Set<string> = new Set();

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

    // Initialize seen event IDs from current log events
    const currentEvents = get(logEvents);
    seenEventIds = new Set(
        currentEvents
            .map((e) => e.event_id)
            .filter((id): id is string => typeof id === 'string' && id !== '')
    );

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

            // Handle log events (messages with a message property and timestamp)
            if (message.message && message.timestamp !== undefined) {
                logEvents.update((events: LogEvent[]): LogEvent[] => {
                    const eventId = message.event_id as string | undefined;

                    // Deduplicate using event_id if available (preferred), otherwise fall back to timestamp
                    if (eventId && eventId !== '') {
                        // Use event_id for deduplication (primary method)
                        if (seenEventIds.has(eventId)) {
                            return events;
                        }
                        seenEventIds.add(eventId);
                    } else {
                        // Fallback: check if we've seen this exact timestamp+message combination
                        // This is less reliable but necessary if event_id is missing
                        const seenByTimestamp = events.some(
                            (e) =>
                                e.timestamp === message.timestamp && e.message === message.message
                        );
                        if (seenByTimestamp) {
                            return events;
                        }
                    }

                    // Assign a new line number (like CLI does - just append to the end)
                    const nextLine =
                        events.length > 0 ? Math.max(...events.map((e) => e.line)) + 1 : 1;

                    const eventWithLine: LogEvent = {
                        message: message.message as string,
                        timestamp: message.timestamp as number,
                        event_id: eventId,
                        line: nextLine
                    };

                    // Simply append the new event (matching CLI behavior)
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
    // Clear seen event IDs when disconnecting
    seenEventIds.clear();
}
