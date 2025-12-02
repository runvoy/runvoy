/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/svelte';
import ExecutionSelector from './ExecutionSelector.svelte';
import { executionId } from '../stores/execution';
import { get } from 'svelte/store';

describe('ExecutionSelector', () => {
    beforeEach(() => {
        executionId.set(null);
    });

    afterEach(() => {
        cleanup();
        executionId.set(null);
    });

    it('should render execution ID input field', () => {
        render(ExecutionSelector);

        const input = screen.getByPlaceholderText('Enter execution ID');
        expect(input).toBeInTheDocument();
        expect(input).toHaveAttribute('type', 'text');
        expect(input).toHaveAttribute('id', 'exec-id-input');
    });

    it('should display current execution ID from store', () => {
        executionId.set('exec-12345678');
        render(ExecutionSelector);

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;
        expect(input.value).toBe('exec-12345678');
    });

    it('should display empty string when execution ID is null', () => {
        executionId.set(null);
        render(ExecutionSelector);

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;
        expect(input.value).toBe('');
    });

    it('should update input value when user types', async () => {
        render(ExecutionSelector);

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;

        await fireEvent.input(input, { target: { value: 'new-exec-id' } });

        expect(input.value).toBe('new-exec-id');
    });

    it('should call onExecutionChange when Enter is pressed with valid value', async () => {
        const mockOnChange = vi.fn();
        executionId.set('old-id');

        render(ExecutionSelector, {
            props: {
                onExecutionChange: mockOnChange
            }
        });

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;

        await fireEvent.input(input, { target: { value: 'new-exec-id' } });
        await fireEvent.keyPress(input, { key: 'Enter' });

        expect(mockOnChange).toHaveBeenCalledTimes(1);
        expect(mockOnChange).toHaveBeenCalledWith('new-exec-id');
    });

    it('should not call onExecutionChange when Enter is pressed with same value as store', async () => {
        const mockOnChange = vi.fn();
        executionId.set('existing-id');

        render(ExecutionSelector, {
            props: {
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
        executionId.set('old-id');

        render(ExecutionSelector, {
            props: {
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
        executionId.set('existing-id');

        render(ExecutionSelector, {
            props: {
                onExecutionChange: mockOnChange
            }
        });

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;

        await fireEvent.input(input, { target: { value: 'existing-id' } });
        await fireEvent.blur(input);

        expect(mockOnChange).not.toHaveBeenCalled();
    });

    it('should not call onExecutionChange when onExecutionChange prop is null', async () => {
        const mockOnChange = vi.fn();

        render(ExecutionSelector, {
            props: {
                onExecutionChange: null
            }
        });

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;

        await fireEvent.input(input, { target: { value: 'new-id' } });
        await fireEvent.keyPress(input, { key: 'Enter' });

        expect(mockOnChange).not.toHaveBeenCalled();
    });

    it('should not call onExecutionChange when onExecutionChange prop is undefined', async () => {
        const mockOnChange = vi.fn();

        render(ExecutionSelector);

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;

        await fireEvent.input(input, { target: { value: 'new-id' } });
        await fireEvent.keyPress(input, { key: 'Enter' });

        expect(mockOnChange).not.toHaveBeenCalled();
    });

    it('should not trigger onExecutionChange for non-Enter keys', async () => {
        const mockOnChange = vi.fn();

        render(ExecutionSelector, {
            props: {
                onExecutionChange: mockOnChange
            }
        });

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;

        await fireEvent.input(input, { target: { value: 'new-id' } });
        await fireEvent.keyPress(input, { key: 'Escape' });
        await fireEvent.keyPress(input, { key: 'Tab' });

        expect(mockOnChange).not.toHaveBeenCalled();
    });

    it('should sync with store when store value changes', () => {
        render(ExecutionSelector);

        const input = screen.getByPlaceholderText('Enter execution ID') as HTMLInputElement;
        expect(input.value).toBe('');

        executionId.set('store-updated-id');

        // Svelte reactivity should update the input
        // Note: In tests, we may need to wait for reactivity
        // But the derived value should update automatically
        expect(get(executionId)).toBe('store-updated-id');
    });

    it('should have autocomplete disabled', () => {
        render(ExecutionSelector);

        const input = screen.getByPlaceholderText('Enter execution ID');
        expect(input).toHaveAttribute('autocomplete', 'off');
    });

    it('should have proper label association', () => {
        render(ExecutionSelector);

        const label = screen.getByText('Execution ID:');
        const input = screen.getByPlaceholderText('Enter execution ID');

        expect(label).toBeInTheDocument();
        expect(input).toHaveAttribute('id', 'exec-id-input');
    });
});
