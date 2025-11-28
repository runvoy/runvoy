import type { PageLoad } from './$types';
import { buildApiClient } from '../loaders/apiClient';

export const load: PageLoad = async ({ fetch, parent }) => {
    const parentData = await parent();
    const apiClient = buildApiClient(parentData, fetch, {
        requireApiKey: false,
        throwOnInvalid: false
    });

    return {
        apiClient,
        isConfigured: Boolean(apiClient)
    };
};
