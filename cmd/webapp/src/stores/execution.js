import { writable } from 'svelte/store';

export const executionId = writable(null);
export const executionStatus = writable(null);
export const isCompleted = writable(false);
export const startedAt = writable(null);
