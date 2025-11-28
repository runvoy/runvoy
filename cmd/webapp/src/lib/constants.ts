/**
 * Application constants
 * Centralized location for all application-wide constants
 */

// Cookie configuration
export const COOKIE_MAX_AGE = 60 * 60 * 24 * 30; // 30 days in seconds

// Logs retry configuration
export const MAX_LOGS_RETRIES = 3;
export const LOGS_RETRY_DELAY = 10000; // 10 seconds in milliseconds
export const STARTING_STATE_DELAY = 30000; // 30 seconds in milliseconds

// View names
export const VIEWS = {
    LOGS: 'logs',
    RUN: 'run',
    CLAIM: 'claim',
    SETTINGS: 'settings',
    LIST: 'list'
} as const;

export type ViewName = (typeof VIEWS)[keyof typeof VIEWS];
