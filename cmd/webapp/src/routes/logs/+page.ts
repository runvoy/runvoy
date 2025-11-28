import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import { buildApiClient } from '../loaders/apiClient';

export const load: PageLoad = async ({ fetch, parent }) => {
    const parentData = await parent();
    const apiClient = buildApiClient(parentData, fetch, { throwOnInvalid: true });

    if (!apiClient) {
        throw error(500, 'API client is required to view logs');
    }

    return {
        apiClient
    };
};
