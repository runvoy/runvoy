/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import { get } from 'svelte/store';
import StatusBar from './StatusBar.svelte';
import {
    executionStatus,
    startedAt,
    isCompleted,
    completedAt,
    exitCode
} from '../stores/execution';
import { ExecutionStatus, FrontendStatus } from '../lib/constants';

describe('StatusBar', () => {
    beforeEach(() => {
        // Reset stores
        executionStatus.set(null);
        startedAt.set(null);
        isCompleted.set(false);
        completedAt.set(null);
        exitCode.set(null);
    });

    afterEach(() => {
        vi.clearAllMocks();
    });

    it('should display status badge with LOADING when status is null', () => {
        render(StatusBar, {
            props: {
                onKill: null
            }
        });

        expect(screen.getByText(FrontendStatus.LOADING)).toBeInTheDocument();
        expect(screen.getByText('Status:')).toBeInTheDocument();
    });

    it('should display execution status', () => {
        executionStatus.set(ExecutionStatus.RUNNING);

        render(StatusBar, {
            props: {
                onKill: null
            }
        });

        expect(screen.getByText(ExecutionStatus.RUNNING)).toBeInTheDocument();
    });

    it('should display started time', () => {
        const startTime = '2025-11-30T10:00:00Z';
        startedAt.set(startTime);

        render(StatusBar, {
            props: {
                onKill: null
            }
        });

        expect(screen.getByText('Started:')).toBeInTheDocument();
        // The date will be formatted, so we check for parts of it
        const startedText = screen.getByText(/Started:/).parentElement?.textContent;
        expect(startedText).toContain('Started:');
    });

    it('should display N/A when started time is missing', () => {
        render(StatusBar, {
            props: {
                onKill: null
            }
        });

        expect(screen.getByText('N/A')).toBeInTheDocument();
    });

    it('should display ended time when completedAt is set', () => {
        const endTime = '2025-11-30T10:05:00Z';
        completedAt.set(endTime);

        render(StatusBar, {
            props: {
                onKill: null
            }
        });

        expect(screen.getByText('Ended:')).toBeInTheDocument();
    });

    it('should not display ended time when completedAt is null', () => {
        render(StatusBar, {
            props: {
                onKill: null
            }
        });

        expect(screen.queryByText('Ended:')).not.toBeInTheDocument();
    });

    it('should display exit code when exitCode is set', () => {
        exitCode.set(0);

        render(StatusBar, {
            props: {
                onKill: null
            }
        });

        expect(screen.getByText('Exit Code:')).toBeInTheDocument();
        expect(screen.getByText('0')).toBeInTheDocument();
    });

    it('should display non-zero exit code', () => {
        exitCode.set(1);

        render(StatusBar, {
            props: {
                onKill: null
            }
        });

        expect(screen.getByText('1')).toBeInTheDocument();
    });

    it('should not display exit code when exitCode is null', () => {
        render(StatusBar, {
            props: {
                onKill: null
            }
        });

        expect(screen.queryByText('Exit Code:')).not.toBeInTheDocument();
    });

    it('should display kill button when onKill is provided and execution is not completed', () => {
        const mockKill = vi.fn();
        isCompleted.set(false);

        render(StatusBar, {
            props: {
                onKill: mockKill
            }
        });

        const killButton = screen.getByText('⏹️ Kill');
        expect(killButton).toBeInTheDocument();
    });

    it('should not display kill button when execution is completed', () => {
        const mockKill = vi.fn();
        isCompleted.set(true);

        render(StatusBar, {
            props: {
                onKill: mockKill
            }
        });

        expect(screen.queryByText('⏹️ Kill')).not.toBeInTheDocument();
    });

    it('should not display kill button when onKill is null', () => {
        isCompleted.set(false);

        render(StatusBar, {
            props: {
                onKill: null
            }
        });

        expect(screen.queryByText('⏹️ Kill')).not.toBeInTheDocument();
    });

    it('should call onKill when kill button is clicked', async () => {
        const mockKill = vi.fn().mockResolvedValue(undefined);
        isCompleted.set(false);

        render(StatusBar, {
            props: {
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
        isCompleted.set(false);

        render(StatusBar, {
            props: {
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

    it('should disable kill button when execution is completed', () => {
        const mockKill = vi.fn();
        isCompleted.set(true);

        render(StatusBar, {
            props: {
                onKill: mockKill
            }
        });

        // Button should not be visible when completed
        expect(screen.queryByText('⏹️ Kill')).not.toBeInTheDocument();
    });

    it('should display all fields for completed execution', () => {
        executionStatus.set(ExecutionStatus.SUCCEEDED);
        startedAt.set('2025-11-30T10:00:00Z');
        completedAt.set('2025-11-30T10:05:00Z');
        exitCode.set(0);
        isCompleted.set(true);

        render(StatusBar, {
            props: {
                onKill: null
            }
        });

        expect(screen.getByText(ExecutionStatus.SUCCEEDED)).toBeInTheDocument();
        expect(screen.getByText('Started:')).toBeInTheDocument();
        expect(screen.getByText('Ended:')).toBeInTheDocument();
        expect(screen.getByText('Exit Code:')).toBeInTheDocument();
        expect(screen.getByText('0')).toBeInTheDocument();
    });

    it('should apply correct CSS class for status badge', () => {
        executionStatus.set(ExecutionStatus.RUNNING);

        const { container } = render(StatusBar, {
            props: {
                onKill: null
            }
        });

        const badge = container.querySelector('.status-badge.running');
        expect(badge).toBeInTheDocument();
    });

    it('should apply correct CSS class for STARTING status', () => {
        executionStatus.set(ExecutionStatus.STARTING);

        const { container } = render(StatusBar, {
            props: {
                onKill: null
            }
        });

        const badge = container.querySelector('.status-badge.starting');
        expect(badge).toBeInTheDocument();
    });

    it('should format exit code with monospace font', () => {
        exitCode.set(42);

        const { container } = render(StatusBar, {
            props: {
                onKill: null
            }
        });

        const exitCodeElement = container.querySelector('.exit-code');
        expect(exitCodeElement).toBeInTheDocument();
        expect(exitCodeElement).toHaveTextContent('42');
    });
});
