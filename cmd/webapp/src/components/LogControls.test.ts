/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/svelte';
import LogControls from './LogControls.svelte';
import type { LogEvent } from '../types/logs';

describe('LogControls', () => {
    afterEach(() => {
        cleanup();
        vi.clearAllMocks();
        vi.restoreAllMocks();
    });

    const mockEvents: LogEvent[] = [
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
    ];

    const defaultProps = {
        executionId: 'exec-123',
        events: [] as LogEvent[],
        showMetadata: true,
        onToggleMetadata: vi.fn(),
        onClear: vi.fn(),
        onPause: vi.fn(),
        onResume: vi.fn()
    };

    it('should render all control buttons', () => {
        render(LogControls, {
            props: defaultProps
        });

        expect(screen.getByText('⏸️ Pause')).toBeInTheDocument();
        expect(screen.getByText(/🗑️ Clear/)).toBeInTheDocument();
        expect(screen.getByText('📥 Download')).toBeInTheDocument();
        expect(screen.getByText(/🙈 Hide/)).toBeInTheDocument();
    });

    it('should call onToggleMetadata when metadata button is clicked', async () => {
        const onToggleMetadata = vi.fn();

        render(LogControls, {
            props: {
                ...defaultProps,
                showMetadata: true,
                onToggleMetadata
            }
        });

        const metadataButton = screen.getByText(/🙈 Hide/);
        await fireEvent.click(metadataButton);

        expect(onToggleMetadata).toHaveBeenCalledTimes(1);
    });

    it('should show "Show Metadata" when metadata is hidden', () => {
        render(LogControls, {
            props: {
                ...defaultProps,
                showMetadata: false
            }
        });

        expect(screen.getByText(/🙉 Show/)).toBeInTheDocument();
    });

    it('should call onClear when clear button is clicked', async () => {
        const onClear = vi.fn();

        render(LogControls, {
            props: {
                ...defaultProps,
                events: mockEvents,
                onClear
            }
        });

        const clearButton = screen.getByText(/🗑️ Clear/);
        await fireEvent.click(clearButton);

        expect(onClear).toHaveBeenCalledTimes(1);
    });

    it('should call onPause when pause button is clicked', async () => {
        const onPause = vi.fn();

        render(LogControls, {
            props: {
                ...defaultProps,
                onPause
            }
        });

        const pauseButton = screen.getByText('⏸️ Pause');
        await fireEvent.click(pauseButton);

        expect(onPause).toHaveBeenCalledTimes(1);
        expect(screen.getByText('▶️ Resume')).toBeInTheDocument();
    });

    it('should call onResume when resume button is clicked', async () => {
        const onPause = vi.fn();
        const onResume = vi.fn();

        render(LogControls, {
            props: {
                ...defaultProps,
                onPause,
                onResume
            }
        });

        // First pause
        const pauseButton = screen.getByText('⏸️ Pause');
        await fireEvent.click(pauseButton);

        // Then resume
        const resumeButton = screen.getByText('▶️ Resume');
        await fireEvent.click(resumeButton);

        expect(onResume).toHaveBeenCalledTimes(1);
    });

    it('should disable download button when events is empty', () => {
        render(LogControls, {
            props: {
                ...defaultProps,
                events: []
            }
        });

        const downloadButton = screen.getByText('📥 Download');
        expect(downloadButton).toBeDisabled();
    });

    it('should enable download button when events has logs', () => {
        render(LogControls, {
            props: {
                ...defaultProps,
                events: mockEvents
            }
        });

        const downloadButton = screen.getByText('📥 Download');
        expect(downloadButton).not.toBeDisabled();
    });

    it('should generate a downloadable log file with execution metadata', async () => {
        const clickSpy = vi.fn();
        const objectURLSpy = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob://logs');
        const revokeSpy = vi.spyOn(URL, 'revokeObjectURL');
        const originalCreateElement = document.createElement.bind(document);
        let createdAnchor: HTMLAnchorElement | null = null;
        const createElementSpy = vi
            .spyOn(document, 'createElement')
            .mockImplementation((tagName: string): any => {
                if (tagName === 'a') {
                    createdAnchor = originalCreateElement('a');
                    createdAnchor.click = clickSpy;
                    return createdAnchor;
                }
                return originalCreateElement(tagName);
            });

        render(LogControls, {
            props: {
                ...defaultProps,
                events: mockEvents
            }
        });

        const downloadButton = screen.getByText('📥 Download');
        await fireEvent.click(downloadButton);

        expect(objectURLSpy).toHaveBeenCalled();
        expect(clickSpy).toHaveBeenCalledTimes(1);
        expect(revokeSpy).toHaveBeenCalledWith('blob://logs');
        expect(createdAnchor?.download).toContain(defaultProps.executionId);

        createElementSpy.mockRestore();
    });
});
