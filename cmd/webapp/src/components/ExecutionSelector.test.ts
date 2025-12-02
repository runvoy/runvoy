/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/svelte';
import ExecutionSelector from './ExecutionSelector.svelte';

describe('ExecutionSelector', () => {
    afterEach(() => {
        cleanup();
    });

    const defaultProps = {
        executionId: null as string | null,
        onExecutionChange: null as ((id: string) => void) | null
    };

    it('should render execution ID input field', () => {
        render(ExecutionSelector, { props: defaultProps });

        const input = screen.getByPlaceholderText('Enter execution ID');
        expect(input).toBeInTheDocument();
        expect(input).toHaveAttribute('type', 'text');
        expect(input).toHaveAttribute('id', 'exec-id-input');
    });

    it('should display current execution ID from prop', () => {
        render(ExecutionSelector, {
            props: {
                ...defaultProps,
                executionId: 'exec-12345678'
            }
        });

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;
        expect(input.value).toBe('exec-12345678');
    });

    it('should display empty string when execution ID is null', () => {
        render(ExecutionSelector, {
            props: {
                ...defaultProps,
                executionId: null
            }
        });

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;
        expect(input.value).toBe('');
    });

    it('should update input value when user types', async () => {
        render(ExecutionSelector, { props: defaultProps });

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;

        await fireEvent.input(input, { target: { value: 'new-exec-id' } });

        expect(input.value).toBe('new-exec-id');
    });

    it('should call onExecutionChange when Enter is pressed with valid value', async () => {
        const mockOnChange = vi.fn();

        render(ExecutionSelector, {
            props: {
                executionId: 'old-id',
                onExecutionChange: mockOnChange
            }
        });

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;

        await fireEvent.input(input, { target: { value: 'new-exec-id' } });
        await fireEvent.keyPress(input, { key: 'Enter' });

        expect(mockOnChange).toHaveBeenCalledTimes(1);
        expect(mockOnChange).toHaveBeenCalledWith('new-exec-id');
    });

    it('should not call onExecutionChange when Enter is pressed with same value as prop', async () => {
        const mockOnChange = vi.fn();

        render(ExecutionSelector, {
            props: {
                executionId: 'existing-id',
                onExecutionChange: mockOnChange
            }
        });

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;

        await fireEvent.input(input, { target: { value: 'existing-id' } });
        await fireEvent.keyPress(input, { key: 'Enter' });

        expect(mockOnChange).not.toHaveBeenCalled();
    });

    it('should not call onExecutionChange when Enter is pressed with empty value', async () => {
        const mockOnChange = vi.fn();

        render(ExecutionSelector, {
            props: {
                ...defaultProps,
                onExecutionChange: mockOnChange
            }
        });

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;

        await fireEvent.input(input, { target: { value: '   ' } });
        await fireEvent.keyPress(input, { key: 'Enter' });

        expect(mockOnChange).not.toHaveBeenCalled();
    });

    it('should trim whitespace before calling onExecutionChange', async () => {
        const mockOnChange = vi.fn();

        render(ExecutionSelector, {
            props: {
                ...defaultProps,
                onExecutionChange: mockOnChange
            }
        });

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;

        await fireEvent.input(input, { target: { value: '  trimmed-id  ' } });
        await fireEvent.keyPress(input, { key: 'Enter' });

        expect(mockOnChange).toHaveBeenCalledWith('trimmed-id');
    });

    it('should call onExecutionChange when input loses focus with valid value', async () => {
        const mockOnChange = vi.fn();

        render(ExecutionSelector, {
            props: {
                executionId: 'old-id',
                onExecutionChange: mockOnChange
            }
        });

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;

        await fireEvent.input(input, { target: { value: 'new-exec-id' } });
        await fireEvent.blur(input);

        expect(mockOnChange).toHaveBeenCalledTimes(1);
        expect(mockOnChange).toHaveBeenCalledWith('new-exec-id');
    });

    it('should not call onExecutionChange when input loses focus with same value', async () => {
        const mockOnChange = vi.fn();

        render(ExecutionSelector, {
            props: {
                executionId: 'existing-id',
                onExecutionChange: mockOnChange
            }
        });

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;

        await fireEvent.input(input, { target: { value: 'existing-id' } });
        await fireEvent.blur(input);

        expect(mockOnChange).not.toHaveBeenCalled();
    });

    it('should not call onExecutionChange when onExecutionChange prop is null', async () => {
        render(ExecutionSelector, {
            props: {
                ...defaultProps,
                onExecutionChange: null
            }
        });

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;

        await fireEvent.input(input, { target: { value: 'new-id' } });
        await fireEvent.keyPress(input, { key: 'Enter' });

        // Should not throw
    });

    it('should not call onExecutionChange when onExecutionChange prop is undefined', async () => {
        render(ExecutionSelector, { props: defaultProps });

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;

        await fireEvent.input(input, { target: { value: 'new-id' } });
        await fireEvent.keyPress(input, { key: 'Enter' });

        // Should not throw
    });

    it('should not trigger onExecutionChange for non-Enter keys', async () => {
        const mockOnChange = vi.fn();

        render(ExecutionSelector, {
            props: {
                ...defaultProps,
                onExecutionChange: mockOnChange
            }
        });

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;

        await fireEvent.input(input, { target: { value: 'new-id' } });
        await fireEvent.keyPress(input, { key: 'Escape' });
        await fireEvent.keyPress(input, { key: 'Tab' });

        expect(mockOnChange).not.toHaveBeenCalled();
    });

    it('should update display when prop changes', () => {
        const { rerender } = render(ExecutionSelector, {
            props: {
                ...defaultProps,
                executionId: 'initial-id'
            }
        });

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;
        expect(input.value).toBe('initial-id');

        rerender({
            ...defaultProps,
            executionId: 'updated-id'
        });

        expect(input.value).toBe('updated-id');
    });

    it('should have autocomplete disabled', () => {
        render(ExecutionSelector, { props: defaultProps });

        const input = screen.getByPlaceholderText('Enter execution ID');
        expect(input).toHaveAttribute('autocomplete', 'off');
    });

    it('should have proper label association', () => {
        render(ExecutionSelector, { props: defaultProps });

        const label = screen.getByText('Execution ID:');
        const input = screen.getByPlaceholderText('Enter execution ID');

        expect(label).toBeInTheDocument();
        expect(input).toHaveAttribute('id', 'exec-id-input');
    });
});
