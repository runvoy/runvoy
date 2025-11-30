/**
 * Application constants
 * Centralized location for all application-wide constants
 */

// Execution status constants (matches backend ExecutionStatus)
export const ExecutionStatus = {
    STARTING: 'STARTING',
    RUNNING: 'RUNNING',
    SUCCEEDED: 'SUCCEEDED',
    FAILED: 'FAILED',
    STOPPED: 'STOPPED',
    TERMINATING: 'TERMINATING'
} as const;

// Frontend-only status values
export const FrontendStatus = {
    LOADING: 'LOADING'
} as const;

// Terminal statuses (executions that have completed)
export const TERMINAL_STATUSES = [
    ExecutionStatus.SUCCEEDED,
    ExecutionStatus.FAILED,
    ExecutionStatus.STOPPED
] as const;

/**
 * Checks if an execution status is terminal (completed)
 */
export function isTerminalStatus(status: string): boolean {
    return (TERMINAL_STATUSES as readonly string[]).includes(status);
}

// View names
export const VIEWS = {
    LOGS: 'logs',
    RUN: 'run',
    CLAIM: 'claim',
    SETTINGS: 'settings',
    LIST: 'list'
} as const;

// View routes - maps view IDs to their URL paths
export const VIEW_ROUTES: Record<string, string> = {
    [VIEWS.RUN]: '/',
    [VIEWS.LOGS]: '/logs',
    [VIEWS.LIST]: '/executions',
    [VIEWS.CLAIM]: '/claim',
    [VIEWS.SETTINGS]: '/settings'
};
