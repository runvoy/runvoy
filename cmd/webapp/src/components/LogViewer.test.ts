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

    beforeAll(() => {
        Object.defineProperty(window, 'scrollTo', { value: vi.fn(), writable: true });
    });

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
        const scrollSpy = vi.spyOn(window, 'scrollTo').mockImplementation(() => {});
        const rafSpy = vi
            .spyOn(window, 'requestAnimationFrame')
            .mockImplementation((cb: FrameRequestCallback): number => {
                cb(0);
                return 1;
            });

        render(LogViewer, {
            props: {
                events,
                showMetadata: true
            }
        });

        await waitFor(() => expect(rafSpy).toHaveBeenCalled());
        expect(scrollSpy).toHaveBeenCalledWith({
            top: document.documentElement.scrollHeight,
            behavior: 'auto'
        });
    });
});
