import { websocketConnection, isConnecting, connectionError } from '../stores/websocket.js';
import { logEvents } from '../stores/logs.js';

let socket = null;

/**
 * Connects to the WebSocket server
 * @param {string} url - The WebSocket URL to connect to
 */
export function connectWebSocket(url) {
    if (socket && socket.readyState === WebSocket.OPEN) {
        return;
    }

    isConnecting.set(true);
    connectionError.set(null);

    try {
        socket = new WebSocket(url);
    } catch (err) {
        console.error('WebSocket connection error:', err);
        connectionError.set('Failed to create WebSocket connection. Invalid URL?');
        isConnecting.set(false);
        return;
    }

    websocketConnection.set(socket);

    socket.onopen = () => {
        console.log('WebSocket connected');
        isConnecting.set(false);
        connectionError.set(null);
    };

    socket.onmessage = (event) => {
        try {
            const message = JSON.parse(event.data);
            
            // Handle disconnect messages
            if (message.type === 'disconnect') {
                console.log('Received disconnect message:', message.reason || 'unknown reason');
                // Close the connection gracefully
                if (socket && socket.readyState === WebSocket.OPEN) {
                    socket.close(1000, 'Execution completed');
                }
                return;
            }
            
            // Handle log events (messages with a message property and timestamp)
            if (message.message && message.timestamp !== undefined) {
                logEvents.update(events => {
                    // Avoid duplicates by checking timestamp (primary key)
                    if (events.some(e => e.timestamp === message.timestamp)) {
                        return events;
                    }

                    // Assign a new line number
                    const nextLine = events.length > 0 ? Math.max(...events.map(e => e.line)) + 1 : 1;
                    const eventWithLine = { ...message, line: nextLine };

                    return [...events, eventWithLine];
                });
            }
        } catch (err) {
            console.error('Error parsing WebSocket message:', err);
        }
    };

    socket.onerror = (error) => {
        console.error('WebSocket error:', error);
        connectionError.set('WebSocket connection failed.');
        isConnecting.set(false);
    };

    socket.onclose = (event) => {
        console.log('WebSocket disconnected:', event.reason);
        isConnecting.set(false);
        if (event.code !== 1000) { // 1000 is normal closure
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
