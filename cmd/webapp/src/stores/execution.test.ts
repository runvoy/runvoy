import { describe, it, expect, beforeEach } from 'vitest';
import { executionId } from './execution';
import { get } from 'svelte/store';

describe('Execution Store', () => {
    beforeEach(() => {
        // Reset store to initial state
        executionId.set(null);
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
});
