import { describe, it, expect, beforeEach } from 'vitest';
import { getCropRatio, setCropRatio, CROP_RATIOS } from './uploadPrefs';

// Browser project: localStorage available.
describe('uploadPrefs', () => {
	beforeEach(() => localStorage.clear());

	it('defaults to 16:9 when nothing is stored', () => {
		expect(getCropRatio().id).toBe('16:9');
	});

	it('falls back to the default for an unknown stored id', () => {
		localStorage.setItem('pf:crop-ratio', 'bogus');
		expect(getCropRatio().id).toBe('16:9');
	});

	it('reads the ratio stored under the crop-ratio key', () => {
		localStorage.setItem('pf:crop-ratio', '4:3');
		expect(getCropRatio().id).toBe('4:3');
	});

	it('round-trips every offered ratio', () => {
		for (const r of CROP_RATIOS) {
			setCropRatio(r);
			expect(getCropRatio()).toEqual(r);
		}
	});
});
