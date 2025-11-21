/**
 * UI state store
 */
import { writable } from 'svelte/store';

export const VIEWS = {
    LOGS: 'logs',
    RUN: 'run',
    CLAIM: 'claim',
    SETTINGS: 'settings',
    LIST: 'list'
} as const;

export type ViewName = (typeof VIEWS)[keyof typeof VIEWS];

export const activeView = writable<ViewName>('run');
