/**
 * API Client for runvoy backend
 */
export class APIClient {
    constructor(endpoint, apiKey) {
        this.endpoint = endpoint;
        this.apiKey = apiKey;
    }

    /**
     * Execute a command via the runvoy backend
     * @param {Object} payload - Execution request payload
     * @returns {Promise<Object>} Execution response containing execution_id and status
     */
    async runCommand(payload) {
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
            const error = new Error(errorData.details || `HTTP ${response.status}`);
            error.status = response.status;
            error.details = errorData;
            throw error;
        }

        return response.json();
    }

    /**
     * Fetch logs for an execution
     * @param {string} executionId - Execution ID
     * @returns {Promise<Object>} Logs response with events and websocket_url
     */
    async getLogs(executionId) {
        const url = `${this.endpoint}/api/v1/executions/${executionId}/logs`;
        const response = await fetch(url, {
            headers: {
                'X-API-Key': this.apiKey
            }
        });

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            const error = new Error(errorData.details || `HTTP ${response.status}`);
            error.status = response.status;
            error.details = errorData;
            throw error;
        }

        return response.json();
    }

    /**
     * Kill a running execution
     * @param {string} executionId - Execution ID
     * @returns {Promise<Object>} Kill response
     */
    async killExecution(executionId) {
        const url = `${this.endpoint}/api/v1/executions/${executionId}/kill`;
        const response = await fetch(url, {
            method: 'POST',
            headers: {
                'X-API-Key': this.apiKey
            }
        });

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            const error = new Error(errorData.details || `HTTP ${response.status}`);
            error.status = response.status;
            error.details = errorData;
            throw error;
        }

        return response.json();
    }

    /**
     * List all executions
     * @returns {Promise<Array>} Array of executions
     */
    async listExecutions() {
        const url = `${this.endpoint}/api/v1/executions`;
        const response = await fetch(url, {
            headers: {
                'X-API-Key': this.apiKey
            }
        });

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            const error = new Error(errorData.details || `HTTP ${response.status}`);
            error.status = response.status;
            error.details = errorData;
            throw error;
        }

        return response.json();
    }
}

export default APIClient;
