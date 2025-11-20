/**
 * API request and response type definitions for the runvoy backend
 */

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
    status: string;
}

export interface LogsResponse {
    events: LogEvent[];
    websocket_url: string;
}

export interface LogEvent {
    message: string;
    timestamp: number;
    line?: number;
}

export interface ExecutionStatusResponse {
    execution_id: string;
    status: string;
    started_at?: string;
    completed_at?: string;
    exit_code?: number;
    error?: string;
}

export interface KillExecutionResponse {
    execution_id: string;
    status: string;
}

export interface ListExecutionsResponse {
    executions: Execution[];
}

export interface Execution {
    execution_id: string;
    status: string;
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

export interface APIClientConfig {
    endpoint: string;
    apiKey: string;
}
