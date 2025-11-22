/// <reference types="vitest" />

import { describe, it, expect } from 'vitest';
import { VIEWS } from '../stores/ui';

describe('Routing Configuration', () => {
    const viewRoutes: Record<string, string> = {
        run: '/',
        logs: '/logs',
        list: '/executions',
        claim: '/claim',
        settings: '/settings'
    };

    describe('View to Route Mapping', () => {
        it('should map RUN view to root path', () => {
            expect(viewRoutes[VIEWS.RUN]).toBe('/');
        });

        it('should map LOGS view to /logs path', () => {
            expect(viewRoutes[VIEWS.LOGS]).toBe('/logs');
        });

        it('should map LIST view to /executions path', () => {
            expect(viewRoutes[VIEWS.LIST]).toBe('/executions');
        });

        it('should map CLAIM view to /claim path', () => {
            expect(viewRoutes[VIEWS.CLAIM]).toBe('/claim');
        });

        it('should map SETTINGS view to /settings path', () => {
            expect(viewRoutes[VIEWS.SETTINGS]).toBe('/settings');
        });

        it('should have routes for all views', () => {
            const allViews = Object.values(VIEWS);
            const routedViews = Object.keys(viewRoutes);

            allViews.forEach((view) => {
                expect(routedViews).toContain(view);
            });
        });

        it('should not have duplicate routes', () => {
            const routes = Object.values(viewRoutes);
            const uniqueRoutes = [...new Set(routes)];

            expect(routes.length).toBe(uniqueRoutes.length);
        });

        it('should have valid URL paths', () => {
            Object.values(viewRoutes).forEach((route) => {
                expect(route).toMatch(/^\/[a-z]*$/);
            });
        });
    });

    describe('Route Structure', () => {
        it('should have root route for default view', () => {
            expect(viewRoutes[VIEWS.RUN]).toBe('/');
        });

        it('should have all routes start with slash', () => {
            Object.values(viewRoutes).forEach((route) => {
                expect(route.startsWith('/')).toBe(true);
            });
        });

        it('should not have trailing slashes except for root', () => {
            Object.values(viewRoutes).forEach((route) => {
                if (route !== '/') {
                    expect(route.endsWith('/')).toBe(false);
                }
            });
        });
    });

    describe('View ID Consistency', () => {
        it('should have matching view IDs in routes', () => {
            const viewIds = Object.values(VIEWS);

            viewIds.forEach((viewId) => {
                expect(viewRoutes).toHaveProperty(viewId);
            });
        });

        it('should only contain lowercase routes', () => {
            Object.values(viewRoutes).forEach((route) => {
                expect(route).toBe(route.toLowerCase());
            });
        });
    });
});
