/**
 * Shared log event types used across the web app.
 */

export interface BaseLogEvent {
    message: string;
    timestamp: number;
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

export function normalizeLogEvents(events: ApiLogEvent[] = []): LogEvent[] {
    return events
        .filter((event) => Boolean(event.message) && typeof event.timestamp === 'number')
        .map((event, index) => ({
            ...event,
            line: event.line ?? index + 1
        }));
}
