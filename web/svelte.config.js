import adapter from '@sveltejs/adapter-static';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	compilerOptions: {
		// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
		runes: ({ filename }) => (filename.split(/[/\\]/).includes('node_modules') ? undefined : true)
	},
	kit: {
		adapter: adapter({
			fallback: 'index.html'
		}),
		// Baked build version (must match the Go binary's version.Version) that stamps kiosk
		// heartbeats for the update commit gate. Set by `make build-ui`/goreleaser; "dev" otherwise.
		version: { name: process.env.PUBLIC_APP_VERSION || 'dev' },
		typescript: {
			// Cover the root Playwright config so it resolves Node globals.
			config: (tsconfig) => {
				tsconfig.include.push('../playwright.config.ts');
				return tsconfig;
			}
		}
	}
};

export default config;
