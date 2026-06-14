import { readFileSync } from 'node:fs';
import { readFile } from 'node:fs/promises';
import { basename } from 'node:path';
import type { Plugin } from 'vite';

const MARKER = '?meteocon';

/** Path of a `*.svg?meteocon` id, or null when the marker is absent. */
export function meteoconPath(id: string): string | null {
	return id.endsWith(MARKER) ? id.slice(0, -MARKER.length) : null;
}

/** First value of a CSS custom property in `css`, or null. */
export function matchThemeColor(css: string, varName: string): string | null {
	const stripped = css.replace(/\/\*[\s\S]*?\*\//g, '');
	const escaped = varName.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
	const m = stripped.match(new RegExp(`${escaped}\\s*:\\s*([^;}]+)`));
	return m ? m[1].trim() : null;
}

/**
 * Reads a theme token from a CSS file; throws so a renamed token fails the
 * build loudly. Resolved once at config load, a token change needs a dev
 * server restart to re-tint the icons.
 */
export function themeColor(cssPath: string, varName: string): string {
	const color = matchThemeColor(readFileSync(cssPath, 'utf8'), varName);
	if (!color) throw new Error(`${varName} not found in ${cssPath}`);
	return color;
}

/** Bakes the tint into the SVG, `<img>`-embedded SVGs can't inherit CSS color. */
export function tintSvg(svg: string, color: string): string {
	// bare replacement also covers style="fill:currentColor", not just attributes
	return svg.replaceAll('currentColor', color);
}

export function toDataUri(svg: string): string {
	return `data:image/svg+xml;base64,${Buffer.from(svg).toString('base64')}`;
}

/**
 * Resolves `*.svg?meteocon` imports, so icons stay in the dependency instead
 * of the repo. Builds emit tinted SVGs as cacheable assets fetched on demand;
 * the dev server (which cannot emit assets) inlines them as data URIs.
 */
export function meteocons(color: string): Plugin {
	let serve = false;
	return {
		name: 'meteocons-tint',
		enforce: 'pre',
		configResolved(config) {
			serve = config.command === 'serve';
		},
		async load(id, options) {
			const path = meteoconPath(id);
			if (!path) return null;
			const svg = tintSvg(await readFile(path, 'utf8'), color);
			// dev server can't emit assets; the SSR pass shouldn't (its copies
			// never ship, and its ROLLUP_FILE_URLs wouldn't resolve)
			if (serve || options?.ssr) return `export default ${JSON.stringify(toDataUri(svg))};`;
			const ref = this.emitFile({ type: 'asset', name: basename(path), source: svg });
			return `export default import.meta.ROLLUP_FILE_URL_${ref};`;
		}
	};
}
