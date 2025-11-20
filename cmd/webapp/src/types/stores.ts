/**
 * Type definitions for Svelte stores
 */

export interface Config {
    endpoint: string | null;
    apiKey: string | null;
}

export interface ExecutionState {
    id: string | null;
    status: string | null;
    isCompleted: boolean;
    startedAt: string | null;
}

export interface UIState {
    activeView: 'run' | 'logs';
}

export interface WebSocketState {
    connection: WebSocket | null;
    isConnecting: boolean;
    connectionError: string | null;
}

export interface LogsState {
    events: LogEvent[];
}

export interface LogEvent {
    message: string;
    timestamp: number;
    line: number;
    level?: 'info' | 'warn' | 'error' | 'debug';
}

export interface EnvRow {
    id: number;
    key: string;
    value: string;
}

export interface ViewNames {
    RUN: 'run';
    LOGS: 'logs';
}
