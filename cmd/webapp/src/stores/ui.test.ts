import { describe, it, expect, beforeEach } from 'vitest';
import { activeView, VIEWS } from './ui';
import { get } from 'svelte/store';
import type { ViewName } from './ui';

describe('UI Store', () => {
    beforeEach(() => {
        activeView.set(VIEWS.RUN);
    });

    describe('VIEWS constant', () => {
        it('should define all view types', () => {
            expect(VIEWS.LOGS).toBe('logs');
            expect(VIEWS.RUN).toBe('run');
            expect(VIEWS.CLAIM).toBe('claim');
            expect(VIEWS.SETTINGS).toBe('settings');
        });

        it('should have four view types', () => {
            const viewNames = Object.keys(VIEWS);
            expect(viewNames).toHaveLength(4);
        });

        it('should contain correct property names', () => {
            expect(Object.keys(VIEWS)).toContain('LOGS');
            expect(Object.keys(VIEWS)).toContain('RUN');
            expect(Object.keys(VIEWS)).toContain('CLAIM');
            expect(Object.keys(VIEWS)).toContain('SETTINGS');
        });
    });

    describe('activeView store', () => {
        it('should initialize to RUN view', () => {
            const view = get(activeView);
            expect(view).toBe(VIEWS.RUN);
        });

        it('should switch to LOGS view', () => {
            activeView.set(VIEWS.LOGS);
            expect(get(activeView)).toBe(VIEWS.LOGS);
        });

        it('should switch to CLAIM view', () => {
            activeView.set(VIEWS.CLAIM);
            expect(get(activeView)).toBe(VIEWS.CLAIM);
        });

        it('should switch to SETTINGS view', () => {
            activeView.set(VIEWS.SETTINGS);
            expect(get(activeView)).toBe(VIEWS.SETTINGS);
        });

        it('should accept string values matching view types', () => {
            const views: ViewName[] = ['logs', 'run', 'claim', 'settings'];

            views.forEach((view) => {
                activeView.set(view);
                expect(get(activeView)).toBe(view);
            });
        });

        it('should allow switching between all views', () => {
            const viewOrder = [VIEWS.RUN, VIEWS.LOGS, VIEWS.CLAIM, VIEWS.SETTINGS];

            viewOrder.forEach((view) => {
                activeView.set(view);
                expect(get(activeView)).toBe(view);
            });
        });

        it('should work with update function', () => {
            activeView.update(() => VIEWS.LOGS);
            expect(get(activeView)).toBe(VIEWS.LOGS);

            activeView.update(() => VIEWS.SETTINGS);
            expect(get(activeView)).toBe(VIEWS.SETTINGS);
        });

        it('should handle rapid view switches', () => {
            activeView.set(VIEWS.LOGS);
            activeView.set(VIEWS.CLAIM);
            activeView.set(VIEWS.SETTINGS);
            activeView.set(VIEWS.RUN);

            expect(get(activeView)).toBe(VIEWS.RUN);
        });
    });

    describe('store subscriptions', () => {
        it('should notify subscribers of view changes', () => {
            const views: ViewName[] = [];
            const unsubscribe = activeView.subscribe((view) => {
                views.push(view);
            });

            activeView.set(VIEWS.LOGS);
            activeView.set(VIEWS.CLAIM);

            expect(views).toHaveLength(3); // Initial RUN + 2 updates
            expect(views[0]).toBe(VIEWS.RUN);
            expect(views[1]).toBe(VIEWS.LOGS);
            expect(views[2]).toBe(VIEWS.CLAIM);

            unsubscribe();
        });

        it('should support multiple subscribers', () => {
            const sub1: ViewName[] = [];
            const sub2: ViewName[] = [];

            const unsub1 = activeView.subscribe((v) => sub1.push(v));
            const unsub2 = activeView.subscribe((v) => sub2.push(v));

            activeView.set(VIEWS.LOGS);

            expect(sub1).toEqual([VIEWS.RUN, VIEWS.LOGS]);
            expect(sub2).toEqual([VIEWS.RUN, VIEWS.LOGS]);

            unsub1();
            unsub2();
        });

        it('should allow unsubscribing', () => {
            const views: ViewName[] = [];
            const unsubscribe = activeView.subscribe((view) => {
                views.push(view);
            });

            activeView.set(VIEWS.LOGS);
            unsubscribe();
            activeView.set(VIEWS.CLAIM);

            expect(views).toHaveLength(2); // RUN + LOGS, not CLAIM
            expect(views[0]).toBe(VIEWS.RUN);
            expect(views[1]).toBe(VIEWS.LOGS);
        });

        it('should call subscriber immediately with current value', () => {
            activeView.set(VIEWS.LOGS);
            const views: ViewName[] = [];

            const unsubscribe = activeView.subscribe((view) => {
                views.push(view);
            });

            expect(views[0]).toBe(VIEWS.LOGS);

            unsubscribe();
        });
    });

    describe('view state management', () => {
        it('should maintain view state across updates', () => {
            activeView.set(VIEWS.LOGS);
            expect(get(activeView)).toBe(VIEWS.LOGS);

            // Update other things
            activeView.update(() => VIEWS.SETTINGS);

            expect(get(activeView)).toBe(VIEWS.SETTINGS);
        });

        it('should not change view on null operations', () => {
            activeView.set(VIEWS.LOGS);
            activeView.set(VIEWS.LOGS); // Set to same value

            expect(get(activeView)).toBe(VIEWS.LOGS);
        });

        it('should handle view switching in component lifecycle', () => {
            // Simulate component mount
            activeView.set(VIEWS.RUN);
            expect(get(activeView)).toBe(VIEWS.RUN);

            // Simulate user navigation
            activeView.set(VIEWS.LOGS);
            expect(get(activeView)).toBe(VIEWS.LOGS);

            // Simulate component cleanup
            activeView.set(VIEWS.RUN);
            expect(get(activeView)).toBe(VIEWS.RUN);
        });
    });

    describe('type safety', () => {
        it('should use correct view type', () => {
            const view: ViewName = VIEWS.LOGS;
            activeView.set(view);
            expect(get(activeView)).toBe(view);
        });

        it('all VIEWS values should be valid ViewName', () => {
            const views = Object.values(VIEWS) as ViewName[];

            views.forEach((view) => {
                activeView.set(view);
                expect(get(activeView)).toBe(view);
            });
        });
    });
});
