/**
 * Execution state management utilities
 *
 * Used primarily by RunView when starting a new execution.
 * LogsView uses URL as source of truth and doesn't need these functions.
 */
import { get } from 'svelte/store';
import { executionId } from '../stores/execution';
import { logEvents } from '../stores/logs';
import { cachedWebSocketURL, websocketConnection } from '../stores/websocket';
import { disconnectWebSocket } from './websocket';
import { activeView, VIEWS } from '../stores/ui';

function resetExecutionData(): void {
    logEvents.set([]);
    cachedWebSocketURL.set(null);
}

function updateDocumentTitle(id: string | null): void {
    if (typeof document === 'undefined') {
        return;
    }
    document.title = id ? `runvoy Logs - ${id}` : 'runvoy Logs';
}

/**
 * Switch to a new execution ID.
 * Used by RunView when starting a new command - it sets up the execution state
 * before navigating to the logs page with goto().
 */
export function switchExecution(newExecutionId: string): void {
    const trimmedId = (newExecutionId || '').trim();
    if (!trimmedId) {
        clearExecution();
        return;
    }

    const currentId = get(executionId);
    if (currentId === trimmedId) {
        return;
    }

    const activeSocket = get(websocketConnection);
    if (activeSocket) {
        activeSocket.close();
        websocketConnection.set(null);
    }
    disconnectWebSocket();

    executionId.set(trimmedId);
    resetExecutionData();
    updateDocumentTitle(trimmedId);
    activeView.set(VIEWS.LOGS);
}

export function clearExecution(): void {
    const activeSocket = get(websocketConnection);
    if (activeSocket) {
        activeSocket.close();
        websocketConnection.set(null);
    }
    disconnectWebSocket();

    executionId.set(null);
    resetExecutionData();
    updateDocumentTitle('');
}
