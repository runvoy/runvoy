import { get } from 'svelte/store';
import { executionId } from '../stores/execution.js';
import { logEvents, logsRetryCount } from '../stores/logs.js';
import { cachedWebSocketURL, websocketConnection } from '../stores/websocket.js';
import { disconnectWebSocket } from './websocket.js';

function resetExecutionData() {
    logEvents.set([]);
    logsRetryCount.set(0);
    cachedWebSocketURL.set(null);
}

function updateDocumentTitle(id) {
    if (typeof document === 'undefined') {
        return;
    }
    document.title = id ? `runvoy Logs - ${id}` : 'runvoy Logs';
}

export function switchExecution(newExecutionId, { updateHistory = true } = {}) {
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

    if (updateHistory && typeof window !== 'undefined') {
        const urlParams = new URLSearchParams(window.location.search);
        urlParams.set('execution_id', trimmedId);
        const newUrl = `${window.location.pathname}?${urlParams.toString()}`;
        window.history.pushState({ executionId: trimmedId }, '', newUrl);
    }
}

export function clearExecution({ updateHistory = true } = {}) {
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
        const newUrl = newQuery ? `${window.location.pathname}?${newQuery}` : window.location.pathname;
        window.history.pushState({}, '', newUrl);
    }
}
