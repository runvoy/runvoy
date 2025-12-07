/**
 * Log viewing module
 *
 * Provides LogsManager for viewing logs of an execution, including:
 * - Log event streaming (WebSocket) and historical retrieval (API)
 * - Execution context (status, timestamps, exit code) for display alongside logs
 *
 * For actions that modify execution state (kill, restart, etc.), see lib/execution.
 */

export { LogsManager, type LogsManagerConfig, type LogsManagerStores } from './LogsManager';
export type { ConnectionStatus, ExecutionMetadata, ExecutionPhase, LogsState } from './types';
