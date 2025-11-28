/**
 * UI state store
 */
import { writable } from 'svelte/store';
import { VIEWS, type ViewName } from '$lib/constants';

// Re-export for convenience
export { VIEWS, type ViewName };

export const activeView = writable<ViewName>('run');
