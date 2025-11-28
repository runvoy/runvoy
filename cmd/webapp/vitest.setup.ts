import '@testing-library/jest-dom';
import { expect, afterEach, vi } from 'vitest';
import { cleanup } from '@testing-library/svelte';

// Cleanup after each test
afterEach(() => {
	cleanup();
});

// Mock fetch globally
globalThis.fetch = vi.fn();

// Mock localStorage with actual storage implementation
const localStorageMock = (() => {
	let store: Record<string, string> = {};

	return {
		getItem: (key: string) => {
			return store[key] ?? null;
		},
		setItem: (key: string, value: string) => {
			store[key] = String(value);
		},
		removeItem: (key: string) => {
			delete store[key];
		},
		clear: () => {
			store = {};
		},
		get length() {
			return Object.keys(store).length;
		},
		key: (index: number) => {
			const keys = Object.keys(store);
			return keys[index] ?? null;
		}
	};
})();

globalThis.localStorage = localStorageMock as any;
