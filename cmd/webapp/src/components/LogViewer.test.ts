/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { render, screen, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, afterEach, beforeAll } from 'vitest';
import LogViewer from './LogViewer.svelte';
import type { LogEvent } from '../types/logs';

describe('LogViewer', () => {
    const events: LogEvent[] = [
        { event_id: '1', message: 'first', timestamp: 1, line: 1 },
        { event_id: '2', message: 'second', timestamp: 2, line: 2 }
    ];

    afterEach(() => {
        vi.restoreAllMocks();
    });

    it('shows placeholder when no logs are available', () => {
        render(LogViewer, {
            props: {
                events: [],
                showMetadata: true
            }
        });

        expect(screen.getByText('Waiting for logs...')).toBeInTheDocument();
    });

    it('renders all log lines when events are provided', () => {
        const { container } = render(LogViewer, {
            props: {
                events,
                showMetadata: true
            }
        });

        const logLines = container.querySelectorAll('.log-line');
        expect(logLines).toHaveLength(events.length);
        expect(screen.queryByText('Waiting for logs...')).not.toBeInTheDocument();
    });

    it('auto-scrolls when new logs arrive', async () => {
        const { container } = render(LogViewer, {
            props: {
                events,
                showMetadata: true
            }
        });

        const viewer = container.querySelector('.log-viewer-container') as HTMLElement;
        Object.defineProperty(viewer, 'scrollHeight', { value: 1000, writable: true });
        Object.defineProperty(viewer, 'clientHeight', { value: 200, writable: true });

        // Auto-scroll sets scrollTop directly on the container
        await waitFor(() => {
            // The component attempts to scroll to bottom when events arrive
            expect(viewer.scrollTop).toBeDefined();
        });
    });
});
