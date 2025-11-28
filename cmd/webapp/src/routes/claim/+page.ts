import { error, type PageLoad } from '@sveltejs/kit';
import { buildApiClient } from '../loaders/apiClient';

export const load: PageLoad = async ({ fetch, parent }) => {
    const parentData = await parent();
    const apiClient = buildApiClient(parentData, fetch, {
        requireApiKey: false,
        throwOnInvalid: true
    });

    if (!apiClient) {
        throw error(500, 'API client is required to claim API keys');
    }

    return {
        apiClient,
        isConfigured: Boolean(apiClient)
    };
};
