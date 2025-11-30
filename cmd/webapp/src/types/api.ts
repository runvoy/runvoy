/**
 * API request and response type definitions for the runvoy backend
 */
import type { ApiLogEvent } from './logs';
import type { ExecutionStatusValue } from './status';

export interface RunCommandPayload {
    command: string;
    image?: string;
    timeout?: number;
    env?: Record<string, string>;
    git_repo?: string;
    git_ref?: string;
    git_path?: string;
}

export interface RunCommandResponse {
    execution_id: string;
    status: ExecutionStatusValue;
    websocket_url?: string;
    image_id?: string;
    log_url?: string;
}

export interface LogsResponse {
    events: ApiLogEvent[] | null;
    websocket_url?: string;
    status: ExecutionStatusValue;
}

export interface ExecutionStatusResponse {
    execution_id: string;
    status: ExecutionStatusValue;
    started_at?: string;
    completed_at?: string;
    exit_code?: number;
    error?: string;
}

export interface KillExecutionResponse {
    execution_id: string;
    status: string;
}

export type ListExecutionsResponse = Execution[];

export interface Execution {
    execution_id: string;
    status: ExecutionStatusValue;
    started_at: string;
    completed_at?: string;
    exit_code?: number;
}

export interface ApiError extends Error {
    status?: number;
    details?: {
        error?: string;
        details?: string;
        code?: string;
    };
}

export interface ClaimAPIKeyResponse {
    api_key: string;
    user_email: string;
    message?: string;
}

export interface HealthResponse {
    status: string;
    version: string;
    provider: string;
    region?: string;
}

export interface APIClientConfig {
    endpoint: string;
    apiKey: string;
}
