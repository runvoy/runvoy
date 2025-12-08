/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import StatusBar from './StatusBar.svelte';
import { ExecutionStatus, FrontendStatus } from '../lib/constants';

describe('StatusBar', () => {
    afterEach(() => {
        vi.clearAllMocks();
    });

    const defaultProps = {
        status: null,
        startedAt: null,
        completedAt: null,
        exitCode: null,
        killInitiated: false,
        onKill: null,
        command: 'echo default',
        imageId: 'image-default'
    };

    it('should display status badge with LOADING when status is null', () => {
        render(StatusBar, {
            props: defaultProps
        });

        expect(screen.getByText(FrontendStatus.LOADING)).toBeInTheDocument();
        expect(screen.getByText('Status:')).toBeInTheDocument();
    });

    it('should display execution status', () => {
        render(StatusBar, {
            props: {
                ...defaultProps,
                status: ExecutionStatus.RUNNING
            }
        });

        expect(screen.getByText(ExecutionStatus.RUNNING)).toBeInTheDocument();
    });

    it('should display started time', () => {
        const startTime = '2025-11-30T10:00:00Z';

        render(StatusBar, {
            props: {
                ...defaultProps,
                startedAt: startTime
            }
        });

        expect(screen.getByText('Started:')).toBeInTheDocument();
        // The date will be formatted, so we check for parts of it
        const startedText = screen.getByText(/Started:/).parentElement?.textContent;
        expect(startedText).toContain('Started:');
    });

    it('should display N/A when started time is missing', () => {
        render(StatusBar, {
            props: defaultProps
        });

        expect(screen.getByText('N/A')).toBeInTheDocument();
    });

    it('should display command and image ID when provided', () => {
        render(StatusBar, {
            props: {
                ...defaultProps,
                status: ExecutionStatus.RUNNING,
                command: 'echo hello',
                imageId: 'alpine:latest-abc123'
            }
        });

        expect(screen.getByText('Command:')).toBeInTheDocument();
        expect(screen.getByText('echo hello')).toBeInTheDocument();
        expect(screen.getByText('Image ID:')).toBeInTheDocument();
        expect(screen.getByText('alpine:latest-abc123')).toBeInTheDocument();
    });

    it('should display ended time when completedAt is set', () => {
        const endTime = '2025-11-30T10:05:00Z';

        render(StatusBar, {
            props: {
                ...defaultProps,
                completedAt: endTime
            }
        });

        expect(screen.getByText('Ended:')).toBeInTheDocument();
    });

    it('should not display ended time when completedAt is null', () => {
        render(StatusBar, {
            props: defaultProps
        });

        expect(screen.queryByText('Ended:')).not.toBeInTheDocument();
    });

    it('should display exit code when exitCode is set', () => {
        render(StatusBar, {
            props: {
                ...defaultProps,
                exitCode: 0
            }
        });

        expect(screen.getByText('Exit Code:')).toBeInTheDocument();
        expect(screen.getByText('0')).toBeInTheDocument();
    });

    it('should display non-zero exit code', () => {
        render(StatusBar, {
            props: {
                ...defaultProps,
                exitCode: 1
            }
        });

        expect(screen.getByText('1')).toBeInTheDocument();
    });

    it('should not display exit code when exitCode is null', () => {
        render(StatusBar, {
            props: defaultProps
        });

        expect(screen.queryByText('Exit Code:')).not.toBeInTheDocument();
    });

    it('should display kill button when onKill is provided and status is killable', () => {
        const mockKill = vi.fn();

        render(StatusBar, {
            props: {
                ...defaultProps,
                status: ExecutionStatus.RUNNING,
                onKill: mockKill
            }
        });

        const killButton = screen.getByText('⏹️ Kill');
        expect(killButton).toBeInTheDocument();
    });

    it('should not display kill button when status is terminal', () => {
        const mockKill = vi.fn();

        render(StatusBar, {
            props: {
                ...defaultProps,
                status: ExecutionStatus.SUCCEEDED,
                onKill: mockKill
            }
        });

        expect(screen.queryByText('⏹️ Kill')).not.toBeInTheDocument();
    });

    it('should not display kill button when status is TERMINATING', () => {
        const mockKill = vi.fn();

        render(StatusBar, {
            props: {
                ...defaultProps,
                status: ExecutionStatus.TERMINATING,
                onKill: mockKill
            }
        });

        expect(screen.queryByText('⏹️ Kill')).not.toBeInTheDocument();
    });

    it('should not display kill button when onKill is null', () => {
        render(StatusBar, {
            props: {
                ...defaultProps,
                status: ExecutionStatus.RUNNING,
                onKill: null
            }
        });

        expect(screen.queryByText('⏹️ Kill')).not.toBeInTheDocument();
    });

    it('should call onKill when kill button is clicked', async () => {
        const mockKill = vi.fn().mockResolvedValue(undefined);

        render(StatusBar, {
            props: {
                ...defaultProps,
                status: ExecutionStatus.RUNNING,
                onKill: mockKill
            }
        });

        const killButton = screen.getByText('⏹️ Kill');
        fireEvent.click(killButton);

        await waitFor(() => {
            expect(mockKill).toHaveBeenCalledTimes(1);
        });
    });

    it('should show killing state when kill is in progress', async () => {
        const mockKill = vi.fn(
            () =>
                new Promise((resolve) => {
                    setTimeout(resolve, 100);
                })
        );

        render(StatusBar, {
            props: {
                ...defaultProps,
                status: ExecutionStatus.RUNNING,
                onKill: mockKill
            }
        });

        const killButton = screen.getByText('⏹️ Kill');
        fireEvent.click(killButton);

        // Should show "Killing..." while in progress
        await waitFor(() => {
            expect(screen.getByText('⏹️ Killing...')).toBeInTheDocument();
        });
    });

    it('should disable kill button when killInitiated is true', () => {
        const mockKill = vi.fn();

        render(StatusBar, {
            props: {
                ...defaultProps,
                status: ExecutionStatus.RUNNING,
                killInitiated: true,
                onKill: mockKill
            }
        });

        const killButton = screen.getByText('⏹️ Kill');
        expect(killButton).toBeDisabled();
    });

    it('should enable kill button when killInitiated is false and status is killable', () => {
        const mockKill = vi.fn();

        render(StatusBar, {
            props: {
                ...defaultProps,
                status: ExecutionStatus.RUNNING,
                killInitiated: false,
                onKill: mockKill
            }
        });

        const killButton = screen.getByText('⏹️ Kill');
        expect(killButton).not.toBeDisabled();
    });

    it('should display all fields for completed execution', () => {
        render(StatusBar, {
            props: {
                status: ExecutionStatus.SUCCEEDED,
                startedAt: '2025-11-30T10:00:00Z',
                completedAt: '2025-11-30T10:05:00Z',
                exitCode: 0,
                onKill: null,
                command: 'echo test',
                imageId: 'alpine:latest'
            }
        });

        expect(screen.getByText(ExecutionStatus.SUCCEEDED)).toBeInTheDocument();
        expect(screen.getByText('Started:')).toBeInTheDocument();
        expect(screen.getByText('Ended:')).toBeInTheDocument();
        expect(screen.getByText('Exit Code:')).toBeInTheDocument();
        expect(screen.getByText('0')).toBeInTheDocument();
    });

    it('should apply correct CSS class for status badge', () => {
        const { container } = render(StatusBar, {
            props: {
                ...defaultProps,
                status: ExecutionStatus.RUNNING
            }
        });

        const badge = container.querySelector('.status-badge.running');
        expect(badge).toBeInTheDocument();
    });

    it('should apply correct CSS class for STARTING status', () => {
        const { container } = render(StatusBar, {
            props: {
                ...defaultProps,
                status: ExecutionStatus.STARTING
            }
        });

        const badge = container.querySelector('.status-badge.starting');
        expect(badge).toBeInTheDocument();
    });

    it('should format exit code with monospace font', () => {
        const { container } = render(StatusBar, {
            props: {
                ...defaultProps,
                exitCode: 42
            }
        });

        const exitCodeElement = container.querySelector('.exit-code');
        expect(exitCodeElement).toBeInTheDocument();
        expect(exitCodeElement).toHaveTextContent('42');
    });
});
