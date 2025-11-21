import { defineConfig } from 'vitest/config';
import { sveltekit } from '@sveltejs/kit/vite';

export default defineConfig({
	// @ts-expect-error - SvelteKit plugin type mismatch due to vite version differences between vite and vitest's bundled vite
	plugins: [sveltekit()],
	test: {
		globals: true,
		environment: 'jsdom',
		setupFiles: ['./vitest.setup.ts'],
		coverage: {
			provider: 'v8',
			reporter: ['text', 'json', 'html'],
			include: ['src/**/*.{ts,svelte}'],
			exclude: [
				'src/**/*.test.ts',
				'src/**/*.test.svelte',
				'src/routes/**',
				'src/**/*.d.ts'
			],
			// Enforce minimum coverage thresholds
			thresholds: {
				lines: 70,
				functions: 70,
				branches: 60,
				statements: 70
			}
		}
	}
});
