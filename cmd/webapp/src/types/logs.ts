/**
 * Shared log event types used across the web app.
 */

export interface BaseLogEvent {
    message: string;
    timestamp: number;
    event_id: string;
    level?: 'info' | 'warn' | 'error' | 'debug';
}

export interface ApiLogEvent extends BaseLogEvent {
    line?: number;
}

export interface LogEvent extends BaseLogEvent {
    line: number;
}

export interface WebSocketLogMessage extends Partial<ApiLogEvent> {
    type?: string;
    reason?: string;
    [key: string]: unknown;
}
