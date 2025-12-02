/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/svelte';
import LogControls from './LogControls.svelte';
import { showMetadata, logEvents } from '../stores/logs';
import { disconnectWebSocket, connectWebSocket } from '../lib/websocket';
import { cachedWebSocketURL } from '../stores/websocket';
import { get } from 'svelte/store';

// Mock the websocket functions
vi.mock('../lib/websocket', () => ({
    disconnectWebSocket: vi.fn(),
    connectWebSocket: vi.fn()
}));

describe('LogControls', () => {
    beforeEach(() => {
        showMetadata.set(true);
        logEvents.set([]);
        cachedWebSocketURL.set(null);
    });

    afterEach(() => {
        cleanup();
        vi.clearAllMocks();
        showMetadata.set(true);
        logEvents.set([]);
        cachedWebSocketURL.set(null);
    });

    it('should render all control buttons', () => {
        render(LogControls, {
            props: {
                executionId: 'exec-123'
            }
        });

        expect(screen.getByText('⏸️ Pause')).toBeInTheDocument();
        expect(screen.getByText(/🗑️ Clear/)).toBeInTheDocument();
        expect(screen.getByText('📥 Download')).toBeInTheDocument();
        expect(screen.getByText(/🙈 Hide/)).toBeInTheDocument();
    });

    it('should toggle metadata visibility when metadata button is clicked', async () => {
        showMetadata.set(true);
        render(LogControls, {
            props: {
                executionId: 'exec-123'
            }
        });

        const metadataButton = screen.getByText(/🙈 Hide/);
        await fireEvent.click(metadataButton);

        expect(get(showMetadata)).toBe(false);
        expect(screen.getByText(/🙉 Show/)).toBeInTheDocument();
    });

    it('should show "Show Metadata" when metadata is hidden', async () => {
        showMetadata.set(false);
        render(LogControls, {
            props: {
                executionId: 'exec-123'
            }
        });

        expect(screen.getByText(/🙉 Show/)).toBeInTheDocument();
    });

    it('should clear logs when clear button is clicked', async () => {
        logEvents.set([
            {
                message: 'Test log 1',
                timestamp: Date.now(),
                event_id: 'event-1',
                line: 1
            },
            {
                message: 'Test log 2',
                timestamp: Date.now(),
                event_id: 'event-2',
                line: 2
            }
        ]);

        render(LogControls, {
            props: {
                executionId: 'exec-123'
            }
        });

        const clearButton = screen.getByText(/🗑️ Clear/);
        await fireEvent.click(clearButton);

        expect(get(logEvents)).toEqual([]);
    });

    it('should pause websocket when pause button is clicked', async () => {
        render(LogControls, {
            props: {
                executionId: 'exec-123'
            }
        });

        const pauseButton = screen.getByText('⏸️ Pause');
        await fireEvent.click(pauseButton);

        expect(disconnectWebSocket).toHaveBeenCalledTimes(1);
        expect(screen.getByText('▶️ Resume')).toBeInTheDocument();
    });

    it('should resume websocket when resume button is clicked', async () => {
        cachedWebSocketURL.set('wss://example.com/logs');

        render(LogControls, {
            props: {
                executionId: 'exec-123'
            }
        });

        // First pause
        const pauseButton = screen.getByText('⏸️ Pause');
        await fireEvent.click(pauseButton);

        // Then resume
        const resumeButton = screen.getByText('▶️ Resume');
        await fireEvent.click(resumeButton);

        expect(connectWebSocket).toHaveBeenCalledWith('wss://example.com/logs');
    });

    it('should not resume if cachedWebSocketURL is not set', async () => {
        cachedWebSocketURL.set(null);

        render(LogControls, {
            props: {
                executionId: 'exec-123'
            }
        });

        // Pause first
        const pauseButton = screen.getByText('⏸️ Pause');
        await fireEvent.click(pauseButton);

        // Try to resume
        const resumeButton = screen.getByText('▶️ Resume');
        await fireEvent.click(resumeButton);

        // Should not call connectWebSocket if URL is not cached
        expect(connectWebSocket).not.toHaveBeenCalled();
    });
});
