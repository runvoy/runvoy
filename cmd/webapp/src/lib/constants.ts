/**
 * Application constants
 * Centralized location for all application-wide constants
 */

// Logs retry configuration
export const MAX_LOGS_RETRIES = 3;
export const LOGS_RETRY_DELAY = 10000; // 10 seconds in milliseconds
export const STARTING_STATE_DELAY = 30000; // 30 seconds in milliseconds

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
