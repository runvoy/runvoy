/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { render, screen } from '@testing-library/svelte';
import { describe, it, expect, beforeEach } from 'vitest';
import LogLine, { clearCaches } from './LogLine.svelte';
import type { LogEvent } from '../types/logs';
import { formatTimestamp } from '../lib/ansi';

describe('LogLine', () => {
    beforeEach(() => {
        clearCaches();
    });

    const baseEvent: LogEvent = {
        event_id: 'event-1',
        message: 'Hello, world',
        timestamp: 1_704_000_000_000,
        line: 42
    };

    it('renders metadata and message content by default', () => {
        render(LogLine, {
            props: {
                event: baseEvent,
                showMetadata: true
            }
        });

        expect(screen.getByText(baseEvent.line.toString())).toBeInTheDocument();
        expect(screen.getByText(formatTimestamp(baseEvent.timestamp))).toBeInTheDocument();
        expect(screen.getByText('Hello, world')).toBeInTheDocument();
    });

    it('hides metadata when showMetadata is false', () => {
        render(LogLine, {
            props: {
                event: baseEvent,
                showMetadata: false
            }
        });

        expect(screen.queryByText(baseEvent.line.toString())).not.toBeInTheDocument();
        expect(screen.queryByText(formatTimestamp(baseEvent.timestamp))).not.toBeInTheDocument();
        expect(screen.getByText('Hello, world')).toBeInTheDocument();
    });

    it('applies ANSI classes to colored log segments', () => {
        const colorfulEvent: LogEvent = {
            ...baseEvent,
            event_id: 'event-colorful',
            message: `Start \u001b[31mError\u001b[0m end`
        };

        render(LogLine, {
            props: {
                event: colorfulEvent,
                showMetadata: true
            }
        });

        expect(screen.getByText('Error')).toHaveClass('ansi-red');
        const messageContainer = document.querySelector('.message');
        expect(messageContainer?.textContent?.replace(/\s+/g, ' ').trim()).toBe('Start Error end');
    });
});
