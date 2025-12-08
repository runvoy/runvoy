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
    HealthResponse,
    ApiError
} from '../types/api';

export class APIClient {
    endpoint: string;
    apiKey: string;
    private fetchFn: typeof fetch;

    constructor(endpoint: string, apiKey: string, fetchFn: typeof fetch = fetch) {
        this.endpoint = endpoint;
        this.apiKey = apiKey;
        this.fetchFn = fetchFn;
    }

    /**
     * Safely join URL paths, handling trailing/leading slashes
     */
    private joinUrl(...parts: string[]): string {
        return parts
            .map((part, index) => {
                // Remove leading slashes from all parts except the first (base URL)
                if (index > 0) {
                    part = part.replace(/^\/+/, '');
                }
                // Remove trailing slashes from all parts
                part = part.replace(/\/+$/, '');
                return part;
            })
            .filter((part) => part.length > 0)
            .join('/');
    }

    /**
     * Execute a command via the runvoy backend
     */
    async runCommand(payload: RunCommandPayload): Promise<RunCommandResponse> {
        const url = this.joinUrl(this.endpoint, 'api/v1/run');
        const response = await this.fetchFn(url, {
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

        const data = (await response.json()) as ExecutionStatusResponse;
        if (!data.command || !data.image_id) {
            throw new Error('Invalid API response: missing command or image_id');
        }
        return data;
    }

    /**
     * Fetch logs for an execution
     */
    async getLogs(executionId: string): Promise<LogsResponse> {
        const url = this.joinUrl(this.endpoint, 'api/v1/executions', executionId, 'logs');
        const response = await this.fetchFn(url, {
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
        const url = this.joinUrl(this.endpoint, 'api/v1/executions', executionId, 'status');
        const response = await this.fetchFn(url, {
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
        const url = this.joinUrl(this.endpoint, 'api/v1/executions', executionId);
        const response = await this.fetchFn(url, {
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
        const url = this.joinUrl(this.endpoint, 'api/v1/executions');
        const response = await this.fetchFn(url, {
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
        const url = this.joinUrl(this.endpoint, 'api/v1/claim', token);
        const response = await this.fetchFn(url);

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
     * Get backend health status and version
     */
    async getHealth(): Promise<HealthResponse> {
        const url = this.joinUrl(this.endpoint, 'api/v1/health');
        const response = await this.fetchFn(url);

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
