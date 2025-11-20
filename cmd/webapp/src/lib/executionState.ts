/**
 * Execution state management utilities
 */
import { get } from 'svelte/store';
import { executionId } from '../stores/execution';
import { logEvents, logsRetryCount } from '../stores/logs';
import { cachedWebSocketURL, websocketConnection } from '../stores/websocket';
import { disconnectWebSocket } from './websocket';
import { activeView, VIEWS } from '../stores/ui';

function resetExecutionData(): void {
    logEvents.set([]);
    logsRetryCount.set(0);
    cachedWebSocketURL.set(null);
}

function updateDocumentTitle(id: string | null): void {
    if (typeof document === 'undefined') {
        return;
    }
    document.title = id ? `runvoy Logs - ${id}` : 'runvoy Logs';
}

interface SwitchExecutionOptions {
    updateHistory?: boolean;
}

export function switchExecution(
    newExecutionId: string,
    { updateHistory = true }: SwitchExecutionOptions = {}
): void {
    const trimmedId = (newExecutionId || '').trim();
    if (!trimmedId) {
        clearExecution({ updateHistory });
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

    if (updateHistory && typeof window !== 'undefined') {
        const urlParams = new URLSearchParams(window.location.search);
        urlParams.set('execution_id', trimmedId);
        const newUrl = `${window.location.pathname}?${urlParams.toString()}`;
        window.history.pushState({ executionId: trimmedId }, '', newUrl);
    }
}

interface ClearExecutionOptions {
    updateHistory?: boolean;
}

export function clearExecution({ updateHistory = true }: ClearExecutionOptions = {}): void {
    const activeSocket = get(websocketConnection);
    if (activeSocket) {
        activeSocket.close();
        websocketConnection.set(null);
    }
    disconnectWebSocket();

    executionId.set(null);
    resetExecutionData();
    updateDocumentTitle('');

    if (updateHistory && typeof window !== 'undefined') {
        const urlParams = new URLSearchParams(window.location.search);
        urlParams.delete('execution_id');
        const newQuery = urlParams.toString();
        const newUrl = newQuery
            ? `${window.location.pathname}?${newQuery}`
            : window.location.pathname;
        window.history.pushState({}, '', newUrl);
    }
}
