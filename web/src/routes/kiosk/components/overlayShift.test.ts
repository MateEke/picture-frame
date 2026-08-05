import { describe, it, expect } from 'vitest';
import { overlayShift, overlayShiftPair, SHIFT_X_REM, SHIFT_Y_REM } from './overlayShift';

function at(minute: number): Date {
	return new Date(2026, 7, 5, 10, minute, 0, 0);
}

const REM = 16;

function lead(minute: number, rem = REM): string {
	return overlayShiftPair(at(minute), rem, false).lead;
}

function parse(style: string): { x: number; y: number } {
	const m = /^translate\((-?\d+)px, (-?\d+)px\)$/.exec(style);
	if (!m) throw new Error(`unparseable transform: ${style}`);
	return { x: Number(m[1]), y: Number(m[2]) };
}

describe('overlayShift', () => {
	describe('overlayShift', () => {
		it('starts the orbit at the positive x extreme', () => {
			const { x, y } = overlayShift(at(0));
			expect(x).toBeCloseTo(SHIFT_X_REM, 6);
			expect(y).toBeCloseTo(0, 6);
		});

		it('reaches the positive y extreme at a quarter turn', () => {
			const { x, y } = overlayShift(at(15));
			expect(x).toBeCloseTo(0, 6);
			expect(y).toBeCloseTo(SHIFT_Y_REM, 6);
		});

		it('reaches the negative x extreme at a half turn', () => {
			const { x, y } = overlayShift(at(30));
			expect(x).toBeCloseTo(-SHIFT_X_REM, 6);
			expect(y).toBeCloseTo(0, 6);
		});

		it('reaches the negative y extreme at a three-quarter turn', () => {
			const { x, y } = overlayShift(at(45));
			expect(x).toBeCloseTo(0, 6);
			expect(y).toBeCloseTo(-SHIFT_Y_REM, 6);
		});

		it('wraps to the start on the next hour', () => {
			expect(overlayShift(new Date(2026, 7, 5, 11, 0, 0, 0))).toEqual(overlayShift(at(0)));
		});

		it('ignores seconds within a minute', () => {
			expect(overlayShift(new Date(2026, 7, 5, 10, 24, 59, 999))).toEqual(overlayShift(at(24)));
		});

		it('visits a distinct position on every step', () => {
			const seen = new Set<string>();
			for (let m = 0; m < 60; m++) {
				const { x, y } = overlayShift(at(m));
				seen.add(`${x.toFixed(6)},${y.toFixed(6)}`);
			}
			expect(seen.size).toBe(60);
		});

		// Perceptibility budget: ~1.3px per step at 16px/rem.
		it('never moves more than 0.1rem in one step', () => {
			for (let m = 0; m < 60; m++) {
				const a = overlayShift(at(m));
				const b = overlayShift(at((m + 1) % 60));
				expect(Math.hypot(b.x - a.x, b.y - a.y)).toBeLessThan(0.1);
			}
		});
	});

	describe('overlayShiftPair', () => {
		// Hard-coded: asserting against SHIFT_*_REM would be self-referential.
		it('emits whole pixels at the x extremes', () => {
			expect(lead(0)).toBe('translate(12px, 0px)');
			expect(lead(30)).toBe('translate(-12px, 0px)');
		});

		it('emits whole pixels at the y extremes', () => {
			expect(lead(15)).toBe('translate(0px, 8px)');
			expect(lead(45)).toBe('translate(0px, -8px)');
		});

		it('rounds intermediate positions to whole pixels', () => {
			expect(lead(24)).toBe('translate(-10px, 5px)');
		});

		it('never emits a fractional or negative-zero value', () => {
			for (let m = 0; m < 60; m++) {
				for (const portrait of [false, true]) {
					const { lead: l, trail } = overlayShiftPair(at(m), REM, portrait);
					for (const style of [l, trail]) {
						expect(style).toMatch(/^translate\(-?\d+px, -?\d+px\)$/);
						expect(style).not.toContain('-0px');
					}
				}
			}
		});

		it('spans the full pixel amplitude over one orbit', () => {
			const xs: number[] = [];
			const ys: number[] = [];
			for (let m = 0; m < 60; m++) {
				const { x, y } = parse(lead(m));
				xs.push(x);
				ys.push(y);
			}
			expect(Math.max(...xs)).toBe(12);
			expect(Math.min(...xs)).toBe(-12);
			expect(Math.max(...ys)).toBe(8);
			expect(Math.min(...ys)).toBe(-8);
		});

		it('scales with the root font size', () => {
			expect(lead(0, 32)).toBe('translate(24px, 0px)');
		});

		it('mirrors the trailing block across the vertical centre in landscape', () => {
			expect(overlayShiftPair(at(0), REM, false)).toEqual({
				lead: 'translate(12px, 0px)',
				trail: 'translate(-12px, 0px)'
			});
			expect(overlayShiftPair(at(24), REM, false)).toEqual({
				lead: 'translate(-10px, 5px)',
				trail: 'translate(10px, 5px)'
			});
		});

		it('keeps the landscape blocks opposed horizontally and level vertically', () => {
			for (let m = 0; m < 60; m++) {
				const { lead: l, trail } = overlayShiftPair(at(m), REM, false);
				const a = parse(l);
				const b = parse(trail);
				expect(a.x + b.x).toBe(0);
				expect(b.y).toBe(a.y);
			}
		});

		it('keeps both portrait blocks in lockstep so they stay left-aligned', () => {
			for (let m = 0; m < 60; m++) {
				const { lead: l, trail } = overlayShiftPair(at(m), REM, true);
				expect(trail).toBe(l);
			}
		});
	});
});
