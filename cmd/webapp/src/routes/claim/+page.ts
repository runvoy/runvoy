import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent }) => {
    const parentData = await parent();

    if (!parentData.apiClient) {
        throw error(500, 'API client is required to claim API keys');
    }

    return {
        apiClient: parentData.apiClient,
        isConfigured: Boolean(parentData.apiClient)
    };
};
