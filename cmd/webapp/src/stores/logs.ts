/**
 * Logs state store
 */
import { writable } from 'svelte/store';
import type { LogEvent } from '../types/logs';

export const logEvents = writable<LogEvent[]>([]);
export const logsRetryCount = writable<number>(0);
export const showMetadata = writable<boolean>(true);

// Re-export constants for convenience
export { MAX_LOGS_RETRIES, LOGS_RETRY_DELAY, STARTING_STATE_DELAY } from '$lib/constants';
