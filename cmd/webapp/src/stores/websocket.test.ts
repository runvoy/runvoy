import { describe, it, expect, beforeEach } from 'vitest';
import {
    websocketConnection,
    cachedWebSocketURL,
    isConnecting,
    connectionError
} from './websocket';
import { get } from 'svelte/store';

describe('WebSocket Store', () => {
    beforeEach(() => {
        // Reset stores to initial state
        websocketConnection.set(null);
        cachedWebSocketURL.set(null);
        isConnecting.set(false);
        connectionError.set(null);
    });

    describe('websocketConnection', () => {
        it('should initialize to null', () => {
            expect(get(websocketConnection)).toBeNull();
        });

        it('should store WebSocket instance', () => {
            const mockWs = {
                readyState: 1,
                close: () => {},
                send: () => {}
            } as any;

            websocketConnection.set(mockWs);
            expect(get(websocketConnection)).toBe(mockWs);
        });

        it('should clear WebSocket connection', () => {
            const mockWs = {
                readyState: 1,
                close: () => {},
                send: () => {}
            } as any;

            websocketConnection.set(mockWs);
            websocketConnection.set(null);
            expect(get(websocketConnection)).toBeNull();
        });

        it('should support update function', () => {
            const mockWs = {
                readyState: 1,
                close: () => {},
                send: () => {}
            } as any;

            websocketConnection.update(() => mockWs);
            expect(get(websocketConnection)).toBe(mockWs);

            websocketConnection.update(() => null);
            expect(get(websocketConnection)).toBeNull();
        });
    });

    describe('cachedWebSocketURL', () => {
        it('should initialize to null', () => {
            expect(get(cachedWebSocketURL)).toBeNull();
        });

        it('should store WebSocket URL', () => {
            const url = 'wss://localhost:8080/api/v1/executions/exec-123/logs';
            cachedWebSocketURL.set(url);
            expect(get(cachedWebSocketURL)).toBe(url);
        });

        it('should accept different URL formats', () => {
            const urls = [
                'wss://example.com/logs',
                'ws://localhost:3000/logs',
                'wss://api.example.com/v1/executions/exec-id/logs'
            ];

            urls.forEach((url) => {
                cachedWebSocketURL.set(url);
                expect(get(cachedWebSocketURL)).toBe(url);
            });
        });

        it('should clear cached URL', () => {
            cachedWebSocketURL.set('wss://example.com/logs');
            cachedWebSocketURL.set(null);
            expect(get(cachedWebSocketURL)).toBeNull();
        });
    });

    describe('isConnecting', () => {
        it('should initialize to false', () => {
            expect(get(isConnecting)).toBe(false);
        });

        it('should track connecting state', () => {
            isConnecting.set(true);
            expect(get(isConnecting)).toBe(true);

            isConnecting.set(false);
            expect(get(isConnecting)).toBe(false);
        });

        it('should toggle connecting state', () => {
            isConnecting.update((state) => !state);
            expect(get(isConnecting)).toBe(true);

            isConnecting.update((state) => !state);
            expect(get(isConnecting)).toBe(false);
        });
    });

    describe('connectionError', () => {
        it('should initialize to null', () => {
            expect(get(connectionError)).toBeNull();
        });

        it('should store error message', () => {
            const error = 'WebSocket connection failed';
            connectionError.set(error);
            expect(get(connectionError)).toBe(error);
        });

        it('should store various error messages', () => {
            const errors = [
                'Connection refused',
                'Network timeout',
                'Invalid URL',
                'Server closed connection'
            ];

            errors.forEach((error) => {
                connectionError.set(error);
                expect(get(connectionError)).toBe(error);
            });
        });

        it('should clear error', () => {
            connectionError.set('Some error');
            connectionError.set(null);
            expect(get(connectionError)).toBeNull();
        });

        it('should work with update function', () => {
            connectionError.update(() => 'Error');
            expect(get(connectionError)).toBe('Error');

            connectionError.update(() => null);
            expect(get(connectionError)).toBeNull();
        });
    });

    describe('store subscriptions', () => {
        it('should notify subscribers of connection changes', () => {
            const states: (WebSocket | null)[] = [];
            const unsubscribe = websocketConnection.subscribe((ws) => {
                states.push(ws);
            });

            const mockWs = { readyState: 1 } as any;
            websocketConnection.set(mockWs);
            websocketConnection.set(null);

            expect(states).toHaveLength(3); // Initial null + 2 updates
            expect(states[1]).toBe(mockWs);
            expect(states[2]).toBeNull();

            unsubscribe();
        });

        it('should notify subscribers of URL changes', () => {
            const urls: (string | null)[] = [];
            const unsubscribe = cachedWebSocketURL.subscribe((url) => {
                urls.push(url);
            });

            const url1 = 'wss://example.com/1';
            const url2 = 'wss://example.com/2';

            cachedWebSocketURL.set(url1);
            cachedWebSocketURL.set(url2);
            cachedWebSocketURL.set(null);

            expect(urls).toHaveLength(4); // Initial null + 3 updates
            expect(urls[1]).toBe(url1);
            expect(urls[2]).toBe(url2);
            expect(urls[3]).toBeNull();

            unsubscribe();
        });

        it('should notify subscribers of error changes', () => {
            const errors: (string | null)[] = [];
            const unsubscribe = connectionError.subscribe((error) => {
                errors.push(error);
            });

            connectionError.set('Error 1');
            connectionError.set('Error 2');
            connectionError.set(null);

            expect(errors).toHaveLength(4); // Initial null + 3 updates
            expect(errors[1]).toBe('Error 1');
            expect(errors[2]).toBe('Error 2');
            expect(errors[3]).toBeNull();

            unsubscribe();
        });
    });

    describe('combined WebSocket lifecycle', () => {
        it('should track complete connection lifecycle', () => {
            // Initial state
            expect(get(websocketConnection)).toBeNull();
            expect(get(cachedWebSocketURL)).toBeNull();
            expect(get(isConnecting)).toBe(false);
            expect(get(connectionError)).toBeNull();

            // Start connecting
            isConnecting.set(true);
            cachedWebSocketURL.set('wss://example.com/logs');

            expect(get(isConnecting)).toBe(true);
            expect(get(cachedWebSocketURL)).toBe('wss://example.com/logs');

            // Connection established
            const mockWs = { readyState: 1 } as any;
            websocketConnection.set(mockWs);
            isConnecting.set(false);

            expect(get(websocketConnection)).toBe(mockWs);
            expect(get(isConnecting)).toBe(false);
            expect(get(connectionError)).toBeNull();

            // Connection error
            websocketConnection.set(null);
            connectionError.set('Connection lost');

            expect(get(websocketConnection)).toBeNull();
            expect(get(connectionError)).toBe('Connection lost');
            expect(get(isConnecting)).toBe(false);

            // Clear error for retry
            connectionError.set(null);
            isConnecting.set(true);

            expect(get(connectionError)).toBeNull();
            expect(get(isConnecting)).toBe(true);
        });
    });
});
