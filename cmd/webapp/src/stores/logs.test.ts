import { describe, it, expect, beforeEach } from 'vitest';
import { logEvents, showMetadata } from './logs';
import { get } from 'svelte/store';
import type { LogEvent } from '../types/logs';

describe('Logs Store', () => {
    beforeEach(() => {
        // Reset stores to initial state
        logEvents.set([]);
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
                event_id: 'event-1',
                line: 0
            };

            logEvents.set([event]);
            expect(get(logEvents)).toHaveLength(1);
            expect(get(logEvents)[0]).toEqual(event);
        });

        it('should handle multiple log events', () => {
            const events: LogEvent[] = [
                { message: 'Line 1', timestamp: 1234567890, event_id: 'event-1', line: 0 },
                { message: 'Line 2', timestamp: 1234567891, event_id: 'event-2', line: 1 },
                { message: 'Line 3', timestamp: 1234567892, event_id: 'event-3', line: 2 }
            ];

            logEvents.set(events);
            expect(get(logEvents)).toHaveLength(3);
            expect(get(logEvents)).toEqual(events);
        });

        it('should update events with update function', () => {
            const initialEvent: LogEvent = {
                message: 'Initial',
                timestamp: 1234567890,
                event_id: 'event-1',
                line: 0
            };

            logEvents.set([initialEvent]);

            logEvents.update((events) => [
                ...events,
                { message: 'Added', timestamp: 1234567891, event_id: 'event-2', line: 1 }
            ]);

            expect(get(logEvents)).toHaveLength(2);
        });

        it('should clear events', () => {
            logEvents.set([
                { message: 'Event', timestamp: 1234567890, event_id: 'event-1', line: 0 }
            ]);

            logEvents.set([]);
            expect(get(logEvents)).toHaveLength(0);
        });

        it('should preserve log order', () => {
            const events: LogEvent[] = Array.from({ length: 5 }, (_, i) => ({
                message: `Line ${i}`,
                timestamp: 1234567890 + i,
                event_id: `event-${i}`,
                line: i
            }));

            logEvents.set(events);
            const stored = get(logEvents);

            for (let i = 0; i < stored.length; i++) {
                expect(stored[i].line).toBe(i);
            }
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

    describe('store subscriptions', () => {
        it('should notify subscribers of log event changes', () => {
            const received: LogEvent[][] = [];
            const unsubscribe = logEvents.subscribe((events) => {
                received.push([...events]);
            });

            const event1: LogEvent = {
                message: 'Event 1',
                timestamp: 1234567890,
                event_id: 'event-1',
                line: 0
            };
            const event2: LogEvent = {
                message: 'Event 2',
                timestamp: 1234567891,
                event_id: 'event-2',
                line: 1
            };

            logEvents.set([event1]);
            logEvents.set([event1, event2]);

            expect(received).toHaveLength(3); // Initial empty array + 2 updates
            expect(received[1]).toEqual([event1]);
            expect(received[2]).toEqual([event1, event2]);

            unsubscribe();
        });
    });

    describe('combined log state', () => {
        it('should track complete logging lifecycle', () => {
            // Start with empty logs
            expect(get(logEvents)).toEqual([]);

            // Add first batch of logs
            const batch1: LogEvent[] = [
                { message: 'Line 1', timestamp: 1000, event_id: 'event-1', line: 0 },
                { message: 'Line 2', timestamp: 1001, event_id: 'event-2', line: 1 }
            ];
            logEvents.set(batch1);

            expect(get(logEvents)).toHaveLength(2);

            // Append more logs
            logEvents.update((events) => [
                ...events,
                { message: 'Line 3', timestamp: 1002, event_id: 'event-3', line: 2 }
            ]);

            expect(get(logEvents)).toHaveLength(3);

            // Clear on new execution
            logEvents.set([]);

            expect(get(logEvents)).toEqual([]);
        });
    });
});
