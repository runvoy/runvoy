/// <reference types="vitest" />

import { describe, it, expect } from 'vitest';
import {
    ExecutionStatus,
    FrontendStatus,
    TERMINAL_STATUSES,
    isTerminalStatus,
    isKillableStatus
} from './constants';

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
            // Type assertion needed for invalid status values in tests
            expect(isTerminalStatus('UNKNOWN_STATUS' as any)).toBe(false);
            expect(isTerminalStatus('' as any)).toBe(false);
        });
    });

    describe('isKillableStatus', () => {
        it('should return true for killable statuses', () => {
            expect(isKillableStatus(ExecutionStatus.STARTING)).toBe(true);
            expect(isKillableStatus(ExecutionStatus.RUNNING)).toBe(true);
        });

        it('should return false for terminal statuses', () => {
            expect(isKillableStatus(ExecutionStatus.SUCCEEDED)).toBe(false);
            expect(isKillableStatus(ExecutionStatus.FAILED)).toBe(false);
            expect(isKillableStatus(ExecutionStatus.STOPPED)).toBe(false);
        });

        it('should return false for TERMINATING status', () => {
            expect(isKillableStatus(ExecutionStatus.TERMINATING)).toBe(false);
        });

        it('should return false for null or empty status', () => {
            expect(isKillableStatus(null)).toBe(false);
            // Type assertion needed for invalid status value in test
            expect(isKillableStatus('' as any)).toBe(false);
        });

        it('should return false for frontend-only LOADING status', () => {
            expect(isKillableStatus(FrontendStatus.LOADING)).toBe(false);
        });
    });
});
