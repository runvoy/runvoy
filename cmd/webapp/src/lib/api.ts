/**
 * API Client for runvoy backend
 */
import type {
    RunCommandPayload,
    RunCommandResponse,
    LogsResponse,
    ExecutionStatusResponse,
    KillExecutionResponse,
    ListExecutionsResponse,
    ClaimAPIKeyResponse,
    ApiError
} from '../types/api';

export class APIClient {
    endpoint: string;
    apiKey: string;

    constructor(endpoint: string, apiKey: string) {
        this.endpoint = endpoint;
        this.apiKey = apiKey;
    }

    /**
     * Execute a command via the runvoy backend
     */
    async runCommand(payload: RunCommandPayload): Promise<RunCommandResponse> {
        const url = `${this.endpoint}/api/v1/run`;
        const response = await fetch(url, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'X-API-Key': this.apiKey
            },
            body: JSON.stringify(payload)
        });

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            const error = new Error(errorData.details || `HTTP ${response.status}`) as ApiError;
            error.status = response.status;
            error.details = errorData;
            throw error;
        }

        return response.json();
    }

    /**
     * Fetch logs for an execution
     */
    async getLogs(executionId: string): Promise<LogsResponse> {
        const url = `${this.endpoint}/api/v1/executions/${executionId}/logs`;
        const response = await fetch(url, {
            headers: {
                'X-API-Key': this.apiKey
            }
        });

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            const error = new Error(errorData.details || `HTTP ${response.status}`) as ApiError;
            error.status = response.status;
            error.details = errorData;
            throw error;
        }

        return response.json();
    }

    /**
     * Get execution status
     */
    async getExecutionStatus(executionId: string): Promise<ExecutionStatusResponse> {
        const url = `${this.endpoint}/api/v1/executions/${executionId}/status`;
        const response = await fetch(url, {
            headers: {
                'X-API-Key': this.apiKey
            }
        });

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            const error = new Error(errorData.details || `HTTP ${response.status}`) as ApiError;
            error.status = response.status;
            error.details = errorData;
            throw error;
        }

        return response.json();
    }

    /**
     * Kill a running execution
     */
    async killExecution(executionId: string): Promise<KillExecutionResponse> {
        const url = `${this.endpoint}/api/v1/executions/${executionId}`;
        const response = await fetch(url, {
            method: 'DELETE',
            headers: {
                'X-API-Key': this.apiKey
            }
        });

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            const error = new Error(errorData.details || `HTTP ${response.status}`) as ApiError;
            error.status = response.status;
            error.details = errorData;
            throw error;
        }

        return response.json();
    }

    /**
     * List all executions
     */
    async listExecutions(): Promise<ListExecutionsResponse> {
        const url = `${this.endpoint}/api/v1/executions`;
        const response = await fetch(url, {
            headers: {
                'X-API-Key': this.apiKey
            }
        });

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            const error = new Error(errorData.details || `HTTP ${response.status}`) as ApiError;
            error.status = response.status;
            error.details = errorData;
            throw error;
        }

        return response.json();
    }

    /**
     * Claim an API key with an invitation token
     */
    async claimAPIKey(token: string): Promise<ClaimAPIKeyResponse> {
        const url = `${this.endpoint}/api/v1/claim/${token}`;
        const response = await fetch(url);

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            const error = new Error(errorData.details || `HTTP ${response.status}`) as ApiError;
            error.status = response.status;
            error.details = errorData;
            throw error;
        }

        return response.json();
    }
}

export default APIClient;
