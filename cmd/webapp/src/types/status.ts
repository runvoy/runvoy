/**
 * Status-related types derived from constants
 */
import type {
    ExecutionStatus,
    FrontendStatus,
    VIEWS
} from '../lib/constants';

/**
 * Union type of all possible execution status values
 * Includes both backend execution statuses and frontend-only statuses
 */
export type ExecutionStatusValue =
    | (typeof ExecutionStatus)[keyof typeof ExecutionStatus]
    | (typeof FrontendStatus)[keyof typeof FrontendStatus];

/**
 * Union type of all valid view names
 */
export type ViewName = (typeof VIEWS)[keyof typeof VIEWS];
