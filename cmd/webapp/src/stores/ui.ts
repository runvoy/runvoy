/**
 * UI state store
 */
import { writable } from 'svelte/store';
import { VIEWS } from '$lib/constants';
import type { ViewName } from '../types/status';

// Re-export for convenience
export { VIEWS, type ViewName };

export const activeView = writable<ViewName>('run');
