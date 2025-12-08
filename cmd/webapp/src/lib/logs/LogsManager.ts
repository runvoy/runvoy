/**
 * LogsManager - Core orchestrator for log viewing
 *
 * Manages log events, execution context, and WebSocket streaming.
 * Exposes reactive stores for UI binding.
 *
 * For actions that modify execution state (kill, etc.), see lib/execution.
 */

import { writable, derived, type Readable, type Writable } from 'svelte/store';
import type { ConnectionStatus, ExecutionMetadata, ExecutionPhase } from './types';
import type { LogEvent } from '../../types/logs';
import type APIClient from '../api';
import type { ApiError, ExecutionStatusResponse, LogsResponse } from '../../types/api';

export interface LogsManagerConfig {
    apiClient: APIClient;
}

export interface LogsManagerStores {
    // Core state
    phase: Readable<ExecutionPhase>;
    events: Readable<LogEvent[]>;
    metadata: Readable<ExecutionMetadata | null>;
    connection: Readable<ConnectionStatus>;
    error: Readable<string | null>;

    // Derived convenience
    isLoading: Readable<boolean>;
    isStreaming: Readable<boolean>;
    isCompleted: Readable<boolean>;
}

export class LogsManager {
    private apiClient: APIClient;
    private ws: WebSocket | null = null;
    private currentExecutionId: string | null = null;
    private receivedDisconnectMessage = false;

    // Internal writable stores
    private _phase: Writable<ExecutionPhase>;
    private _events: Writable<LogEvent[]>;
    private _metadata: Writable<ExecutionMetadata | null>;
    private _connection: Writable<ConnectionStatus>;
    private _error: Writable<string | null>;

    // Public read-only stores
    public readonly stores: LogsManagerStores;

    constructor(config: LogsManagerConfig) {
        this.apiClient = config.apiClient;

        // Initialize internal state
        this._phase = writable<ExecutionPhase>('idle');
        this._events = writable<LogEvent[]>([]);
        this._metadata = writable<ExecutionMetadata | null>(null);
        this._connection = writable<ConnectionStatus>('disconnected');
        this._error = writable<string | null>(null);

        // Expose public read-only stores
        this.stores = {
            phase: { subscribe: this._phase.subscribe },
            events: { subscribe: this._events.subscribe },
            metadata: { subscribe: this._metadata.subscribe },
            connection: { subscribe: this._connection.subscribe },
            error: { subscribe: this._error.subscribe },

            isLoading: derived(this._phase, (p) => p === 'loading'),
            isStreaming: derived(this._phase, (p) => p === 'streaming'),
            isCompleted: derived(this._phase, (p) => p === 'completed')
        };
    }

    /**
     * Load logs for an execution. Handles both terminal (historical)
     * and running (WebSocket streaming) executions.
     */
    async loadExecution(executionId: string): Promise<void> {
        // Ignore if already loading this execution
        if (this.currentExecutionId === executionId) return;

        // Clean up previous execution
        this.reset();
        this.currentExecutionId = executionId;

        this._phase.set('loading');
        this._metadata.set(null);

        try {
            const [status, response] = await Promise.all([
                this.apiClient.getExecutionStatus(executionId),
                this.apiClient.getLogs(executionId)
            ]);

            // Verify we're still interested in this execution
            if (this.currentExecutionId !== executionId) return;

            this.setMetadataFromStatus(executionId, status);

            if (response.websocket_url) {
                // Running execution → stream via WebSocket
                this._metadata.update((m) =>
                    m ? { ...m, status: response.status || status.status } : m
                );
                this.connectWebSocket(response.websocket_url);
            } else {
                // Terminal execution → display historical logs
                await this.handleTerminalExecution(executionId, response, status);
            }
        } catch (err) {
            if (this.currentExecutionId !== executionId) return;
            this.setError(err);
        }
    }

    /**
     * Update the execution status (used when kill is initiated externally)
     */
    setStatus(status: string): void {
        this._metadata.update((m) => (m ? { ...m, status } : m));
    }

    /**
     * Pause log streaming (disconnect WebSocket)
     */
    pause(): void {
        this.disconnectWebSocket();
    }

    /**
     * Resume log streaming (reconnect to cached URL or re-fetch)
     */
    async resume(): Promise<void> {
        if (!this.currentExecutionId) return;

        // Re-fetch to get fresh WebSocket URL
        try {
            const response = await this.apiClient.getLogs(this.currentExecutionId);
            if (response.websocket_url) {
                this.connectWebSocket(response.websocket_url);
            }
        } catch (err) {
            this.setError(err);
        }
    }

    /**
     * Clear all logs (UI only, doesn't affect backend)
     */
    clearLogs(): void {
        this._events.set([]);
    }

    /**
     * Reset all state and disconnect
     */
    reset(): void {
        this.disconnectWebSocket();
        this.currentExecutionId = null;
        this._phase.set('idle');
        this._events.set([]);
        this._metadata.set(null);
        this._error.set(null);
    }

    /**
     * Clean up resources (call on component unmount)
     */
    destroy(): void {
        this.reset();
    }

    /**
     * Get the current execution ID
     */
    getExecutionId(): string | null {
        return this.currentExecutionId;
    }

    // --- Private methods ---

    private connectWebSocket(url: string): void {
        this.disconnectWebSocket();
        this._connection.set('connecting');
        this.receivedDisconnectMessage = false;

        try {
            this.ws = new WebSocket(url);
        } catch (err) {
            this._connection.set('disconnected');
            this.setError(err);
            return;
        }

        this.ws.onopen = (): void => {
            this._connection.set('connected');
            this._phase.set('streaming');
        };

        this.ws.onmessage = (event: MessageEvent): void => {
            this.handleWebSocketMessage(event.data);
        };

        this.ws.onerror = (): void => {
            this._error.set('WebSocket connection failed');
            this._connection.set('disconnected');
        };

        this.ws.onclose = (event: CloseEvent): void => {
            this._connection.set('disconnected');

            if (event.code === 1000) {
                // Clean close - if we didn't receive a disconnect message, execution completed
                if (!this.receivedDisconnectMessage) {
                    this.handleExecutionComplete();
                }
            } else {
                // Non-clean close - report error
                this._error.set(`Disconnected: ${event.reason || 'Connection lost'}`);
            }
        };
    }

    private disconnectWebSocket(): void {
        if (this.ws) {
            // Remove onclose to prevent it from triggering handleExecutionComplete
            this.ws.onclose = null;
            this.ws.close(1000, 'User disconnected');
            this.ws = null;
        }
        this._connection.set('disconnected');
    }

    private handleWebSocketMessage(data: string): void {
        try {
            const message = JSON.parse(data);

            if (message.type === 'disconnect') {
                this.receivedDisconnectMessage = true;
                this.handleExecutionComplete();
                // Close the connection gracefully
                if (this.ws && this.ws.readyState === WebSocket.OPEN) {
                    this.ws.close(1000, 'Execution completed');
                }
                return;
            }

            // Validate log event - event_id is required for Svelte key tracking
            const eventId = message.event_id;
            if (
                message.message &&
                message.timestamp !== undefined &&
                eventId &&
                typeof eventId === 'string'
            ) {
                this._events.update((events) => {
                    const newEvent: LogEvent = {
                        message: message.message as string,
                        timestamp: message.timestamp as number,
                        event_id: eventId,
                        line: events.length + 1
                    };
                    return [...events, newEvent];
                });

                // Update status to RUNNING on first log
                this._metadata.update((m) => {
                    if (m && (m.status === 'STARTING' || m.status === null)) {
                        return { ...m, status: 'RUNNING' };
                    }
                    return m;
                });
            }
        } catch {
            this._error.set('Received invalid data from server');
        }
    }

    private async handleExecutionComplete(): Promise<void> {
        if (!this.currentExecutionId) return;

        try {
            const status = await this.apiClient.getExecutionStatus(this.currentExecutionId);

            this._metadata.update((m) =>
                m
                    ? {
                          ...m,
                          status: status.status,
                          startedAt: status.started_at ?? m.startedAt,
                          completedAt: status.completed_at ?? null,
                          exitCode: status.exit_code ?? null,
                          command: status.command,
                          imageId: status.image_id
                      }
                    : m
            );

            this._phase.set('completed');
        } catch (err) {
            this.setError(err);
        }
    }

    private async handleTerminalExecution(
        executionId: string,
        response: LogsResponse,
        status: ExecutionStatusResponse
    ): Promise<void> {
        if (!response.status) {
            this._error.set('Invalid API response: missing execution status');
            return;
        }

        // Set events with line numbers
        const events = (response.events ?? []).map((e, i) => ({
            ...e,
            line: i + 1
        }));
        this._events.set(events);

        this.setMetadataFromStatus(executionId, status, events);

        this._phase.set('completed');
    }

    private setMetadataFromStatus(
        executionId: string,
        status: ExecutionStatusResponse,
        events?: LogEvent[]
    ): void {
        this._metadata.set({
            executionId,
            status: status.status,
            startedAt: status.started_at ?? (events ? this.deriveStartedAt(events) : null),
            completedAt: status.completed_at ?? null,
            exitCode: status.exit_code ?? null,
            command: status.command,
            imageId: status.image_id
        });
    }

    private deriveStartedAt(events: LogEvent[]): string | null {
        if (events.length === 0) return null;
        const timestamps = events.map((e) => e.timestamp).filter((t) => typeof t === 'number');
        if (timestamps.length === 0) return null;
        return new Date(Math.min(...timestamps)).toISOString();
    }

    private setError(err: unknown): void {
        const error = err as ApiError;
        const message = error.details?.error || error.message || 'An error occurred';
        this._error.set(message);
        this._phase.set('idle');
    }
}
