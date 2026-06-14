import { fileURLToPath } from 'node:url';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vitest/config';
import { sveltekit } from '@sveltejs/kit/vite';
import { meteocons, themeColor } from './src/lib/build/meteocons';

// Stryker-only Vitest config: the node-side `server` project in isolation.
// Deliberately omits the hey-api plugin (the committed src/lib/api is the test
// source of truth) so Stryker never triggers a mid-run client regen, and omits
// the browser `client` project, which Stryker's vitest runner can't drive.
const kioskCss = fileURLToPath(new URL('./src/routes/kiosk/layout.css', import.meta.url));

export default defineConfig({
	plugins: [tailwindcss(), meteocons(themeColor(kioskCss, '--color-kiosk-fg')), sveltekit()],
	// Per-worker Vite cache: concurrent Stryker runners racing one dep-optimizer
	// commit fail with ENOTEMPTY on cold caches (first CI run).
	cacheDir: `node_modules/.vite/stryker-${process.env.STRYKER_MUTATOR_WORKER ?? '0'}`,
	test: {
		name: 'server',
		environment: 'node',
		expect: { requireAssertions: true },
		include: ['src/**/*.{test,spec}.{js,ts}'],
		exclude: ['src/**/*.svelte.{test,spec}.{js,ts}']
	}
});
