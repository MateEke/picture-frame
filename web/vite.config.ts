import { fileURLToPath } from 'node:url';
import { heyApiPlugin } from '@hey-api/vite-plugin';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vitest/config';
import { playwright } from '@vitest/browser-playwright';
import { sveltekit } from '@sveltejs/kit/vite';
import sbom from 'rollup-plugin-sbom';
import { meteocons, themeColor } from './src/lib/build/meteocons';

// anchored to this file, not the cwd, so invoking vite from elsewhere still works
const kioskCss = fileURLToPath(new URL('./src/routes/kiosk/layout.css', import.meta.url));

// When BACKEND_URL is set, proxy API and SSE requests to a remote backend
// instead of localhost:8080. Useful for developing against the Pi directly:
//   BACKEND_URL=http://1.2.3.4:8080 npm run dev
const backendURL = process.env.BACKEND_URL ?? 'http://localhost:8080';
const backendProxy = {
	'/events': { target: backendURL, changeOrigin: true, compress: false },
	'/api': backendURL,
	'/img': backendURL,
	'/healthz': backendURL
};

// Skip client regeneration under vitest: the committed src/lib/api is the test
// source of truth, and a mid-run delete+rewrite races test module loads.
const apiPlugins = process.env.VITEST
	? []
	: [
			heyApiPlugin({
				config: {
					input: process.env.BACKEND_URL
						? { path: `${backendURL}/openapi.json`, watch: true, fetch: { method: 'GET' } }
						: './openapi.json',
					output: './src/lib/api',
					plugins: [
						'@hey-api/typescript',
						{ name: '@hey-api/sdk', operations: { methodName: 'api{{name}}' } },
						{ name: '@hey-api/client-fetch', bundle: true, baseUrl: false }
					]
				}
			})
		];

export default defineConfig({
	plugins: [
		...apiPlugins,
		tailwindcss(),
		meteocons(themeColor(kioskCss, '--color-kiosk-fg')),
		sveltekit(),
		// Bundle-truth CycloneDX SBOM, only what survives tree-shaking (cf. build.license).
		sbom()
	],
	server: { proxy: backendProxy },
	// Native bundle-truth license notices for the client JS bundle (full text per package).
	// NOTE: like every JS-graph tool, this misses CSS-only @fontsource fonts, make notices
	// adds those explicitly from node_modules.
	build: { license: true },
	test: {
		expect: { requireAssertions: true },
		projects: [
			{
				extends: './vite.config.ts',
				test: {
					name: 'client',
					browser: {
						enabled: true,
						provider: playwright(),
						instances: [{ browser: 'chromium', headless: true }]
					},
					include: ['src/**/*.svelte.{test,spec}.{js,ts}'],
					exclude: ['src/lib/server/**']
				}
			},

			{
				extends: './vite.config.ts',
				test: {
					name: 'server',
					environment: 'node',
					include: ['src/**/*.{test,spec}.{js,ts}'],
					exclude: ['src/**/*.svelte.{test,spec}.{js,ts}']
				}
			}
		]
	}
});
