import { describe, it, expect, beforeEach } from 'vitest';
import {
    logEvents,
    logsRetryCount,
    showMetadata,
    MAX_LOGS_RETRIES,
    LOGS_RETRY_DELAY,
    STARTING_STATE_DELAY
} from './logs';
import { get } from 'svelte/store';
import type { LogEvent } from '../types/logs';

describe('Logs Store', () => {
    beforeEach(() => {
        // Reset stores to initial state
        logEvents.set([]);
        logsRetryCount.set(0);
        showMetadata.set(true);
    });

    describe('logEvents', () => {
        it('should initialize as empty array', () => {
            expect(get(logEvents)).toEqual([]);
        });

        it('should add log events', () => {
            const event: LogEvent = {
                message: 'Starting execution',
                timestamp: 1234567890,
                line: 0
            };

            logEvents.set([event]);
            expect(get(logEvents)).toHaveLength(1);
            expect(get(logEvents)[0]).toEqual(event);
        });

        it('should handle multiple log events', () => {
            const events: LogEvent[] = [
                { message: 'Line 1', timestamp: 1234567890, line: 0 },
                { message: 'Line 2', timestamp: 1234567891, line: 1 },
                { message: 'Line 3', timestamp: 1234567892, line: 2 }
            ];

            logEvents.set(events);
            expect(get(logEvents)).toHaveLength(3);
            expect(get(logEvents)).toEqual(events);
        });

        it('should update events with update function', () => {
            const initialEvent: LogEvent = {
                message: 'Initial',
                timestamp: 1234567890,
                line: 0
            };

            logEvents.set([initialEvent]);

            logEvents.update((events) => [
                ...events,
                { message: 'Added', timestamp: 1234567891, line: 1 }
            ]);

            expect(get(logEvents)).toHaveLength(2);
        });

        it('should clear events', () => {
            logEvents.set([{ message: 'Event', timestamp: 1234567890, line: 0 }]);

            logEvents.set([]);
            expect(get(logEvents)).toHaveLength(0);
        });

        it('should preserve log order', () => {
            const events: LogEvent[] = Array.from({ length: 5 }, (_, i) => ({
                message: `Line ${i}`,
                timestamp: 1234567890 + i,
                line: i
            }));

            logEvents.set(events);
            const stored = get(logEvents);

            for (let i = 0; i < stored.length; i++) {
                expect(stored[i].line).toBe(i);
            }
        });
    });

    describe('logsRetryCount', () => {
        it('should initialize to 0', () => {
            expect(get(logsRetryCount)).toBe(0);
        });

        it('should increment retry count', () => {
            logsRetryCount.set(1);
            expect(get(logsRetryCount)).toBe(1);

            logsRetryCount.update((count) => count + 1);
            expect(get(logsRetryCount)).toBe(2);
        });

        it('should not exceed max retries', () => {
            logsRetryCount.set(MAX_LOGS_RETRIES);
            expect(get(logsRetryCount)).toBe(MAX_LOGS_RETRIES);
        });

        it('should reset retry count', () => {
            logsRetryCount.set(5);
            logsRetryCount.set(0);
            expect(get(logsRetryCount)).toBe(0);
        });
    });

    describe('showMetadata', () => {
        it('should initialize to true', () => {
            expect(get(showMetadata)).toBe(true);
        });

        it('should toggle metadata display', () => {
            expect(get(showMetadata)).toBe(true);

            showMetadata.set(false);
            expect(get(showMetadata)).toBe(false);

            showMetadata.set(true);
            expect(get(showMetadata)).toBe(true);
        });

        it('should work with update function', () => {
            showMetadata.update((show) => !show);
            expect(get(showMetadata)).toBe(false);
        });
    });

    describe('retry constants', () => {
        it('should define MAX_LOGS_RETRIES', () => {
            expect(MAX_LOGS_RETRIES).toBe(3);
        });

        it('should define LOGS_RETRY_DELAY in milliseconds', () => {
            expect(LOGS_RETRY_DELAY).toBe(10000);
        });

        it('should define STARTING_STATE_DELAY in milliseconds', () => {
            expect(STARTING_STATE_DELAY).toBe(30000);
        });

        it('STARTING_STATE_DELAY should be longer than LOGS_RETRY_DELAY', () => {
            expect(STARTING_STATE_DELAY).toBeGreaterThan(LOGS_RETRY_DELAY);
        });
    });

    describe('store subscriptions', () => {
        it('should notify subscribers of log event changes', () => {
            const received: LogEvent[][] = [];
            const unsubscribe = logEvents.subscribe((events) => {
                received.push([...events]);
            });

            const event1: LogEvent = { message: 'Event 1', timestamp: 1234567890, line: 0 };
            const event2: LogEvent = { message: 'Event 2', timestamp: 1234567891, line: 1 };

            logEvents.set([event1]);
            logEvents.set([event1, event2]);

            expect(received).toHaveLength(3); // Initial empty array + 2 updates
            expect(received[1]).toEqual([event1]);
            expect(received[2]).toEqual([event1, event2]);

            unsubscribe();
        });

        it('should notify subscribers of retry count changes', () => {
            const counts: number[] = [];
            const unsubscribe = logsRetryCount.subscribe((count) => {
                counts.push(count);
            });

            logsRetryCount.set(1);
            logsRetryCount.set(2);
            logsRetryCount.set(3);

            expect(counts).toEqual([0, 1, 2, 3]);

            unsubscribe();
        });
    });

    describe('combined log state', () => {
        it('should track complete logging lifecycle', () => {
            // Start with empty logs
            expect(get(logEvents)).toEqual([]);
            expect(get(logsRetryCount)).toBe(0);

            // Add first batch of logs
            const batch1: LogEvent[] = [
                { message: 'Line 1', timestamp: 1000, line: 0 },
                { message: 'Line 2', timestamp: 1001, line: 1 }
            ];
            logEvents.set(batch1);

            expect(get(logEvents)).toHaveLength(2);
            expect(get(logsRetryCount)).toBe(0);

            // Append more logs
            logEvents.update((events) => [
                ...events,
                { message: 'Line 3', timestamp: 1002, line: 2 }
            ]);

            expect(get(logEvents)).toHaveLength(3);

            // Increment retry
            logsRetryCount.update((count) => count + 1);
            expect(get(logsRetryCount)).toBe(1);

            // Clear on new execution
            logEvents.set([]);
            logsRetryCount.set(0);

            expect(get(logEvents)).toEqual([]);
            expect(get(logsRetryCount)).toBe(0);
        });
    });
});
