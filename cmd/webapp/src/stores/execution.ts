/**
 * Execution state store
 */
import { writable } from 'svelte/store';

export const executionId = writable<string | null>(null);
export const executionStatus = writable<string | null>(null);
export const isCompleted = writable<boolean>(false);
export const startedAt = writable<string | null>(null);
export const completedAt = writable<string | null>(null);
export const exitCode = writable<number | null>(null);
