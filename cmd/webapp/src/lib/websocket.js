import { websocketConnection, isConnecting, connectionError } from '../stores/websocket.js';
import { logEvents } from '../stores/logs.js';

let socket = null;
let lastSeenTimestamp = null;

/**
 * Connects to the WebSocket server and requests backlog after connection
 * @param {string} url - The WebSocket URL to connect to
 * @param {string} executionId - The execution ID to request backlog for
 */
export function connectWebSocket(url, executionId) {
    if (socket && socket.readyState === WebSocket.OPEN) {
        return;
    }

    isConnecting.set(true);
    connectionError.set(null);

    try {
        socket = new WebSocket(url);
    } catch (err) {
        // eslint-disable-next-line no-console
        console.error('WebSocket connection error:', err);
        connectionError.set('Failed to create WebSocket connection. Invalid URL?');
        isConnecting.set(false);
        return;
    }

    websocketConnection.set(socket);

    socket.onopen = () => {
        // eslint-disable-next-line no-console
        console.log('WebSocket connected');
        isConnecting.set(false);
        connectionError.set(null);

        // Request backlog after connection is established
        if (executionId) {
            requestBacklog(executionId);
        }
    };

    socket.onmessage = (event) => {
        try {
            const message = JSON.parse(event.data);

            // Handle disconnect messages
            if (message.type === 'disconnect') {
                // eslint-disable-next-line no-console
                console.log('Received disconnect message:', message.reason || 'unknown reason');
                // Close the connection gracefully
                if (socket && socket.readyState === WebSocket.OPEN) {
                    socket.close(1000, 'Execution completed');
                }
                return;
            }

            // Handle log events (messages with a message property and timestamp)
            if (message.message && message.timestamp !== undefined) {
                logEvents.update((events) => {
                    // Avoid duplicates by checking timestamp (primary key)
                    if (events.some((e) => e.timestamp === message.timestamp)) {
                        return events;
                    }

                    // Assign a new line number
                    const nextLine =
                        events.length > 0 ? Math.max(...events.map((e) => e.line)) + 1 : 1;
                    const eventWithLine = { ...message, line: nextLine };

                    // Track the latest timestamp for reconnection support
                    if (!lastSeenTimestamp || message.timestamp > lastSeenTimestamp) {
                        lastSeenTimestamp = message.timestamp;
                    }

                    return [...events, eventWithLine];
                });
            }
        } catch (err) {
            // eslint-disable-next-line no-console
            console.error('Error parsing WebSocket message:', err);
        }
    };

    socket.onerror = (error) => {
        // eslint-disable-next-line no-console
        console.error('WebSocket error:', error);
        connectionError.set('WebSocket connection failed.');
        isConnecting.set(false);
    };

    socket.onclose = (event) => {
        // eslint-disable-next-line no-console
        console.log('WebSocket disconnected:', event.reason);
        isConnecting.set(false);
        if (event.code !== 1000) {
            // 1000 is normal closure
            connectionError.set(`Disconnected: ${event.reason || 'Connection lost'}`);
        }
        websocketConnection.set(null);
    };
}

/**
 * Disconnects the WebSocket connection
 */
export function disconnectWebSocket() {
    if (socket) {
        socket.close(1000, 'User disconnected');
        socket = null;
        websocketConnection.set(null);
    }
}

/**
 * Requests backlog of logs from the server after WebSocket connection is established
 * @param {string} executionId - The execution ID to request backlog for
 */
function requestBacklog(executionId) {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
        // eslint-disable-next-line no-console
        console.warn('Cannot request backlog: WebSocket not open');
        return;
    }

    const message = {
        type: 'getBacklog',
        execution_id: executionId
        // Optionally include 'since' to fetch only logs after a certain timestamp
        // since: lastSeenTimestamp
    };

    try {
        socket.send(JSON.stringify(message));
        // eslint-disable-next-line no-console
        console.log('Requested backlog for execution:', executionId);
    } catch (err) {
        // eslint-disable-next-line no-console
        console.error('Failed to send backlog request:', err);
    }
}
