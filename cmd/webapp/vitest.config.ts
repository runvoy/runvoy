import { defineConfig } from 'vitest/config';
import { sveltekit } from '@sveltejs/kit/vite';

export default defineConfig({
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
			// Fails tests if coverage drops below these percentages
			lines: 70,
			functions: 70,
			branches: 60,
			statements: 70,
			// Check coverage for each file individually
			perFile: true,
			// Skip coverage for files in these paths
			skipFull: false,
			// Show which files are below thresholds
			reportOnFailure: true
		}
	}
});
