import { defineConfig } from 'vitest/config';
import { sveltekit } from '@sveltejs/kit/vite';

export default defineConfig({
	plugins: [sveltekit()],
	resolve: {
		conditions: ['browser', 'import', 'module', 'default']
	},
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
				lines: 30,
				functions: 40,
				branches: 20,
				statements: 30
			}
		}
	}
});
