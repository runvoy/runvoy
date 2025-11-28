import { error } from '@sveltejs/kit';
import APIClient from './api';

export type ApiConfiguration = {
    endpoint?: string | null;
    apiKey?: string | null;
};

export type ValidationOptions = {
    requireApiKey?: boolean;
    throwOnInvalid?: boolean;
};

export function normalizeEndpoint(endpoint?: string | null): string | null {
    if (!endpoint) {
        return null;
    }

    try {
        const url = new URL(endpoint);
        const normalized = url.toString();
        return normalized.endsWith('/') ? normalized.slice(0, -1) : normalized;
    } catch {
        return null;
    }
}

export function validateApiConfiguration(
    config: ApiConfiguration,
    options: ValidationOptions = {}
): ApiConfiguration | null {
    const { requireApiKey = true } = options;
    const endpoint = normalizeEndpoint(config.endpoint);
    const apiKey = config.apiKey?.trim() || null;

    if (!endpoint) {
        if (options.throwOnInvalid) {
            throw error(500, 'Invalid API endpoint configuration');
        }
        return null;
    }

    if (requireApiKey && !apiKey) {
        if (options.throwOnInvalid) {
            throw error(500, 'API key is required');
        }
        return null;
    }

    // At this point, endpoint is guaranteed to be a string (not null)
    return { endpoint: endpoint as string, apiKey };
}

export function createApiClientFromConfig(
    config: ApiConfiguration,
    fetcher: typeof fetch,
    options: ValidationOptions = {}
): APIClient | null {
    const validated = validateApiConfiguration(config, options);

    if (!validated) {
        if (options.throwOnInvalid) {
            throw error(500, 'API configuration is incomplete');
        }
        return null;
    }

    const apiKey: string = validated.apiKey ?? '';
    // validated.endpoint is guaranteed to be a string after validation (not null/undefined)
    const endpoint: string = validated.endpoint as string;
    return new APIClient(endpoint, apiKey, fetcher);
}
