/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/svelte';
import RunView from './RunView.svelte';
import type APIClient from '../lib/api';
import type { RunCommandResponse } from '../types/api';
import { switchExecution } from '../lib/executionState';
import { cachedWebSocketURL } from '../stores/websocket';
import { get } from 'svelte/store';
import * as navigation from '$app/navigation';

// Mock the modules
vi.mock('../lib/executionState', () => ({
    switchExecution: vi.fn()
}));

vi.mock('$app/navigation', () => ({
    goto: vi.fn()
}));

describe('RunView', () => {
    let mockApiClient: Partial<APIClient>;

    beforeEach(() => {
        mockApiClient = {
            runCommand: vi.fn()
        };
        cachedWebSocketURL.set(null);
        vi.clearAllMocks();
    });

    afterEach(() => {
        cleanup();
        vi.clearAllMocks();
        cachedWebSocketURL.set(null);
    });

    it('should render command input field', () => {
        render(RunView, {
            props: {
                apiClient: mockApiClient as APIClient
            }
        });

        const commandInput = screen.getByPlaceholderText('e.g. uname -a');
        expect(commandInput).toBeInTheDocument();
        expect(commandInput.tagName).toBe('TEXTAREA');
    });

    it('should render submit button', () => {
        render(RunView, {
            props: {
                apiClient: mockApiClient as APIClient
            }
        });

        const submitButton = screen.getByText('Run command');
        expect(submitButton).toBeInTheDocument();
        expect(submitButton.tagName).toBe('BUTTON');
    });

    it('should show advanced options toggle', () => {
        render(RunView, {
            props: {
                apiClient: mockApiClient as APIClient
            }
        });

        const toggleButton = screen.getByText('Show advanced options');
        expect(toggleButton).toBeInTheDocument();
    });

    it('should toggle advanced options visibility', async () => {
        render(RunView, {
            props: {
                apiClient: mockApiClient as APIClient
            }
        });

        const toggleButton = screen.getByText('Show advanced options');
        await fireEvent.click(toggleButton);

        expect(screen.getByText('Hide advanced options')).toBeInTheDocument();
        expect(screen.getByPlaceholderText('Optional image override')).toBeInTheDocument();
    });

    it('should show error when API client is not available', async () => {
        render(RunView, {
            props: {
                apiClient: null
            }
        });

        const commandInput = screen.getByPlaceholderText('e.g. uname -a');
        await fireEvent.input(commandInput, { target: { value: 'echo test' } });

        const submitButton = screen.getByText('Run command');
        await fireEvent.click(submitButton);

        await waitFor(() => {
            expect(screen.getByText('API client is not available.')).toBeInTheDocument();
        });
    });

    it('should show error when command is empty', async () => {
        render(RunView, {
            props: {
                apiClient: mockApiClient as APIClient
            }
        });

        const form = document.querySelector('form');
        if (form) {
            await fireEvent.submit(form);
        } else {
            const submitButton = screen.getByText('Run command');
            await fireEvent.click(submitButton);
        }

        await waitFor(() => {
            expect(screen.getByText('Command is required.')).toBeInTheDocument();
        });
    });

    it('should submit command successfully', async () => {
        const mockResponse: RunCommandResponse = {
            execution_id: 'exec-123',
            status: 'RUNNING',
            websocket_url: 'wss://example.com/logs'
        };

        vi.mocked(mockApiClient.runCommand as any).mockResolvedValue(mockResponse);

        render(RunView, {
            props: {
                apiClient: mockApiClient as APIClient
            }
        });

        const commandInput = screen.getByPlaceholderText('e.g. uname -a');
        await fireEvent.input(commandInput, { target: { value: 'echo hello' } });

        const submitButton = screen.getByText('Run command');
        await fireEvent.click(submitButton);

        await waitFor(() => {
            expect(mockApiClient.runCommand).toHaveBeenCalledWith({
                command: 'echo hello'
            });
            expect(switchExecution).toHaveBeenCalledWith('exec-123');
            expect(get(cachedWebSocketURL)).toBe('wss://example.com/logs');
            expect(navigation.goto).toHaveBeenCalledWith('/logs?execution_id=exec-123');
        });
    });

    it('should trim command before submitting', async () => {
        const mockResponse: RunCommandResponse = {
            execution_id: 'exec-123',
            status: 'RUNNING'
        };

        vi.mocked(mockApiClient.runCommand as any).mockResolvedValue(mockResponse);

        render(RunView, {
            props: {
                apiClient: mockApiClient as APIClient
            }
        });

        const commandInput = screen.getByPlaceholderText('e.g. uname -a');
        await fireEvent.input(commandInput, { target: { value: '  echo hello  ' } });

        const submitButton = screen.getByText('Run command');
        await fireEvent.click(submitButton);

        await waitFor(() => {
            expect(mockApiClient.runCommand).toHaveBeenCalledWith({
                command: 'echo hello'
            });
        });
    });

    it('should include image in payload when provided', async () => {
        const mockResponse: RunCommandResponse = {
            execution_id: 'exec-123',
            status: 'RUNNING'
        };

        vi.mocked(mockApiClient.runCommand as any).mockResolvedValue(mockResponse);

        render(RunView, {
            props: {
                apiClient: mockApiClient as APIClient
            }
        });

        // Show advanced options
        const toggleButton = screen.getByText('Show advanced options');
        await fireEvent.click(toggleButton);

        const commandInput = screen.getByPlaceholderText('e.g. uname -a');
        await fireEvent.input(commandInput, { target: { value: 'echo test' } });

        const imageInput = screen.getByPlaceholderText('Optional image override');
        await fireEvent.input(imageInput, { target: { value: 'custom-image:latest' } });

        const submitButton = screen.getByText('Run command');
        await fireEvent.click(submitButton);

        await waitFor(() => {
            expect(mockApiClient.runCommand).toHaveBeenCalledWith({
                command: 'echo test',
                image: 'custom-image:latest'
            });
        });
    });

    it('should include git repository in payload when provided', async () => {
        const mockResponse: RunCommandResponse = {
            execution_id: 'exec-123',
            status: 'RUNNING'
        };

        vi.mocked(mockApiClient.runCommand as any).mockResolvedValue(mockResponse);

        render(RunView, {
            props: {
                apiClient: mockApiClient as APIClient
            }
        });

        // Show advanced options
        const toggleButton = screen.getByText('Show advanced options');
        await fireEvent.click(toggleButton);

        const commandInput = screen.getByPlaceholderText('e.g. uname -a');
        await fireEvent.input(commandInput, { target: { value: 'echo test' } });

        const gitRepoInput = screen.getByPlaceholderText('https://github.com/runvoy/runvoy');
        await fireEvent.input(gitRepoInput, { target: { value: 'https://github.com/user/repo' } });

        const submitButton = screen.getByText('Run command');
        await fireEvent.click(submitButton);

        await waitFor(() => {
            expect(mockApiClient.runCommand).toHaveBeenCalledWith({
                command: 'echo test',
                git_repo: 'https://github.com/user/repo'
            });
        });
    });

    it('should include git ref and path when provided', async () => {
        const mockResponse: RunCommandResponse = {
            execution_id: 'exec-123',
            status: 'RUNNING'
        };

        vi.mocked(mockApiClient.runCommand as any).mockResolvedValue(mockResponse);

        render(RunView, {
            props: {
                apiClient: mockApiClient as APIClient
            }
        });

        // Show advanced options
        const toggleButton = screen.getByText('Show advanced options');
        await fireEvent.click(toggleButton);

        const commandInput = screen.getByPlaceholderText('e.g. uname -a');
        await fireEvent.input(commandInput, { target: { value: 'echo test' } });

        const gitRepoInput = screen.getByPlaceholderText('https://github.com/runvoy/runvoy');
        await fireEvent.input(gitRepoInput, { target: { value: 'https://github.com/user/repo' } });

        const gitRefInput = screen.getByPlaceholderText('main');
        await fireEvent.input(gitRefInput, { target: { value: 'develop' } });

        const gitPathInput = screen.getByPlaceholderText('.');
        await fireEvent.input(gitPathInput, { target: { value: 'subdir' } });

        const submitButton = screen.getByText('Run command');
        await fireEvent.click(submitButton);

        await waitFor(() => {
            expect(mockApiClient.runCommand).toHaveBeenCalledWith({
                command: 'echo test',
                git_repo: 'https://github.com/user/repo',
                git_ref: 'develop',
                git_path: 'subdir'
            });
        });
    });

    it('should add environment variable rows', async () => {
        render(RunView, {
            props: {
                apiClient: mockApiClient as APIClient
            }
        });

        // Show advanced options
        const toggleButton = screen.getByText('Show advanced options');
        await fireEvent.click(toggleButton);

        const addEnvButton = screen.getByText('Add environment variable');
        await fireEvent.click(addEnvButton);

        // Should have 2 env rows now (initial + added)
        const envKeyInputs = screen.getAllByPlaceholderText('KEY');
        expect(envKeyInputs.length).toBeGreaterThanOrEqual(2);
    });

    it('should remove environment variable rows', async () => {
        render(RunView, {
            props: {
                apiClient: mockApiClient as APIClient
            }
        });

        // Show advanced options
        const toggleButton = screen.getByText('Show advanced options');
        await fireEvent.click(toggleButton);

        // Add an env row
        const addEnvButton = screen.getByText('Add environment variable');
        await fireEvent.click(addEnvButton);

        // Remove buttons should be present
        const removeButtons = screen.getAllByLabelText('Remove environment variable');
        expect(removeButtons.length).toBeGreaterThan(0);

        // Click remove on first row
        await fireEvent.click(removeButtons[0]);

        // Should still have at least one row (can't remove the last one)
        const envKeyInputs = screen.getAllByPlaceholderText('KEY');
        expect(envKeyInputs.length).toBeGreaterThanOrEqual(1);
    });

    it('should include environment variables in payload', async () => {
        const mockResponse: RunCommandResponse = {
            execution_id: 'exec-123',
            status: 'RUNNING'
        };

        vi.mocked(mockApiClient.runCommand as any).mockResolvedValue(mockResponse);

        render(RunView, {
            props: {
                apiClient: mockApiClient as APIClient
            }
        });

        // Show advanced options
        const toggleButton = screen.getByText('Show advanced options');
        await fireEvent.click(toggleButton);

        const commandInput = screen.getByPlaceholderText('e.g. uname -a');
        await fireEvent.input(commandInput, { target: { value: 'echo test' } });

        // Get the first env row inputs
        const envKeyInputs = screen.getAllByPlaceholderText('KEY');
        const envValueInputs = screen.getAllByPlaceholderText('value');

        await fireEvent.input(envKeyInputs[0], { target: { value: 'DEBUG' } });
        await fireEvent.input(envValueInputs[0], { target: { value: 'true' } });

        const submitButton = screen.getByText('Run command');
        await fireEvent.click(submitButton);

        await waitFor(() => {
            expect(mockApiClient.runCommand).toHaveBeenCalledWith({
                command: 'echo test',
                env: {
                    DEBUG: 'true'
                }
            });
        });
    });

    it('should not include empty environment variables in payload', async () => {
        const mockResponse: RunCommandResponse = {
            execution_id: 'exec-123',
            status: 'RUNNING'
        };

        vi.mocked(mockApiClient.runCommand as any).mockResolvedValue(mockResponse);

        render(RunView, {
            props: {
                apiClient: mockApiClient as APIClient
            }
        });

        // Show advanced options
        const toggleButton = screen.getByText('Show advanced options');
        await fireEvent.click(toggleButton);

        const commandInput = screen.getByPlaceholderText('e.g. uname -a');
        await fireEvent.input(commandInput, { target: { value: 'echo test' } });

        // Leave env vars empty
        const submitButton = screen.getByText('Run command');
        await fireEvent.click(submitButton);

        await waitFor(() => {
            const call = vi.mocked(mockApiClient.runCommand as any).mock.calls[0][0];
            expect(call.env).toBeUndefined();
        });
    });

    it('should show error message when command fails', async () => {
        const error = new Error('Failed to start command');
        (error as any).details = { error: 'Invalid command' };

        vi.mocked(mockApiClient.runCommand as any).mockRejectedValue(error);

        render(RunView, {
            props: {
                apiClient: mockApiClient as APIClient
            }
        });

        const commandInput = screen.getByPlaceholderText('e.g. uname -a');
        await fireEvent.input(commandInput, { target: { value: 'invalid command' } });

        const submitButton = screen.getByText('Run command');
        await fireEvent.click(submitButton);

        await waitFor(() => {
            expect(screen.getByText('Invalid command')).toBeInTheDocument();
        });
    });

    it('should show loading state while submitting', async () => {
        let resolveCommand: (value: RunCommandResponse) => void;
        const commandPromise = new Promise<RunCommandResponse>((resolve) => {
            resolveCommand = resolve;
        });

        vi.mocked(mockApiClient.runCommand as any).mockReturnValue(commandPromise);

        render(RunView, {
            props: {
                apiClient: mockApiClient as APIClient
            }
        });

        const commandInput = screen.getByPlaceholderText('e.g. uname -a');
        await fireEvent.input(commandInput, { target: { value: 'echo test' } });

        const submitButton = screen.getByText('Run command');
        await fireEvent.click(submitButton);

        await waitFor(() => {
            expect(screen.getByText('Starting...')).toBeInTheDocument();
            expect(submitButton).toBeDisabled();
        });

        resolveCommand!({
            execution_id: 'exec-123',
            status: 'RUNNING'
        });

        await waitFor(() => {
            expect(screen.getByText('Run command')).toBeInTheDocument();
        });
    });

    it('should not navigate if execution_id is missing', async () => {
        const mockResponse: RunCommandResponse = {
            execution_id: '',
            status: 'RUNNING'
        } as any;

        vi.mocked(mockApiClient.runCommand as any).mockResolvedValue(mockResponse);

        render(RunView, {
            props: {
                apiClient: mockApiClient as APIClient
            }
        });

        const commandInput = screen.getByPlaceholderText('e.g. uname -a');
        await fireEvent.input(commandInput, { target: { value: 'echo test' } });

        const submitButton = screen.getByText('Run command');
        await fireEvent.click(submitButton);

        await waitFor(() => {
            expect(switchExecution).not.toHaveBeenCalled();
            expect(navigation.goto).not.toHaveBeenCalled();
        });
    });
});
