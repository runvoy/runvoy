import { describe, it, expect, beforeEach } from 'vitest';
import { executionId, executionStatus, isCompleted, startedAt } from './execution';
import { get } from 'svelte/store';

describe('Execution Store', () => {
    beforeEach(() => {
        // Reset stores to initial state
        executionId.set(null);
        executionStatus.set(null);
        isCompleted.set(false);
        startedAt.set(null);
    });

    describe('executionId', () => {
        it('should initialize to null', () => {
            expect(get(executionId)).toBeNull();
        });

        it('should update when set is called', () => {
            const id = 'exec-123';
            executionId.set(id);
            expect(get(executionId)).toBe(id);
        });

        it('should support update function', () => {
            executionId.set('exec-initial');
            executionId.update((current) => (current ? `${current}-updated` : 'new'));
            expect(get(executionId)).toBe('exec-initial-updated');
        });
    });

    describe('executionStatus', () => {
        it('should initialize to null', () => {
            expect(get(executionStatus)).toBeNull();
        });

        it('should accept status values', () => {
            const statuses = ['PENDING', 'RUNNING', 'SUCCEEDED', 'FAILED', 'KILLED'];

            statuses.forEach((status) => {
                executionStatus.set(status);
                expect(get(executionStatus)).toBe(status);
            });
        });

        it('should update status', () => {
            executionStatus.set('RUNNING');
            executionStatus.set('SUCCEEDED');
            expect(get(executionStatus)).toBe('SUCCEEDED');
        });
    });

    describe('isCompleted', () => {
        it('should initialize to false', () => {
            expect(get(isCompleted)).toBe(false);
        });

        it('should toggle between true and false', () => {
            expect(get(isCompleted)).toBe(false);
            isCompleted.set(true);
            expect(get(isCompleted)).toBe(true);
            isCompleted.set(false);
            expect(get(isCompleted)).toBe(false);
        });

        it('should work with update function', () => {
            isCompleted.update((current) => !current);
            expect(get(isCompleted)).toBe(true);
        });
    });

    describe('startedAt', () => {
        it('should initialize to null', () => {
            expect(get(startedAt)).toBeNull();
        });

        it('should store timestamp strings', () => {
            const timestamp = '2025-01-01T00:00:00Z';
            startedAt.set(timestamp);
            expect(get(startedAt)).toBe(timestamp);
        });

        it('should support clearing', () => {
            startedAt.set('2025-01-01T00:00:00Z');
            startedAt.set(null);
            expect(get(startedAt)).toBeNull();
        });
    });

    describe('store subscriptions', () => {
        it('should notify subscribers of changes', () => {
            let value: string | null = null;
            const unsubscribe = executionId.subscribe((v) => {
                value = v;
            });

            expect(value).toBeNull();
            executionId.set('exec-123');
            expect(value).toBe('exec-123');

            unsubscribe();
        });

        it('should support multiple subscribers', () => {
            const values1: (string | null)[] = [];
            const values2: (string | null)[] = [];

            const unsub1 = executionId.subscribe((v) => values1.push(v));
            const unsub2 = executionId.subscribe((v) => values2.push(v));

            executionId.set('exec-1');
            executionId.set('exec-2');

            expect(values1).toEqual([null, 'exec-1', 'exec-2']);
            expect(values2).toEqual([null, 'exec-1', 'exec-2']);

            unsub1();
            unsub2();
        });
    });

    describe('combined store state', () => {
        it('should track execution lifecycle', () => {
            // Start execution
            executionId.set('exec-123');
            executionStatus.set('PENDING');

            expect(get(executionId)).toBe('exec-123');
            expect(get(executionStatus)).toBe('PENDING');
            expect(get(isCompleted)).toBe(false);

            // Execution starts
            executionStatus.set('RUNNING');
            startedAt.set('2025-01-01T00:00:00Z');

            expect(get(executionStatus)).toBe('RUNNING');
            expect(get(startedAt)).not.toBeNull();

            // Execution completes
            executionStatus.set('SUCCEEDED');
            isCompleted.set(true);

            expect(get(executionStatus)).toBe('SUCCEEDED');
            expect(get(isCompleted)).toBe(true);
        });
    });
});
