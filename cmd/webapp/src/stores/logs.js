import { writable } from 'svelte/store';

export const logEvents = writable([]);
export const logsRetryCount = writable(0);
export const showMetadata = writable(true);

// Constants for retry logic
export const MAX_LOGS_RETRIES = 3;
export const LOGS_RETRY_DELAY = 10000; // 10 seconds in milliseconds
export const STARTING_STATE_DELAY = 15000; // 15 seconds in milliseconds - Fargate tasks take ~15s to start
