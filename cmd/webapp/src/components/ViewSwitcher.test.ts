/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import ViewSwitcher from './ViewSwitcher.svelte';
import { VIEWS } from '../stores/ui';

// Use vi.hoisted to create variables that can be used in mocks
// eslint-disable-next-line @typescript-eslint/no-var-requires
const mocks = vi.hoisted(() => {
    // @ts-expect-error - require is available in test environment
    const { writable } = require('svelte/store');
    return {
        mockPageStore: writable({
            url: new URL('http://localhost:5173/')
        })
    };
});

// Mock the $app/stores module
vi.mock('$app/stores', () => {
    return {
        page: mocks.mockPageStore
    };
});

describe('ViewSwitcher', () => {
    const views = [
        { id: VIEWS.RUN, label: 'Run Command' },
        { id: VIEWS.LIST, label: 'Executions' },
        { id: VIEWS.CLAIM, label: 'Claim Key' },
        { id: VIEWS.LOGS, label: 'Logs' },
        { id: VIEWS.SETTINGS, label: 'Settings' }
    ];

    beforeEach(() => {
        // Reset the page store to root path before each test
        mocks.mockPageStore.set({
            url: new URL('http://localhost:5173/')
        });
    });

    it('should render all view links', () => {
        render(ViewSwitcher, { props: { views } });

        expect(screen.getByText('Run Command')).toBeInTheDocument();
        expect(screen.getByText('Executions')).toBeInTheDocument();
        expect(screen.getByText('Claim Key')).toBeInTheDocument();
        expect(screen.getByText('Logs')).toBeInTheDocument();
        expect(screen.getByText('Settings')).toBeInTheDocument();
    });

    it('should render links with correct hrefs', () => {
        render(ViewSwitcher, { props: { views } });

        const runLink = screen.getByText('Run Command');
        expect(runLink).toHaveAttribute('href', '/');

        const execLink = screen.getByText('Executions');
        expect(execLink).toHaveAttribute('href', '/executions');

        const claimLink = screen.getByText('Claim Key');
        expect(claimLink).toHaveAttribute('href', '/claim');

        const logsLink = screen.getByText('Logs');
        expect(logsLink).toHaveAttribute('href', '/logs');

        const settingsLink = screen.getByText('Settings');
        expect(settingsLink).toHaveAttribute('href', '/settings');
    });

    it('should mark root path as active when on home page', () => {
        mocks.mockPageStore.set({
            url: new URL('http://localhost:5173/')
        });

        render(ViewSwitcher, { props: { views } });

        const runLink = screen.getByText('Run Command');
        expect(runLink).toHaveClass('active');
    });

    it('should mark correct link as active based on current path', () => {
        mocks.mockPageStore.set({
            url: new URL('http://localhost:5173/logs')
        });

        render(ViewSwitcher, { props: { views } });

        const logsLink = screen.getByText('Logs');
        expect(logsLink).toHaveClass('active');

        const runLink = screen.getByText('Run Command');
        expect(runLink).not.toHaveClass('active');
    });

    it('should handle disabled views', () => {
        const viewsWithDisabled = [
            { id: VIEWS.RUN, label: 'Run Command' },
            { id: VIEWS.LOGS, label: 'Logs', disabled: true }
        ];

        render(ViewSwitcher, { props: { views: viewsWithDisabled } });

        const logsLink = screen.getByText('Logs');
        expect(logsLink).toHaveClass('disabled');
        expect(logsLink).toHaveAttribute('aria-disabled', 'true');
    });

    it('should not mark disabled links as active', () => {
        mocks.mockPageStore.set({
            url: new URL('http://localhost:5173/logs')
        });

        const viewsWithDisabled = [
            { id: VIEWS.RUN, label: 'Run Command' },
            { id: VIEWS.LOGS, label: 'Logs', disabled: true }
        ];

        render(ViewSwitcher, { props: { views: viewsWithDisabled } });

        const logsLink = screen.getByText('Logs');
        // Even though we're on /logs, the link should show disabled style
        expect(logsLink).toHaveClass('disabled');
    });

    it('should render with aria-current for active page', () => {
        mocks.mockPageStore.set({
            url: new URL('http://localhost:5173/settings')
        });

        render(ViewSwitcher, { props: { views } });

        const settingsLink = screen.getByText('Settings');
        expect(settingsLink).toHaveAttribute('aria-current', 'page');

        const runLink = screen.getByText('Run Command');
        expect(runLink).not.toHaveAttribute('aria-current');
    });

    it('should handle query parameters in URL', () => {
        mocks.mockPageStore.set({
            url: new URL('http://localhost:5173/logs?execution_id=abc123')
        });

        render(ViewSwitcher, { props: { views } });

        const logsLink = screen.getByText('Logs');
        expect(logsLink).toHaveClass('active');
    });

    it('should handle empty views array', () => {
        const { container } = render(ViewSwitcher, { props: { views: [] } });

        const nav = container.querySelector('nav');
        expect(nav).toBeInTheDocument();
        expect(nav?.children.length).toBe(0);
    });

    it('should have proper accessibility attributes', () => {
        render(ViewSwitcher, { props: { views } });

        const nav = screen.getByRole('navigation');
        expect(nav).toHaveAttribute('aria-label', 'View selection');
    });

    it('should match executions route correctly', () => {
        mocks.mockPageStore.set({
            url: new URL('http://localhost:5173/executions')
        });

        render(ViewSwitcher, { props: { views } });

        const execLink = screen.getByText('Executions');
        expect(execLink).toHaveClass('active');

        const runLink = screen.getByText('Run Command');
        expect(runLink).not.toHaveClass('active');
    });

    it('should only mark root as active for exact match', () => {
        mocks.mockPageStore.set({
            url: new URL('http://localhost:5173/logs')
        });

        render(ViewSwitcher, { props: { views } });

        const runLink = screen.getByText('Run Command');
        expect(runLink).not.toHaveClass('active');

        const logsLink = screen.getByText('Logs');
        expect(logsLink).toHaveClass('active');
    });
});
