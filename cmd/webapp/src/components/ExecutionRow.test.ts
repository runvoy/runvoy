/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/svelte';
import ExecutionRow from './ExecutionRow.svelte';
import type { Execution } from '../types/api';

describe('ExecutionRow', () => {
    let mockOnView: (execution: Execution) => void;
    let mockExecution: Execution;

    beforeEach(() => {
        mockOnView = vi.fn();
        mockExecution = {
            execution_id: 'exec-12345678-abcd-efgh',
            status: 'SUCCEEDED',
            started_at: '2025-11-21T10:00:00Z',
            completed_at: '2025-11-21T10:01:00Z',
            exit_code: 0
        };
    });

    afterEach(() => {
        cleanup();
        // Remove any tables that were added to the body
        document.body.querySelectorAll('table').forEach((table) => table.remove());
    });

    const renderExecutionRow = (execution: Execution) => {
        const table = document.createElement('table');
        const tbody = document.createElement('tbody');
        table.appendChild(tbody);
        document.body.appendChild(table);

        const { container } = render(ExecutionRow, {
            props: {
                execution,
                onView: mockOnView as (execution: Execution) => void
            },
            target: tbody
        });

        return { container, table };
    };

    it('should render execution ID truncated to 8 characters', () => {
        renderExecutionRow(mockExecution);

        const codeElement = screen.getByText('exec-123...');
        expect(codeElement).toBeInTheDocument();
        expect(codeElement.tagName).toBe('CODE');
    });

    it('should render "N/A" when execution ID is missing', () => {
        const executionWithoutId: Execution = {
            ...mockExecution,
            execution_id: undefined as any
        };

        renderExecutionRow(executionWithoutId);

        expect(screen.getByText('N/A')).toBeInTheDocument();
    });

    it('should render status badge with SUCCEEDED status', () => {
        renderExecutionRow(mockExecution);

        const statusBadge = screen.getByText('SUCCEEDED');
        expect(statusBadge).toBeInTheDocument();
        expect(statusBadge).toHaveClass('status-badge');
        expect(statusBadge).toHaveClass('status-success');
    });

    it('should render status badge with FAILED status and danger color', () => {
        const failedExecution: Execution = {
            ...mockExecution,
            status: 'FAILED'
        };

        renderExecutionRow(failedExecution);

        const statusBadge = screen.getByText('FAILED');
        expect(statusBadge).toHaveClass('status-danger');
    });

    it('should render status badge with STOPPED status and danger color', () => {
        const stoppedExecution: Execution = {
            ...mockExecution,
            status: 'STOPPED'
        };

        renderExecutionRow(stoppedExecution);

        const statusBadge = screen.getByText('STOPPED');
        expect(statusBadge).toHaveClass('status-danger');
    });

    it('should render status badge with RUNNING status and info color', () => {
        const runningExecution: Execution = {
            ...mockExecution,
            status: 'RUNNING'
        };

        renderExecutionRow(runningExecution);

        const statusBadge = screen.getByText('RUNNING');
        expect(statusBadge).toHaveClass('status-info');
    });

    it('should render status badge with unknown status and default color', () => {
        const unknownExecution: Execution = {
            ...mockExecution,
            status: 'UNKNOWN_STATUS'
        };

        renderExecutionRow(unknownExecution);

        const statusBadge = screen.getByText('UNKNOWN_STATUS');
        expect(statusBadge).toHaveClass('status-default');
    });

    it('should format and display started_at date', () => {
        renderExecutionRow(mockExecution);

        const cells = screen.getAllByRole('cell');
        const startedCell = cells.find((cell) => cell.textContent?.includes('2025'));
        expect(startedCell).toBeInTheDocument();
    });

    it('should display "-" when started_at is missing', () => {
        const executionWithoutStarted: Execution = {
            ...mockExecution,
            started_at: undefined as any
        };

        renderExecutionRow(executionWithoutStarted);

        const dashes = screen.getAllByText('-');
        expect(dashes.length).toBeGreaterThan(0);
    });

    it('should format and display completed_at date', () => {
        renderExecutionRow(mockExecution);

        const cells = screen.getAllByRole('cell');
        expect(cells.some((cell) => cell.textContent?.includes('2025'))).toBe(true);
    });

    it('should display "-" when completed_at is missing', () => {
        const executionWithoutCompleted: Execution = {
            ...mockExecution,
            completed_at: undefined
        };

        renderExecutionRow(executionWithoutCompleted);

        const dashes = screen.getAllByText('-');
        expect(dashes.length).toBeGreaterThan(0);
    });

    it('should display exit code when present', () => {
        renderExecutionRow(mockExecution);

        expect(screen.getByText('0')).toBeInTheDocument();
    });

    it('should display "-" when exit code is missing', () => {
        const executionWithoutExitCode: Execution = {
            ...mockExecution,
            exit_code: undefined
        };

        renderExecutionRow(executionWithoutExitCode);

        const dashes = screen.getAllByText('-');
        expect(dashes.some((dash) => dash.textContent === '-')).toBe(true);
    });

    it('should display exit code 1 for failed execution', () => {
        const failedExecution: Execution = {
            ...mockExecution,
            status: 'FAILED',
            exit_code: 1
        };

        renderExecutionRow(failedExecution);

        expect(screen.getByText('1')).toBeInTheDocument();
    });

    it('should render View button', () => {
        renderExecutionRow(mockExecution);

        const viewButton = screen.getByText('View');
        expect(viewButton).toBeInTheDocument();
        expect(viewButton.tagName).toBe('BUTTON');
        expect(viewButton).toHaveClass('secondary');
    });

    it('should have proper aria-label on View button', () => {
        renderExecutionRow(mockExecution);

        const viewButton = screen.getByLabelText(`View execution ${mockExecution.execution_id}`);
        expect(viewButton).toBeInTheDocument();
    });

    it('should call onView callback when View button is clicked', () => {
        renderExecutionRow(mockExecution);

        const viewButton = screen.getByText('View');
        fireEvent.click(viewButton);

        expect(mockOnView).toHaveBeenCalledTimes(1);
        expect(mockOnView).toHaveBeenCalledWith(mockExecution);
    });

    it('should render all table cells', () => {
        renderExecutionRow(mockExecution);

        const cells = screen.getAllByRole('cell');
        expect(cells.length).toBe(6); // ID, Status, Started, Completed, Exit Code, Action
    });

    it('should handle execution with short ID', () => {
        const shortIdExecution: Execution = {
            ...mockExecution,
            execution_id: 'abc123'
        };

        renderExecutionRow(shortIdExecution);

        expect(screen.getByText('abc123...')).toBeInTheDocument();
    });

    it('should handle invalid date strings gracefully', () => {
        const invalidDateExecution: Execution = {
            ...mockExecution,
            started_at: 'not-a-date'
        };

        renderExecutionRow(invalidDateExecution);

        // Invalid dates are formatted as "Invalid Date" by toLocaleString()
        expect(screen.getByText('Invalid Date')).toBeInTheDocument();
    });

    it('should render with correct CSS classes', () => {
        const { container } = renderExecutionRow(mockExecution);

        const executionIdCell = container.querySelector('.execution-id');
        expect(executionIdCell).toBeInTheDocument();

        const exitCodeCell = container.querySelector('.exit-code');
        expect(exitCodeCell).toBeInTheDocument();

        const actionCell = container.querySelector('.action-cell');
        expect(actionCell).toBeInTheDocument();
    });
});
