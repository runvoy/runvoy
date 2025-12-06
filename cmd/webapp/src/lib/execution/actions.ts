/**
 * Execution action factories for kill and other lifecycle operations.
 * These are designed to be composed with other modules (e.g., LogsManager)
 * and reused across different views.
 */

import { writable, type Readable } from 'svelte/store';
import type APIClient from '../api';
import type { ApiError } from '../../types/api';
import type { KillState } from './types';

export interface ExecutionKillResult {
	/** Reactive state for the kill operation */
	state: Readable<KillState>;
	/** Attempt to kill the execution. Returns true on success, false on failure. */
	kill: (executionId: string) => Promise<boolean>;
	/** Reset state (e.g., when switching executions) */
	reset: () => void;
}

/**
 * Creates a kill operation handler for executions.
 * Returns reactive state and a kill function.
 *
 * @example
 * ```svelte
 * <script>
 *   const killer = createExecutionKiller(apiClient);
 *   const { state, kill } = killer;
 * </script>
 *
 * <button
 *   onclick={() => kill(executionId)}
 *   disabled={$state.isKilling || $state.killInitiated}
 * >
 *   {$state.isKilling ? 'Killing...' : 'Kill'}
 * </button>
 * ```
 */
export function createExecutionKiller(apiClient: APIClient): ExecutionKillResult {
	const state = writable<KillState>({
		isKilling: false,
		killInitiated: false,
		error: null
	});

	async function kill(executionId: string): Promise<boolean> {
		if (!executionId) return false;

		state.update((s) => ({ ...s, isKilling: true, error: null }));

		try {
			await apiClient.killExecution(executionId);
			state.update((s) => ({ ...s, isKilling: false, killInitiated: true }));
			return true;
		} catch (err) {
			const error = err as ApiError;
			const message = error.details?.error || error.message || 'Failed to stop execution';
			state.update((s) => ({ ...s, isKilling: false, error: message }));
			return false;
		}
	}

	function reset(): void {
		state.set({
			isKilling: false,
			killInitiated: false,
			error: null
		});
	}

	return {
		state: { subscribe: state.subscribe },
		kill,
		reset
	};
}

