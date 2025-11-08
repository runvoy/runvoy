import { writable } from 'svelte/store';

export const VIEWS = {
    LOGS: 'logs',
    RUN: 'run'
};

export const activeView = writable(VIEWS.LOGS);
