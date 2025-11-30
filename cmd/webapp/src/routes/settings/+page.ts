import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent }) => {
    const parentData = await parent();

    return {
        apiClient: parentData.apiClient,
        isConfigured: Boolean(parentData.apiClient)
    };
};
