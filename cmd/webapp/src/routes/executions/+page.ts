import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent }) => {
    const parentData = await parent();

    if (!parentData.apiClient) {
        throw error(500, 'API client is required to list executions');
    }

    return {
        apiClient: parentData.apiClient
    };
};
