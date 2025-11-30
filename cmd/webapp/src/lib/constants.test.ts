/// <reference types="vitest" />

import { describe, it, expect } from 'vitest';
import { ExecutionStatus, FrontendStatus, TERMINAL_STATUSES, isTerminalStatus } from './constants';

describe('constants', () => {
    describe('ExecutionStatus', () => {
        it('should have all expected status values', () => {
            expect(ExecutionStatus.STARTING).toBe('STARTING');
            expect(ExecutionStatus.RUNNING).toBe('RUNNING');
            expect(ExecutionStatus.SUCCEEDED).toBe('SUCCEEDED');
            expect(ExecutionStatus.FAILED).toBe('FAILED');
            expect(ExecutionStatus.STOPPED).toBe('STOPPED');
            expect(ExecutionStatus.TERMINATING).toBe('TERMINATING');
        });
    });

    describe('FrontendStatus', () => {
        it('should have LOADING status', () => {
            expect(FrontendStatus.LOADING).toBe('LOADING');
        });
    });

    describe('TERMINAL_STATUSES', () => {
        it('should include all terminal statuses', () => {
            expect(TERMINAL_STATUSES).toContain(ExecutionStatus.SUCCEEDED);
            expect(TERMINAL_STATUSES).toContain(ExecutionStatus.FAILED);
            expect(TERMINAL_STATUSES).toContain(ExecutionStatus.STOPPED);
        });

        it('should not include non-terminal statuses', () => {
            expect(TERMINAL_STATUSES).not.toContain(ExecutionStatus.STARTING);
            expect(TERMINAL_STATUSES).not.toContain(ExecutionStatus.RUNNING);
            expect(TERMINAL_STATUSES).not.toContain(ExecutionStatus.TERMINATING);
        });
    });

    describe('isTerminalStatus', () => {
        it('should return true for terminal statuses', () => {
            expect(isTerminalStatus(ExecutionStatus.SUCCEEDED)).toBe(true);
            expect(isTerminalStatus(ExecutionStatus.FAILED)).toBe(true);
            expect(isTerminalStatus(ExecutionStatus.STOPPED)).toBe(true);
        });

        it('should return false for non-terminal statuses', () => {
            expect(isTerminalStatus(ExecutionStatus.STARTING)).toBe(false);
            expect(isTerminalStatus(ExecutionStatus.RUNNING)).toBe(false);
            expect(isTerminalStatus(ExecutionStatus.TERMINATING)).toBe(false);
        });

        it('should return false for frontend-only statuses', () => {
            expect(isTerminalStatus(FrontendStatus.LOADING)).toBe(false);
        });

        it('should return false for unknown statuses', () => {
            expect(isTerminalStatus('UNKNOWN_STATUS')).toBe(false);
            expect(isTerminalStatus('')).toBe(false);
        });
    });
});
