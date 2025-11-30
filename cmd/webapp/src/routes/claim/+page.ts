import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent }) => {
    const parentData = await parent();

    // Allow null apiClient - component will handle it (may redirect client-side)
    return {
        apiClient: parentData.apiClient,
        isConfigured: Boolean(parentData.apiClient)
    };
};
