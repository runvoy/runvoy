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
        const { container } = render(LogControls, {
            props: defaultProps
        });

        const buttons = container.querySelectorAll('.control-btn');
        expect(buttons.length).toBe(4); // pause, clear, download, metadata
    });

    it('should display log count', () => {
        render(LogControls, {
            props: {
                ...defaultProps,
                events: mockEvents
            }
        });

        expect(screen.getByText('2 lines')).toBeInTheDocument();
    });

    it('should call onToggleMetadata when metadata button is clicked', async () => {
        const onToggleMetadata = vi.fn();

        const { container } = render(LogControls, {
            props: {
                ...defaultProps,
                showMetadata: true,
                onToggleMetadata
            }
        });

        const buttons = container.querySelectorAll('.control-btn');
        const metadataButton = buttons[3]; // Last button is metadata
        await fireEvent.click(metadataButton);

        expect(onToggleMetadata).toHaveBeenCalledTimes(1);
    });

    it('should call onClear when clear button is clicked', async () => {
        const onClear = vi.fn();

        const { container } = render(LogControls, {
            props: {
                ...defaultProps,
                events: mockEvents,
                onClear
            }
        });

        const buttons = container.querySelectorAll('.control-btn');
        const clearButton = buttons[1]; // Second button is clear
        await fireEvent.click(clearButton);

        expect(onClear).toHaveBeenCalledTimes(1);
    });

    it('should call onPause when pause button is clicked', async () => {
        const onPause = vi.fn();

        const { container } = render(LogControls, {
            props: {
                ...defaultProps,
                onPause
            }
        });

        const buttons = container.querySelectorAll('.control-btn');
        const pauseButton = buttons[0]; // First button is pause/resume
        await fireEvent.click(pauseButton);

        expect(onPause).toHaveBeenCalledTimes(1);
    });

    it('should call onResume when resume button is clicked', async () => {
        const onPause = vi.fn();
        const onResume = vi.fn();

        const { container } = render(LogControls, {
            props: {
                ...defaultProps,
                onPause,
                onResume
            }
        });

        const buttons = container.querySelectorAll('.control-btn');
        const pauseButton = buttons[0];

        // First pause
        await fireEvent.click(pauseButton);

        // Then resume
        await fireEvent.click(pauseButton);

        expect(onResume).toHaveBeenCalledTimes(1);
    });

    it('should disable download button when events is empty', () => {
        const { container } = render(LogControls, {
            props: {
                ...defaultProps,
                events: []
            }
        });

        const buttons = container.querySelectorAll('.control-btn');
        const downloadButton = buttons[2] as HTMLButtonElement; // Third button is download
        expect(downloadButton).toBeDisabled();
    });

    it('should enable download button when events has logs', () => {
        const { container } = render(LogControls, {
            props: {
                ...defaultProps,
                events: mockEvents
            }
        });

        const buttons = container.querySelectorAll('.control-btn');
        const downloadButton = buttons[2] as HTMLButtonElement;
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

        const { container } = render(LogControls, {
            props: {
                ...defaultProps,
                events: mockEvents
            }
        });

        const buttons = container.querySelectorAll('.control-btn');
        const downloadButton = buttons[2];
        await fireEvent.click(downloadButton);

        expect(objectURLSpy).toHaveBeenCalled();
        expect(clickSpy).toHaveBeenCalledTimes(1);
        expect(revokeSpy).toHaveBeenCalledWith('blob://logs');
        expect(createdAnchor).not.toBeNull();
        expect(createdAnchor!.download).toContain(defaultProps.executionId);

        createElementSpy.mockRestore();
    });
});
