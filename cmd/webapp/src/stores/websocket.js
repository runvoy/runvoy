import { writable } from 'svelte/store';

export const websocketConnection = writable(null);
export const cachedWebSocketURL = writable(null);
export const isConnecting = writable(false);
export const connectionError = writable(null);
