/**
 * Execution state store
 *
 */
import { writable } from 'svelte/store';

// Tracks the current/most recent execution ID so it can be reused across navigations.
export const executionId = writable<string | null>(null);
