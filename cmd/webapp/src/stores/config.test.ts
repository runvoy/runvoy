import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { apiEndpoint, apiKey } from './config';
import { get } from 'svelte/store';

describe('Config Store', () => {
    const localStorageMock = {
        getItem: vi.fn(),
        setItem: vi.fn(),
        removeItem: vi.fn(),
        clear: vi.fn()
    };

    beforeEach(() => {
        // Clear all mocks before each test
        vi.clearAllMocks();
        // Set default localStorage behavior
        localStorageMock.getItem.mockReturnValue(null);
    });

    afterEach(() => {
        // Reset stores after each test
        apiEndpoint.set(null);
        apiKey.set(null);
    });

    describe('apiEndpoint store', () => {
        it('should initialize with null value', () => {
            expect(get(apiEndpoint)).toBeNull();
        });

        it('should accept endpoint string', () => {
            const endpoint = 'http://localhost:8080';
            apiEndpoint.set(endpoint);
            expect(get(apiEndpoint)).toBe(endpoint);
        });

        it('should update endpoint', () => {
            apiEndpoint.set('http://localhost:8080');
            apiEndpoint.set('https://api.example.com');
            expect(get(apiEndpoint)).toBe('https://api.example.com');
        });

        it('should clear endpoint', () => {
            apiEndpoint.set('http://localhost:8080');
            apiEndpoint.set(null);
            expect(get(apiEndpoint)).toBeNull();
        });

        it('should accept various endpoint formats', () => {
            const endpoints = [
                'http://localhost:3000',
                'https://api.example.com',
                'https://api.example.com/runvoy',
                'http://192.168.1.1:8080'
            ];

            endpoints.forEach((endpoint) => {
                apiEndpoint.set(endpoint);
                expect(get(apiEndpoint)).toBe(endpoint);
            });
        });

        it('should work with update function', () => {
            apiEndpoint.set('http://localhost:8080');
            apiEndpoint.update((current) => (current ? `${current}/api` : null));
            expect(get(apiEndpoint)).toBe('http://localhost:8080/api');
        });
    });

    describe('apiKey store', () => {
        it('should initialize with null value', () => {
            expect(get(apiKey)).toBeNull();
        });

        it('should accept API key string', () => {
            const key = 'test-api-key-123';
            apiKey.set(key);
            expect(get(apiKey)).toBe(key);
        });

        it('should update API key', () => {
            apiKey.set('key-1');
            apiKey.set('key-2');
            expect(get(apiKey)).toBe('key-2');
        });

        it('should clear API key', () => {
            apiKey.set('test-key');
            apiKey.set(null);
            expect(get(apiKey)).toBeNull();
        });

        it('should accept various key formats', () => {
            const keys = [
                'simple-key',
                'complex-key-with-dashes',
                'key_with_underscores',
                'key123456',
                'abcdef1234567890'
            ];

            keys.forEach((key) => {
                apiKey.set(key);
                expect(get(apiKey)).toBe(key);
            });
        });

        it('should work with update function', () => {
            apiKey.set('old-key');
            apiKey.update((current) => (current ? `${current}-updated` : 'new-key'));
            expect(get(apiKey)).toBe('old-key-updated');
        });
    });

    describe('store subscriptions', () => {
        it('should notify subscribers of endpoint changes', () => {
            const values: (string | null)[] = [];
            const unsubscribe = apiEndpoint.subscribe((value) => {
                values.push(value);
            });

            apiEndpoint.set('http://localhost:8080');
            apiEndpoint.set('https://example.com');
            apiEndpoint.set(null);

            expect(values).toHaveLength(4); // Initial null + 3 updates
            expect(values[0]).toBeNull();
            expect(values[1]).toBe('http://localhost:8080');
            expect(values[2]).toBe('https://example.com');
            expect(values[3]).toBeNull();

            unsubscribe();
        });

        it('should notify subscribers of apiKey changes', () => {
            const values: (string | null)[] = [];
            const unsubscribe = apiKey.subscribe((value) => {
                values.push(value);
            });

            apiKey.set('key-1');
            apiKey.set('key-2');
            apiKey.set(null);

            expect(values).toHaveLength(4); // Initial null + 3 updates
            expect(values[0]).toBeNull();
            expect(values[1]).toBe('key-1');
            expect(values[2]).toBe('key-2');
            expect(values[3]).toBeNull();

            unsubscribe();
        });

        it('should support multiple subscribers', () => {
            const sub1Values: (string | null)[] = [];
            const sub2Values: (string | null)[] = [];

            const unsub1 = apiEndpoint.subscribe((v) => sub1Values.push(v));
            const unsub2 = apiEndpoint.subscribe((v) => sub2Values.push(v));

            apiEndpoint.set('http://localhost:8080');

            expect(sub1Values).toHaveLength(2);
            expect(sub2Values).toHaveLength(2);
            expect(sub1Values[1]).toBe('http://localhost:8080');
            expect(sub2Values[1]).toBe('http://localhost:8080');

            unsub1();
            unsub2();
        });
    });

    describe('combined config state', () => {
        it('should track complete configuration lifecycle', () => {
            // Initialize empty
            expect(get(apiEndpoint)).toBeNull();
            expect(get(apiKey)).toBeNull();

            // Set endpoint first
            apiEndpoint.set('http://localhost:8080');
            expect(get(apiEndpoint)).toBe('http://localhost:8080');
            expect(get(apiKey)).toBeNull();

            // Set API key
            apiKey.set('test-key-123');
            expect(get(apiEndpoint)).toBe('http://localhost:8080');
            expect(get(apiKey)).toBe('test-key-123');

            // Update endpoint
            apiEndpoint.set('https://api.example.com');
            expect(get(apiEndpoint)).toBe('https://api.example.com');
            expect(get(apiKey)).toBe('test-key-123');

            // Clear all
            apiEndpoint.set(null);
            apiKey.set(null);
            expect(get(apiEndpoint)).toBeNull();
            expect(get(apiKey)).toBeNull();
        });

        it('should allow independent updates', () => {
            // Set both
            apiEndpoint.set('http://localhost:8080');
            apiKey.set('key-123');

            // Update only endpoint
            apiEndpoint.set('https://new.example.com');
            expect(get(apiEndpoint)).toBe('https://new.example.com');
            expect(get(apiKey)).toBe('key-123');

            // Update only key
            apiKey.set('new-key');
            expect(get(apiEndpoint)).toBe('https://new.example.com');
            expect(get(apiKey)).toBe('new-key');
        });
    });
});
