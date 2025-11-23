/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import ConnectionManager from './ConnectionManager.svelte';
import { apiEndpoint, apiKey } from '../stores/config';
import { get } from 'svelte/store';

describe('ConnectionManager', () => {
    beforeEach(() => {
        // Reset stores before each test
        apiEndpoint.set(null);
        apiKey.set(null);
        vi.clearAllMocks();
    });

    afterEach(() => {
        // Clean up after each test
        apiEndpoint.set(null);
        apiKey.set(null);
    });

    describe('Modal visibility', () => {
        it('should render the configure button', () => {
            render(ConnectionManager);
            expect(screen.getByText('⚙️ Configure API')).toBeInTheDocument();
        });

        it('should not show modal initially', () => {
            render(ConnectionManager);
            expect(screen.queryByText('API Configuration')).not.toBeInTheDocument();
        });

        it('should open modal when button is clicked', async () => {
            render(ConnectionManager);
            const button = screen.getByText('⚙️ Configure API');

            await fireEvent.click(button);

            expect(screen.getByText('API Configuration')).toBeInTheDocument();
        });

        it('should close modal when cancel button is clicked', async () => {
            render(ConnectionManager);
            const configButton = screen.getByText('⚙️ Configure API');
            await fireEvent.click(configButton);

            const cancelButton = screen.getByText('Cancel');
            await fireEvent.click(cancelButton);

            await waitFor(() => {
                expect(screen.queryByText('API Configuration')).not.toBeInTheDocument();
            });
        });

        it('should close modal when clicking backdrop', async () => {
            render(ConnectionManager);
            const configButton = screen.getByText('⚙️ Configure API');
            await fireEvent.click(configButton);

            const backdrop = document.querySelector('.modal-backdrop');
            expect(backdrop).toBeInTheDocument();

            await fireEvent.click(backdrop!);

            await waitFor(() => {
                expect(screen.queryByText('API Configuration')).not.toBeInTheDocument();
            });
        });

        it('should not close modal when clicking modal content', async () => {
            render(ConnectionManager);
            const configButton = screen.getByText('⚙️ Configure API');
            await fireEvent.click(configButton);

            const modalContent = document.querySelector('.modal-content');
            expect(modalContent).toBeInTheDocument();

            await fireEvent.click(modalContent!);

            // Modal should still be visible
            expect(screen.getByText('API Configuration')).toBeInTheDocument();
        });
    });

    describe('Form inputs', () => {
        it('should display endpoint input field', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            expect(screen.getByLabelText(/API Endpoint:/)).toBeInTheDocument();
        });

        it('should display API key input field as optional', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            expect(screen.getByLabelText(/API Key \(optional\):/)).toBeInTheDocument();
        });

        it('should show placeholder text for endpoint', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const endpointInput = screen.getByPlaceholderText('https://api.runvoy.example.com');
            expect(endpointInput).toBeInTheDocument();
        });

        it('should show helpful placeholder for API key', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const apiKeyInput = screen.getByPlaceholderText(/Enter API key \(or claim one later\)/);
            expect(apiKeyInput).toBeInTheDocument();
        });

        it('should show help text about claim flow', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            expect(
                screen.getByText(/Leave empty to claim one using an invitation token/)
            ).toBeInTheDocument();
        });

        it('should allow typing in endpoint field', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const endpointInput = screen.getByLabelText(/API Endpoint:/) as HTMLInputElement;
            await fireEvent.input(endpointInput, { target: { value: 'https://api.example.com' } });

            expect(endpointInput.value).toBe('https://api.example.com');
        });

        it('should allow typing in API key field', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const apiKeyInput = screen.getByLabelText(/API Key \(optional\):/) as HTMLInputElement;
            await fireEvent.input(apiKeyInput, { target: { value: 'test-api-key' } });

            expect(apiKeyInput.value).toBe('test-api-key');
        });
    });

    describe('Endpoint-only configuration (claim flow)', () => {
        it('should save configuration with only endpoint', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const endpointInput = screen.getByLabelText(/API Endpoint:/);
            await fireEvent.input(endpointInput, { target: { value: 'https://api.example.com' } });

            const saveButton = screen.getByText('Save');
            await fireEvent.click(saveButton);

            await waitFor(() => {
                expect(get(apiEndpoint)).toBe('https://api.example.com');
                expect(get(apiKey)).toBeNull();
            });
        });

        it('should close modal after saving endpoint only', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const endpointInput = screen.getByLabelText(/API Endpoint:/);
            await fireEvent.input(endpointInput, { target: { value: 'http://localhost:8080' } });

            const saveButton = screen.getByText('Save');
            await fireEvent.click(saveButton);

            await waitFor(() => {
                expect(screen.queryByText('API Configuration')).not.toBeInTheDocument();
            });
        });

        it('should dispatch credentials-updated event after saving endpoint only', async () => {
            const eventListener = vi.fn();
            window.addEventListener('credentials-updated', eventListener);

            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const endpointInput = screen.getByLabelText(/API Endpoint:/);
            await fireEvent.input(endpointInput, { target: { value: 'https://api.example.com' } });

            const saveButton = screen.getByText('Save');
            await fireEvent.click(saveButton);

            await waitFor(() => {
                expect(eventListener).toHaveBeenCalledTimes(1);
            });

            window.removeEventListener('credentials-updated', eventListener);
        });
    });

    describe('Full configuration (endpoint + API key)', () => {
        it('should save both endpoint and API key', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const endpointInput = screen.getByLabelText(/API Endpoint:/);
            await fireEvent.input(endpointInput, { target: { value: 'https://api.example.com' } });

            const apiKeyInput = screen.getByLabelText(/API Key \(optional\):/);
            await fireEvent.input(apiKeyInput, { target: { value: 'test-key-123' } });

            const saveButton = screen.getByText('Save');
            await fireEvent.click(saveButton);

            await waitFor(() => {
                expect(get(apiEndpoint)).toBe('https://api.example.com');
                expect(get(apiKey)).toBe('test-key-123');
            });
        });

        it('should close modal after saving full configuration', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const endpointInput = screen.getByLabelText(/API Endpoint:/);
            await fireEvent.input(endpointInput, { target: { value: 'http://localhost:8080' } });

            const apiKeyInput = screen.getByLabelText(/API Key \(optional\):/);
            await fireEvent.input(apiKeyInput, { target: { value: 'my-api-key' } });

            const saveButton = screen.getByText('Save');
            await fireEvent.click(saveButton);

            await waitFor(() => {
                expect(screen.queryByText('API Configuration')).not.toBeInTheDocument();
            });
        });
    });

    describe('Validation', () => {
        it('should show error when endpoint is empty', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const saveButton = screen.getByText('Save');
            await fireEvent.click(saveButton);

            await waitFor(() => {
                expect(screen.getByText('Please enter an endpoint URL')).toBeInTheDocument();
            });
        });

        it('should show error for invalid URL format', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const endpointInput = screen.getByLabelText(/API Endpoint:/);
            await fireEvent.input(endpointInput, { target: { value: 'not-a-valid-url' } });

            const saveButton = screen.getByText('Save');
            await fireEvent.click(saveButton);

            await waitFor(() => {
                expect(screen.getByText('Invalid URL format')).toBeInTheDocument();
            });
        });

        it('should accept valid HTTP URL', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const endpointInput = screen.getByLabelText(/API Endpoint:/);
            await fireEvent.input(endpointInput, { target: { value: 'http://localhost:8080' } });

            const saveButton = screen.getByText('Save');
            await fireEvent.click(saveButton);

            await waitFor(() => {
                expect(get(apiEndpoint)).toBe('http://localhost:8080');
            });
        });

        it('should accept valid HTTPS URL', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const endpointInput = screen.getByLabelText(/API Endpoint:/);
            await fireEvent.input(endpointInput, { target: { value: 'https://api.example.com' } });

            const saveButton = screen.getByText('Save');
            await fireEvent.click(saveButton);

            await waitFor(() => {
                expect(get(apiEndpoint)).toBe('https://api.example.com');
            });
        });

        it('should accept URL with port', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const endpointInput = screen.getByLabelText(/API Endpoint:/);
            await fireEvent.input(endpointInput, { target: { value: 'http://192.168.1.1:3000' } });

            const saveButton = screen.getByText('Save');
            await fireEvent.click(saveButton);

            await waitFor(() => {
                expect(get(apiEndpoint)).toBe('http://192.168.1.1:3000');
            });
        });

        it('should accept URL with path', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const endpointInput = screen.getByLabelText(/API Endpoint:/);
            await fireEvent.input(endpointInput, { target: { value: 'https://api.example.com/runvoy/v1' } });

            const saveButton = screen.getByText('Save');
            await fireEvent.click(saveButton);

            await waitFor(() => {
                expect(get(apiEndpoint)).toBe('https://api.example.com/runvoy/v1');
            });
        });

        it('should trim whitespace from endpoint', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const endpointInput = screen.getByLabelText(/API Endpoint:/);
            await fireEvent.input(endpointInput, { target: { value: '  https://api.example.com  ' } });

            const saveButton = screen.getByText('Save');
            await fireEvent.click(saveButton);

            await waitFor(() => {
                expect(get(apiEndpoint)).toBe('https://api.example.com');
            });
        });

        it('should not save masked placeholder as API key', async () => {
            // Set up initial configuration
            apiEndpoint.set('https://api.example.com');
            apiKey.set('original-key');

            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            // Change only the endpoint, leave the masked API key
            const endpointInput = screen.getByLabelText(/API Endpoint:/);
            await fireEvent.input(endpointInput, { target: { value: '' } });
            await fireEvent.input(endpointInput, { target: { value: 'https://new-api.example.com' } });

            const saveButton = screen.getByText('Save');
            await fireEvent.click(saveButton);

            await waitFor(() => {
                expect(get(apiEndpoint)).toBe('https://new-api.example.com');
                expect(get(apiKey)).toBe('original-key'); // Should remain unchanged
            });
        });
    });

    describe('Current configuration display', () => {
        it('should show "Not configured" when no endpoint is set', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const currentConfig = screen.getByText('Current Configuration');
            expect(currentConfig).toBeInTheDocument();

            expect(screen.getAllByText('Not configured')).toHaveLength(2); // Endpoint and API Key
        });

        it('should display current endpoint', async () => {
            apiEndpoint.set('https://api.example.com');

            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            expect(screen.getByText('https://api.example.com')).toBeInTheDocument();
        });

        it('should display masked API key when set', async () => {
            apiKey.set('secret-key-123');

            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            // The masked placeholder should be shown
            const apiKeyInput = screen.getByLabelText(/API Key \(optional\):/) as HTMLInputElement;
            expect(apiKeyInput.placeholder).toContain('••••••••');
        });

        it('should populate endpoint field with current value when modal opens', async () => {
            apiEndpoint.set('https://api.example.com');

            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const endpointInput = screen.getByLabelText(/API Endpoint:/) as HTMLInputElement;
            expect(endpointInput.value).toBe('https://api.example.com');
        });
    });

    describe('Error message handling', () => {
        it('should clear error when modal is opened again', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            // Trigger error
            const saveButton = screen.getByText('Save');
            await fireEvent.click(saveButton);

            await waitFor(() => {
                expect(screen.getByText('Please enter an endpoint URL')).toBeInTheDocument();
            });

            // Close modal
            const cancelButton = screen.getByText('Cancel');
            await fireEvent.click(cancelButton);

            // Reopen modal
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            // Error should be cleared
            expect(screen.queryByText('Please enter an endpoint URL')).not.toBeInTheDocument();
        });

        it('should clear error when modal is closed', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            // Trigger error
            const saveButton = screen.getByText('Save');
            await fireEvent.click(saveButton);

            await waitFor(() => {
                expect(screen.getByText('Please enter an endpoint URL')).toBeInTheDocument();
            });

            // Close modal by clicking cancel
            const cancelButton = screen.getByText('Cancel');
            await fireEvent.click(cancelButton);

            // Reopen and check error is gone
            await fireEvent.click(screen.getByText('⚙️ Configure API'));
            expect(screen.queryByText('Please enter an endpoint URL')).not.toBeInTheDocument();
        });
    });

    describe('Update existing configuration', () => {
        it('should allow updating endpoint while keeping API key', async () => {
            apiEndpoint.set('https://old-api.example.com');
            apiKey.set('my-api-key');

            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const endpointInput = screen.getByLabelText(/API Endpoint:/);
            await fireEvent.input(endpointInput, { target: { value: '' } });
            await fireEvent.input(endpointInput, { target: { value: 'https://new-api.example.com' } });

            const apiKeyInput = screen.getByLabelText(/API Key \(optional\):/);
            await fireEvent.input(apiKeyInput, { target: { value: 'new-key-456' } });

            const saveButton = screen.getByText('Save');
            await fireEvent.click(saveButton);

            await waitFor(() => {
                expect(get(apiEndpoint)).toBe('https://new-api.example.com');
                expect(get(apiKey)).toBe('new-key-456');
            });
        });

        it('should allow adding API key to endpoint-only config', async () => {
            apiEndpoint.set('https://api.example.com');
            apiKey.set(null);

            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const apiKeyInput = screen.getByLabelText(/API Key \(optional\):/);
            await fireEvent.input(apiKeyInput, { target: { value: 'newly-claimed-key' } });

            const saveButton = screen.getByText('Save');
            await fireEvent.click(saveButton);

            await waitFor(() => {
                expect(get(apiEndpoint)).toBe('https://api.example.com');
                expect(get(apiKey)).toBe('newly-claimed-key');
            });
        });
    });

    describe('Accessibility', () => {
        it('should have proper labels for form inputs', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            expect(screen.getByLabelText(/API Endpoint:/)).toBeInTheDocument();
            expect(screen.getByLabelText(/API Key \(optional\):/)).toBeInTheDocument();
        });

        it('should show error with role="alert"', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const saveButton = screen.getByText('Save');
            await fireEvent.click(saveButton);

            await waitFor(() => {
                const errorMessage = screen.getByText('Please enter an endpoint URL');
                expect(errorMessage.closest('.error-message')).toHaveAttribute('role', 'alert');
            });
        });
    });

    describe('Form submission', () => {
        it('should submit form on Enter key in endpoint field', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const endpointInput = screen.getByLabelText(/API Endpoint:/);
            await fireEvent.input(endpointInput, { target: { value: 'https://api.example.com' } });
            await fireEvent.submit(endpointInput.closest('form')!);

            await waitFor(() => {
                expect(get(apiEndpoint)).toBe('https://api.example.com');
            });
        });

        it('should submit form via Save button', async () => {
            render(ConnectionManager);
            await fireEvent.click(screen.getByText('⚙️ Configure API'));

            const endpointInput = screen.getByLabelText(/API Endpoint:/);
            await fireEvent.input(endpointInput, { target: { value: 'https://api.example.com' } });

            const saveButton = screen.getByText('Save');
            await fireEvent.click(saveButton);

            await waitFor(() => {
                expect(get(apiEndpoint)).toBe('https://api.example.com');
            });
        });
    });
});
