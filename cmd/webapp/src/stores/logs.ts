/**
 * Logs state store
 */
import { writable } from 'svelte/store';
import type { LogEvent } from '../types/stores';

export const logEvents = writable<LogEvent[]>([]);
export const logsRetryCount = writable<number>(0);
export const showMetadata = writable<boolean>(true);

// Constants for retry logic
export const MAX_LOGS_RETRIES = 3;
export const LOGS_RETRY_DELAY = 10000; // 10 seconds in milliseconds
export const STARTING_STATE_DELAY = 30000; // 30 seconds in milliseconds
