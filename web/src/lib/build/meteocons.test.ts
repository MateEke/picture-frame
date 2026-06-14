import { readFile } from 'node:fs/promises';
import { describe, expect, it, vi } from 'vitest';
import {
	matchThemeColor,
	meteoconPath,
	meteocons,
	themeColor,
	tintSvg,
	toDataUri
} from './meteocons';

const SAMPLE = '<svg fill="none"><path stroke="currentColor"/><g fill="currentColor"/></svg>';

describe('meteocons build plugin', () => {
	it('tints every currentColor fill and stroke', () => {
		const out = tintSvg(SAMPLE, '#fff8f0');
		expect(out).not.toContain('currentColor');
		expect(out).toContain('stroke="#fff8f0"');
		expect(out).toContain('fill="#fff8f0"');
	});

	it('leaves other paint values alone', () => {
		expect(tintSvg(SAMPLE, '#fff8f0')).toContain('fill="none"');
	});

	it('tints the css style form too', () => {
		const out = tintSvg('<path style="fill:currentColor"/>', '#fff8f0');
		expect(out).toContain('style="fill:#fff8f0"');
	});

	it('encodes the svg as a base64 data uri', () => {
		const uri = toDataUri(SAMPLE);
		expect(uri.startsWith('data:image/svg+xml;base64,')).toBe(true);
		const decoded = Buffer.from(uri.split(',')[1], 'base64').toString();
		expect(decoded).toBe(SAMPLE);
	});

	it('tints a real package icon completely', async () => {
		const icon = 'node_modules/@meteocons/svg-static/monochrome/rain.svg';
		const tinted = tintSvg(await readFile(icon, 'utf8'), '#fff8f0');
		expect(tinted).toContain('#fff8f0');
		expect(tinted).not.toContain('currentColor');
		expect(tinted).not.toContain('<animate');
	});

	it('extracts the path from marked ids and rejects unmarked ones', () => {
		expect(meteoconPath('a/b/rain.svg?meteocon')).toBe('a/b/rain.svg');
		expect(meteoconPath('a/b/rain.svg')).toBeNull();
	});

	it('extracts a theme color from css text', () => {
		const css = '@theme {\n\t--font-sans: X;\n\t--color-kiosk-fg: #fff8f0;\n}';
		expect(matchThemeColor(css, '--color-kiosk-fg')).toBe('#fff8f0');
		expect(matchThemeColor(css, '--color-missing')).toBeNull();
	});

	it('survives a trailing comment on the declaration', () => {
		const css = '@theme {\n\t--color-kiosk-fg: #fff8f0 /* warm white */;\n}';
		expect(matchThemeColor(css, '--color-kiosk-fg')).toBe('#fff8f0');
	});

	it('survives a missing semicolon before the closing brace', () => {
		const css = '@theme {\n\t--color-kiosk-fg: #fff8f0\n}';
		expect(matchThemeColor(css, '--color-kiosk-fg')).toBe('#fff8f0');
	});

	it('treats regex metacharacters in the token name literally', () => {
		const css = '--colorXkiosk-fg: #bad;\n--color.kiosk-fg: #good;';
		expect(matchThemeColor(css, '--color.kiosk-fg')).toBe('#good');
	});

	it('reads the kiosk foreground token the build actually uses', () => {
		const color = themeColor('src/routes/kiosk/layout.css', '--color-kiosk-fg');
		expect(color).toMatch(/^#[0-9a-fA-F]{3,8}$/);
	});

	it('throws for a missing token, failing the build loudly', () => {
		expect(() => themeColor('src/routes/kiosk/layout.css', '--color-nope')).toThrow(/--color-nope/);
	});

	describe('the vite plugin load hook', () => {
		const ICON = 'node_modules/@meteocons/svg-static/monochrome/rain.svg';
		// The hooks are plain functions here, but Vite types them as ObjectHook
		// unions; pin the callable shapes once so the tests can invoke them.
		function build() {
			const plugin = meteocons('#fff8f0');
			const configResolved = plugin.configResolved as (c: { command: string }) => void;
			const load = plugin.load as (
				this: { emitFile?: (a: unknown) => string },
				id: string,
				options?: { ssr?: boolean }
			) => Promise<string | null>;
			return { configResolved, load };
		}

		it('declares itself as a pre-enforced named plugin', () => {
			// enforce:'pre' matters: it must intercept ?meteocon ids before SvelteKit.
			const plugin = meteocons('#fff8f0');
			expect(plugin.name).toBe('meteocons-tint');
			expect(plugin.enforce).toBe('pre');
		});

		it('ignores ids without the ?meteocon marker', async () => {
			const { load } = build();
			expect(await load.call({}, ICON, undefined)).toBeNull();
		});

		it('inlines a tinted data URI in the dev server', async () => {
			const { configResolved, load } = build();
			configResolved({ command: 'serve' });
			const out = await load.call({}, `${ICON}?meteocon`, undefined);
			expect(out).toMatch(/^export default "data:image\/svg\+xml;base64,/);
			const json = out!.slice('export default '.length, -1);
			const decoded = Buffer.from(JSON.parse(json).split(',')[1], 'base64').toString();
			expect(decoded).toContain('#fff8f0');
			expect(decoded).not.toContain('currentColor');
		});

		it('inlines a data URI during SSR even in a build', async () => {
			const { configResolved, load } = build();
			configResolved({ command: 'build' });
			const out = await load.call({ emitFile: vi.fn() }, `${ICON}?meteocon`, { ssr: true });
			expect(out).toMatch(/^export default "data:/);
		});

		it('emits a cacheable asset and references it in a client build', async () => {
			const { configResolved, load } = build();
			configResolved({ command: 'build' });
			const emitFile = vi.fn(() => 'REF123');
			// Vite may call load without an options arg: the optional chain must hold.
			const out = await load.call({ emitFile }, `${ICON}?meteocon`, undefined);

			expect(emitFile).toHaveBeenCalledWith({
				type: 'asset',
				name: 'rain.svg',
				source: expect.stringContaining('#fff8f0')
			});
			expect(out).toBe('export default import.meta.ROLLUP_FILE_URL_REF123;');
		});
	});
});
