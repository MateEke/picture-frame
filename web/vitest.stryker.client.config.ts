import { fileURLToPath } from 'node:url';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vitest/config';
import { playwright } from '@vitest/browser-playwright';
import { sveltekit } from '@sveltejs/kit/vite';
import { meteocons, themeColor } from './src/lib/build/meteocons';

// EXPERIMENT: can Stryker drive the browser `client` project? (sse.svelte.ts only)
const kioskCss = fileURLToPath(new URL('./src/routes/kiosk/layout.css', import.meta.url));

export default defineConfig({
	plugins: [tailwindcss(), meteocons(themeColor(kioskCss, '--color-kiosk-fg')), sveltekit()],
	// Per-worker Vite cache: concurrent Stryker runners racing one dep-optimizer
	// commit fail with ENOTEMPTY on cold caches (first CI run).
	cacheDir: `node_modules/.vite/stryker-${process.env.STRYKER_MUTATOR_WORKER ?? '0'}`,
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
});
