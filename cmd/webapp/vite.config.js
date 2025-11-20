import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import { readFileSync } from 'fs';
import { join } from 'path';

// Read version from VERSION file if VITE_RUNVOY_VERSION is not set
if (!process.env.VITE_RUNVOY_VERSION) {
	try {
		const versionPath = join(__dirname, '../../VERSION');
		const version = readFileSync(versionPath, 'utf-8').trim();
		process.env.VITE_RUNVOY_VERSION = version;
	} catch (error) {
		// VERSION file not found, leave it empty
		process.env.VITE_RUNVOY_VERSION = '';
	}
}

export default defineConfig({
	plugins: [sveltekit()]
});


