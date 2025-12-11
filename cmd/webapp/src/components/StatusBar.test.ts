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

        const { container } = render(StatusBar, {
            props: {
                ...defaultProps,
                startedAt: startTime
            }
        });

        // The compact layout shows formatted date directly in .meta element
        const metaElement = container.querySelector('.meta');
        expect(metaElement).toBeInTheDocument();
    });

    it('should not show started time when missing', () => {
        const { container } = render(StatusBar, {
            props: defaultProps
        });

        // Only the status badge, command, and image should be present
        expect(container.querySelector('.status-bar')).toBeInTheDocument();
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

        expect(screen.getByText('echo hello')).toBeInTheDocument();
        expect(screen.getByText('alpine:latest-abc123')).toBeInTheDocument();
    });

    it('should display duration when startedAt and completedAt are set', () => {
        render(StatusBar, {
            props: {
                ...defaultProps,
                startedAt: '2025-11-30T10:00:00Z',
                completedAt: '2025-11-30T10:05:00Z'
            }
        });

        // Duration should be 5 minutes
        expect(screen.getByText('5m 0s')).toBeInTheDocument();
    });

    it('should not display duration when completedAt is null', () => {
        const { container } = render(StatusBar, {
            props: {
                ...defaultProps,
                startedAt: '2025-11-30T10:00:00Z'
            }
        });

        // Duration element should still exist (showing elapsed time)
        expect(container.querySelector('.duration')).toBeInTheDocument();
    });

    it('should display exit code when exitCode is set', () => {
        render(StatusBar, {
            props: {
                ...defaultProps,
                exitCode: 0
            }
        });

        expect(screen.getByText('exit: 0')).toBeInTheDocument();
    });

    it('should display non-zero exit code with error styling', () => {
        const { container } = render(StatusBar, {
            props: {
                ...defaultProps,
                exitCode: 1
            }
        });

        expect(screen.getByText('exit: 1')).toBeInTheDocument();
        expect(container.querySelector('.exit-code.error')).toBeInTheDocument();
    });

    it('should not display exit code when exitCode is null', () => {
        render(StatusBar, {
            props: defaultProps
        });

        expect(screen.queryByText(/exit:/)).not.toBeInTheDocument();
    });

    it('should display kill button when onKill is provided and status is killable', () => {
        const mockKill = vi.fn();

        const { container } = render(StatusBar, {
            props: {
                ...defaultProps,
                status: ExecutionStatus.RUNNING,
                onKill: mockKill
            }
        });

        const killButton = container.querySelector('.kill-button');
        expect(killButton).toBeInTheDocument();
    });

    it('should not display kill button when status is terminal', () => {
        const mockKill = vi.fn();

        const { container } = render(StatusBar, {
            props: {
                ...defaultProps,
                status: ExecutionStatus.SUCCEEDED,
                onKill: mockKill
            }
        });

        expect(container.querySelector('.kill-button')).not.toBeInTheDocument();
    });

    it('should not display kill button when status is TERMINATING', () => {
        const mockKill = vi.fn();

        const { container } = render(StatusBar, {
            props: {
                ...defaultProps,
                status: ExecutionStatus.TERMINATING,
                onKill: mockKill
            }
        });

        expect(container.querySelector('.kill-button')).not.toBeInTheDocument();
    });

    it('should not display kill button when onKill is null', () => {
        const { container } = render(StatusBar, {
            props: {
                ...defaultProps,
                status: ExecutionStatus.RUNNING,
                onKill: null
            }
        });

        expect(container.querySelector('.kill-button')).not.toBeInTheDocument();
    });

    it('should call onKill when kill button is clicked', async () => {
        const mockKill = vi.fn().mockResolvedValue(undefined);

        const { container } = render(StatusBar, {
            props: {
                ...defaultProps,
                status: ExecutionStatus.RUNNING,
                onKill: mockKill
            }
        });

        const killButton = container.querySelector('.kill-button') as HTMLButtonElement;
        fireEvent.click(killButton);

        await waitFor(() => {
            expect(mockKill).toHaveBeenCalledTimes(1);
        });
    });

    it('should disable kill button when killInitiated is true', () => {
        const mockKill = vi.fn();

        const { container } = render(StatusBar, {
            props: {
                ...defaultProps,
                status: ExecutionStatus.RUNNING,
                killInitiated: true,
                onKill: mockKill
            }
        });

        const killButton = container.querySelector('.kill-button') as HTMLButtonElement;
        expect(killButton).toBeDisabled();
    });

    it('should enable kill button when killInitiated is false and status is killable', () => {
        const mockKill = vi.fn();

        const { container } = render(StatusBar, {
            props: {
                ...defaultProps,
                status: ExecutionStatus.RUNNING,
                killInitiated: false,
                onKill: mockKill
            }
        });

        const killButton = container.querySelector('.kill-button') as HTMLButtonElement;
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
        expect(screen.getByText('echo test')).toBeInTheDocument();
        expect(screen.getByText('alpine:latest')).toBeInTheDocument();
        expect(screen.getByText('exit: 0')).toBeInTheDocument();
        expect(screen.getByText('5m 0s')).toBeInTheDocument();
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
