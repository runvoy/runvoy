/**
 * Logs state store
 */
import { writable } from 'svelte/store';
import type { LogEvent } from '../types/logs';

export const logEvents = writable<LogEvent[]>([]);
export const showMetadata = writable<boolean>(true);
