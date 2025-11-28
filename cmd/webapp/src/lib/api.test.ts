import { describe, it, expect, beforeEach, vi } from 'vitest';
import APIClient from './api';
import type { RunCommandPayload, RunCommandResponse } from '../types/api';

describe('APIClient', () => {
    let client: APIClient;
    const testEndpoint = 'http://localhost:8080';
    const testApiKey = 'test-api-key-123';

    beforeEach(() => {
        client = new APIClient(testEndpoint, testApiKey);
        vi.clearAllMocks();
    });

    describe('constructor', () => {
        it('should initialize with endpoint and apiKey', () => {
            expect(client.endpoint).toBe(testEndpoint);
            expect(client.apiKey).toBe(testApiKey);
        });
    });

    describe('runCommand', () => {
        it('should make a POST request to /api/v1/run with correct headers', async () => {
            const payload: RunCommandPayload = {
                command: 'echo "hello"'
            };
            const mockResponse: RunCommandResponse = {
                execution_id: 'exec-123',
                status: 'RUNNING'
            };

            vi.mocked(globalThis.fetch).mockResolvedValueOnce({
                ok: true,
                json: vi.fn().mockResolvedValueOnce(mockResponse)
            } as any);

            const result = await client.runCommand(payload);

            expect(globalThis.fetch).toHaveBeenCalledWith(`${testEndpoint}/api/v1/run`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-API-Key': testApiKey
                },
                body: JSON.stringify(payload)
            });
            expect(result).toEqual(mockResponse);
        });

        it('should throw error with status when request fails', async () => {
            const payload: RunCommandPayload = {
                command: 'echo "hello"'
            };
            const errorResponse = {
                details: 'Unauthorized',
                code: 'UNAUTHORIZED'
            };

            vi.mocked(globalThis.fetch).mockResolvedValueOnce({
                ok: false,
                status: 401,
                json: vi.fn().mockResolvedValueOnce(errorResponse)
            } as any);

            await expect(client.runCommand(payload)).rejects.toThrow();
        });

        it('should handle invalid JSON response on error', async () => {
            const payload: RunCommandPayload = {
                command: 'echo "hello"'
            };

            vi.mocked(globalThis.fetch).mockResolvedValueOnce({
                ok: false,
                status: 500,
                json: vi.fn().mockRejectedValueOnce(new Error('Invalid JSON'))
            } as any);

            await expect(client.runCommand(payload)).rejects.toThrow();
        });

        it('should include optional payload fields', async () => {
            const payload: RunCommandPayload = {
                command: 'echo "hello"',
                image: 'custom-image:latest',
                timeout: 300,
                env: { DEBUG: 'true' },
                git_repo: 'https://github.com/user/repo',
                git_ref: 'main',
                git_path: 'subdir'
            };

            const mockResponse: RunCommandResponse = {
                execution_id: 'exec-456',
                status: 'RUNNING'
            };

            vi.mocked(globalThis.fetch).mockResolvedValueOnce({
                ok: true,
                json: vi.fn().mockResolvedValueOnce(mockResponse)
            } as any);

            await client.runCommand(payload);

            expect(globalThis.fetch).toHaveBeenCalledWith(
                expect.any(String),
                expect.objectContaining({
                    body: JSON.stringify(payload)
                })
            );
        });
    });

    describe('getLogs', () => {
        it('should fetch logs for an execution', async () => {
            const executionId = 'exec-123';
            const mockResponse = {
                events: [{ message: 'Starting...', timestamp: 1234567890, line: 0 }],
                websocket_url: 'wss://localhost:8080/api/v1/executions/exec-123/logs'
            };

            vi.mocked(globalThis.fetch).mockResolvedValueOnce({
                ok: true,
                json: vi.fn().mockResolvedValueOnce(mockResponse)
            } as any);

            const result = await client.getLogs(executionId);

            expect(globalThis.fetch).toHaveBeenCalledWith(
                `${testEndpoint}/api/v1/executions/${executionId}/logs`,
                {
                    headers: {
                        'X-API-Key': testApiKey
                    }
                }
            );
            expect(result).toEqual(mockResponse);
        });

        it('should throw error when getLogs fails', async () => {
            const executionId = 'exec-123';

            vi.mocked(globalThis.fetch).mockResolvedValueOnce({
                ok: false,
                status: 404,
                json: vi.fn().mockResolvedValueOnce({ details: 'Not found' })
            } as any);

            await expect(client.getLogs(executionId)).rejects.toThrow();
        });
    });

    describe('getExecutionStatus', () => {
        it('should fetch execution status', async () => {
            const executionId = 'exec-123';
            const mockResponse = {
                execution_id: executionId,
                status: 'SUCCEEDED',
                started_at: '2025-01-01T00:00:00Z',
                completed_at: '2025-01-01T00:05:00Z',
                exit_code: 0
            };

            vi.mocked(globalThis.fetch).mockResolvedValueOnce({
                ok: true,
                json: vi.fn().mockResolvedValueOnce(mockResponse)
            } as any);

            const result = await client.getExecutionStatus(executionId);

            expect(globalThis.fetch).toHaveBeenCalledWith(
                `${testEndpoint}/api/v1/executions/${executionId}/status`,
                {
                    headers: {
                        'X-API-Key': testApiKey
                    }
                }
            );
            expect(result).toEqual(mockResponse);
        });

        it('should throw error when getExecutionStatus fails', async () => {
            const executionId = 'exec-123';

            vi.mocked(globalThis.fetch).mockResolvedValueOnce({
                ok: false,
                status: 500,
                json: vi.fn().mockResolvedValueOnce({ details: 'Server error' })
            } as any);

            await expect(client.getExecutionStatus(executionId)).rejects.toThrow();
        });
    });

    describe('killExecution', () => {
        it('should send DELETE request to kill execution', async () => {
            const executionId = 'exec-123';
            const mockResponse = {
                execution_id: executionId,
                status: 'KILLED'
            };

            vi.mocked(globalThis.fetch).mockResolvedValueOnce({
                ok: true,
                json: vi.fn().mockResolvedValueOnce(mockResponse)
            } as any);

            const result = await client.killExecution(executionId);

            expect(globalThis.fetch).toHaveBeenCalledWith(
                `${testEndpoint}/api/v1/executions/${executionId}`,
                {
                    method: 'DELETE',
                    headers: {
                        'X-API-Key': testApiKey
                    }
                }
            );
            expect(result).toEqual(mockResponse);
        });

        it('should throw error when killExecution fails', async () => {
            const executionId = 'exec-123';

            vi.mocked(globalThis.fetch).mockResolvedValueOnce({
                ok: false,
                status: 404,
                json: vi.fn().mockResolvedValueOnce({ details: 'Not found' })
            } as any);

            await expect(client.killExecution(executionId)).rejects.toThrow();
        });
    });

    describe('listExecutions', () => {
        it('should fetch list of executions', async () => {
            const mockResponse = [
                {
                    execution_id: 'exec-1',
                    status: 'SUCCEEDED',
                    started_at: '2025-01-01T00:00:00Z',
                    exit_code: 0
                },
                {
                    execution_id: 'exec-2',
                    status: 'FAILED',
                    started_at: '2025-01-01T00:05:00Z',
                    exit_code: 1
                }
            ];

            vi.mocked(globalThis.fetch).mockResolvedValueOnce({
                ok: true,
                json: vi.fn().mockResolvedValueOnce(mockResponse)
            } as any);

            const result = await client.listExecutions();

            expect(globalThis.fetch).toHaveBeenCalledWith(`${testEndpoint}/api/v1/executions`, {
                headers: {
                    'X-API-Key': testApiKey
                }
            });
            expect(result).toEqual(mockResponse);
            expect(result).toHaveLength(2);
        });

        it('should throw error when listExecutions fails', async () => {
            vi.mocked(globalThis.fetch).mockResolvedValueOnce({
                ok: false,
                status: 500,
                json: vi.fn().mockResolvedValueOnce({ details: 'Server error' })
            } as any);

            await expect(client.listExecutions()).rejects.toThrow();
        });
    });

    describe('claimAPIKey', () => {
        it('should claim API key with token', async () => {
            const token = 'claim-token-xyz';
            const mockResponse = {
                api_key: 'new-api-key-123',
                user_email: 'user@example.com',
                message: 'API key claimed successfully'
            };

            vi.mocked(globalThis.fetch).mockResolvedValueOnce({
                ok: true,
                json: vi.fn().mockResolvedValueOnce(mockResponse)
            } as any);

            const result = await client.claimAPIKey(token);

            expect(globalThis.fetch).toHaveBeenCalledWith(`${testEndpoint}/api/v1/claim/${token}`);
            expect(result).toEqual(mockResponse);
        });

        it('should throw error when claimAPIKey fails', async () => {
            const token = 'invalid-token';

            vi.mocked(globalThis.fetch).mockResolvedValueOnce({
                ok: false,
                status: 400,
                json: vi.fn().mockResolvedValueOnce({ details: 'Invalid token' })
            } as any);

            await expect(client.claimAPIKey(token)).rejects.toThrow();
        });

        it('should handle JSON parse error on claim failure', async () => {
            const token = 'invalid-token';

            vi.mocked(globalThis.fetch).mockResolvedValueOnce({
                ok: false,
                status: 500,
                json: vi.fn().mockRejectedValueOnce(new Error('Invalid JSON'))
            } as any);

            await expect(client.claimAPIKey(token)).rejects.toThrow();
        });
    });

    describe('error handling', () => {
        it('should preserve error status in thrown error', async () => {
            const payload: RunCommandPayload = {
                command: 'echo "hello"'
            };

            vi.mocked(globalThis.fetch).mockResolvedValueOnce({
                ok: false,
                status: 403,
                json: vi.fn().mockResolvedValueOnce({ details: 'Forbidden' })
            } as any);

            try {
                await client.runCommand(payload);
                expect.fail('Should have thrown');
            } catch (error: any) {
                expect(error.status).toBe(403);
                expect(error.details).toEqual({ details: 'Forbidden' });
            }
        });

        it('should use HTTP status as fallback message', async () => {
            const payload: RunCommandPayload = {
                command: 'echo "hello"'
            };

            vi.mocked(globalThis.fetch).mockResolvedValueOnce({
                ok: false,
                status: 503,
                json: vi.fn().mockResolvedValueOnce({})
            } as any);

            try {
                await client.runCommand(payload);
                expect.fail('Should have thrown');
            } catch (error: any) {
                expect(error.message).toContain('503');
            }
        });
    });
});
