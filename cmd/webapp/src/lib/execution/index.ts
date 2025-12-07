/**
 * Execution actions module
 *
 * Provides factories for execution lifecycle operations (kill, etc.)
 * that can be composed with other modules and reused across views.
 */

export { createExecutionKiller, type ExecutionKillResult } from './actions';
export type { KillState } from './types';
