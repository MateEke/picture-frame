import adapter from '@sveltejs/adapter-static';
import type { Config } from '@sveltejs/kit/vite';

// Shared by every sveltekit() call (app + Stryker) and ESLint's Svelte parser.
export const compilerOptions: Config['compilerOptions'] = {
	// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
	runes: ({ filename }) => (filename.split(/[/\\]/).includes('node_modules') ? undefined : true)
};

export const kitConfig: Config = {
	compilerOptions,
	adapter: adapter({ fallback: 'index.html' }),
	version: {
		// Baked build version (must match the Go binary's version.Version) that stamps kiosk
		// heartbeats for the update commit gate. Set by `make build-ui`/goreleaser; "dev" otherwise.
		name: process.env.PUBLIC_APP_VERSION || 'dev',
		// versionReload owns reloads via SSE; skip Kit's hourly version.json poll.
		pollInterval: 0
	}
};
