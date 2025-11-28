import type { ApiConfiguration, ValidationOptions } from '../../lib/apiConfig';
import { createApiClientFromConfig } from '../../lib/apiConfig';

export type ParentConfig = Pick<ApiConfiguration, 'endpoint' | 'apiKey'>;

export function buildApiClient(
    parentData: ParentConfig,
    fetcher: typeof fetch,
    options: ValidationOptions = {}
) {
    return createApiClientFromConfig(
        {
            endpoint: parentData.endpoint,
            apiKey: parentData.apiKey
        },
        fetcher,
        options
    );
}
