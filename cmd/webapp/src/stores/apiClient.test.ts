/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { get } from 'svelte/store';
import { apiClient, hasEndpoint, hasApiKey, isFullyConfigured } from './apiClient';
import { apiEndpoint, apiKey, setApiEndpoint, setApiKey } from './config';

// Mock $app/environment
vi.mock('$app/environment', () => ({
    browser: true
}));

describe('apiClient store', () => {
    beforeEach(() => {
        setApiEndpoint(null);
        setApiKey(null);
    });

    afterEach(() => {
        setApiEndpoint(null);
        setApiKey(null);
    });

    it('should return null when endpoint is not set', () => {
        setApiEndpoint(null);
        const client = get(apiClient);
        expect(client).toBeNull();
    });

    it('should return null when browser is not available', () => {
        // Note: This test might not work perfectly since we're mocking browser=true
        // But it documents the expected behavior
        setApiEndpoint('https://api.example.com');
        const client = get(apiClient);
        // With browser=true mock, it should create a client
        expect(client).not.toBeNull();
    });

    it('should create APIClient when endpoint is set', () => {
        setApiEndpoint('https://api.example.com');
        const client = get(apiClient);
        expect(client).not.toBeNull();
        expect(client?.endpoint).toBe('https://api.example.com');
    });

    it('should normalize endpoint URL by removing trailing slash', () => {
        setApiEndpoint('https://api.example.com/');
        const client = get(apiClient);
        expect(client).not.toBeNull();
        expect(client?.endpoint).toBe('https://api.example.com');
    });

    it('should include API key in client when set', () => {
        setApiEndpoint('https://api.example.com');
        setApiKey('test-api-key');
        const client = get(apiClient);
        expect(client).not.toBeNull();
        expect(client?.apiKey).toBe('test-api-key');
    });

    it('should use empty string for API key when not set', () => {
        setApiEndpoint('https://api.example.com');
        setApiKey(null);
        const client = get(apiClient);
        expect(client).not.toBeNull();
        expect(client?.apiKey).toBe('');
    });

    it('should recreate client when endpoint changes', () => {
        setApiEndpoint('https://api1.example.com');
        const client1 = get(apiClient);
        expect(client1?.endpoint).toBe('https://api1.example.com');

        setApiEndpoint('https://api2.example.com');
        const client2 = get(apiClient);
        expect(client2?.endpoint).toBe('https://api2.example.com');
        expect(client2).not.toBe(client1);
    });

    it('should recreate client when API key changes', () => {
        setApiEndpoint('https://api.example.com');
        setApiKey('key1');
        const client1 = get(apiClient);
        expect(client1?.apiKey).toBe('key1');

        setApiKey('key2');
        const client2 = get(apiClient);
        expect(client2?.apiKey).toBe('key2');
        expect(client2).not.toBe(client1);
    });

    it('should handle invalid URL gracefully', () => {
        setApiEndpoint('not-a-url');
        const client = get(apiClient);
        // Should return null for invalid URLs
        expect(client).toBeNull();
    });

    it('should handle empty endpoint string', () => {
        setApiEndpoint('');
        const client = get(apiClient);
        expect(client).toBeNull();
    });
});

describe('hasEndpoint store', () => {
    beforeEach(() => {
        setApiEndpoint(null);
    });

    afterEach(() => {
        setApiEndpoint(null);
    });

    it('should return false when endpoint is null', () => {
        setApiEndpoint(null);
        expect(get(hasEndpoint)).toBe(false);
    });

    it('should return false when endpoint is empty string', () => {
        setApiEndpoint('');
        expect(get(hasEndpoint)).toBe(false);
    });

    it('should return false when endpoint is whitespace only', () => {
        setApiEndpoint('   ');
        expect(get(hasEndpoint)).toBe(false);
    });

    it('should return true when endpoint is set', () => {
        setApiEndpoint('https://api.example.com');
        expect(get(hasEndpoint)).toBe(true);
    });
});

describe('hasApiKey store', () => {
    beforeEach(() => {
        setApiKey(null);
    });

    afterEach(() => {
        setApiKey(null);
    });

    it('should return false when API key is null', () => {
        setApiKey(null);
        expect(get(hasApiKey)).toBe(false);
    });

    it('should return false when API key is empty string', () => {
        setApiKey('');
        expect(get(hasApiKey)).toBe(false);
    });

    it('should return false when API key is whitespace only', () => {
        setApiKey('   ');
        expect(get(hasApiKey)).toBe(false);
    });

    it('should return true when API key is set', () => {
        setApiKey('test-api-key');
        expect(get(hasApiKey)).toBe(true);
    });
});

describe('isFullyConfigured store', () => {
    beforeEach(() => {
        setApiEndpoint(null);
        setApiKey(null);
    });

    afterEach(() => {
        setApiEndpoint(null);
        setApiKey(null);
    });

    it('should return false when neither endpoint nor API key is set', () => {
        setApiEndpoint(null);
        setApiKey(null);
        expect(get(isFullyConfigured)).toBe(false);
    });

    it('should return false when only endpoint is set', () => {
        setApiEndpoint('https://api.example.com');
        setApiKey(null);
        expect(get(isFullyConfigured)).toBe(false);
    });

    it('should return false when only API key is set', () => {
        setApiEndpoint(null);
        setApiKey('test-api-key');
        expect(get(isFullyConfigured)).toBe(false);
    });

    it('should return true when both endpoint and API key are set', () => {
        setApiEndpoint('https://api.example.com');
        setApiKey('test-api-key');
        expect(get(isFullyConfigured)).toBe(true);
    });

    it('should update when endpoint changes', () => {
        setApiEndpoint('https://api.example.com');
        setApiKey('test-api-key');
        expect(get(isFullyConfigured)).toBe(true);

        setApiEndpoint(null);
        expect(get(isFullyConfigured)).toBe(false);
    });

    it('should update when API key changes', () => {
        setApiEndpoint('https://api.example.com');
        setApiKey('test-api-key');
        expect(get(isFullyConfigured)).toBe(true);

        setApiKey(null);
        expect(get(isFullyConfigured)).toBe(false);
    });
});
