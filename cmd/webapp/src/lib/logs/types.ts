/**
 * Types for the log viewing module
 */

import type { LogEvent } from '../../types/logs';
import type { ExecutionStatusValue } from '../../types/status';

export type ConnectionStatus = 'disconnected' | 'connecting' | 'connected';

export type ExecutionPhase =
    | 'idle' // No execution selected
    | 'loading' // Fetching initial data
    | 'streaming' // WebSocket active, receiving logs
    | 'completed'; // Terminal state reached

/**
 * Execution context displayed alongside logs.
 * This is read-only display data, not a full execution model.
 */
export interface ExecutionMetadata {
    executionId: string;
    status: ExecutionStatusValue | null;
    startedAt: string | null;
    completedAt: string | null;
    exitCode: number | null;
    command: string;
    imageId: string;
}

export interface LogsState {
    phase: ExecutionPhase;
    events: LogEvent[];
    metadata: ExecutionMetadata | null;
    connection: ConnectionStatus;
    error: string | null;
}
