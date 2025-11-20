/**
 * WebSocket connection state store
 */
import { writable } from 'svelte/store';

export const websocketConnection = writable<WebSocket | null>(null);
export const cachedWebSocketURL = writable<string | null>(null);
export const isConnecting = writable<boolean>(false);
export const connectionError = writable<string | null>(null);
