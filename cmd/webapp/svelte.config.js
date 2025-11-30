import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

const appVersion = (() => {
        if (process.env.RUNVOY_VERSION) {
                return process.env.RUNVOY_VERSION;
        }

        if (process.env.VITE_RUNVOY_VERSION) {
                return process.env.VITE_RUNVOY_VERSION;
        }

        try {
                return readFileSync(join(__dirname, '../../VERSION'), 'utf-8').trim();
        } catch (error) {
                return '';
        }
})();

/** @type {import('@sveltejs/kit').Config} */
const config = {
        preprocess: vitePreprocess(),
        compilerOptions: {
                runes: true
        },
        kit: {
                adapter: adapter({
			fallback: 'index.html'
		}),
                version: {
                        name: appVersion
                },
                prerender: {
                        handleHttpError: 'warn'
                }
        }
};

export default config;

