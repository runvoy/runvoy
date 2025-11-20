/**
 * UI state store
 */
import { writable } from 'svelte/store';

export const VIEWS = {
    LOGS: 'logs',
    RUN: 'run'
} as const;

export type ViewName = (typeof VIEWS)[keyof typeof VIEWS];

export const activeView = writable<ViewName>('logs');
