import { describe, expect, it } from 'vitest';
import { METEOCONS, weatherIconFor } from './weather-icons';

const OWM_CODES = [
	'01d',
	'01n',
	'02d',
	'02n',
	'03d',
	'03n',
	'04d',
	'04n',
	'09d',
	'09n',
	'10d',
	'10n',
	'11d',
	'11n',
	'13d',
	'13n',
	'50d',
	'50n'
];

describe('weather icons', () => {
	it('covers all 18 OWM icon codes', () => {
		for (const code of OWM_CODES) {
			expect(METEOCONS[code], `missing icon for ${code}`).toBeTruthy();
		}
	});

	it('falls back to clear-day for unknown codes', () => {
		expect(weatherIconFor('unknown')).toBe(METEOCONS['01d']);
	});

	it('ships every icon fully tinted and static', () => {
		// in vitest the plugin serves data URIs; decode and sweep the artwork
		const unique = new Set(Object.values(METEOCONS));
		for (const uri of unique) {
			expect(uri, 'expected dev-server data URIs').toMatch(/^data:image\/svg\+xml;base64,/);
			const svg = Buffer.from(uri.split('base64,')[1], 'base64').toString();
			expect(svg).not.toContain('currentColor');
			expect(svg).not.toContain('<animate');
		}
	});
});
