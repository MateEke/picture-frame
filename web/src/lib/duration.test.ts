import { describe, it, expect } from 'vitest';
import {
	toSeconds,
	formatDuration,
	toGoDuration,
	splitDuration,
	checkDurationParts,
	durationHint,
	isDurationUnit,
	sliderOptions,
	type DurationUnit,
	DURATIONS
} from './duration';

describe('duration', () => {
	describe('toSeconds', () => {
		it('treats an empty string as zero', () => {
			expect(toSeconds('')).toBe(0);
		});

		it('parses single units', () => {
			expect(toSeconds('30s')).toBe(30);
			expect(toSeconds('2m')).toBe(120);
			expect(toSeconds('1h')).toBe(3600);
		});

		it('sums compound durations', () => {
			expect(toSeconds('1h30m')).toBe(5400);
		});

		it('parses sub-second units', () => {
			expect(toSeconds('500ms')).toBeCloseTo(0.5);
		});

		it('parses fractional values', () => {
			expect(toSeconds('1.5h')).toBe(5400);
		});

		it('parses multi-digit fractional values (regex keeps all decimals)', () => {
			expect(toSeconds('1.25h')).toBe(4500);
		});

		it('returns zero for unparseable input', () => {
			expect(toSeconds('abc')).toBe(0);
		});
	});

	describe('formatDuration', () => {
		it('renders the zero label for a blank duration', () => {
			expect(formatDuration('', 'Never')).toBe('Never');
		});

		it('defaults the zero label to Off', () => {
			expect(formatDuration('0s')).toBe('Off');
		});

		it('renders single units', () => {
			expect(formatDuration('30s')).toBe('30 sec');
			expect(formatDuration('2m')).toBe('2 min');
			expect(formatDuration('1h')).toBe('1 hr');
		});

		it('renders the two most-significant units', () => {
			expect(formatDuration('1h30m')).toBe('1 hr 30 min');
		});

		it('caps at the two most-significant units, dropping the rest', () => {
			expect(formatDuration('1h30m45s')).toBe('1 hr 30 min');
		});

		it('renders days', () => {
			expect(formatDuration('24h')).toBe('1 day');
		});
	});

	describe('toGoDuration', () => {
		it.each([
			[0, ''],
			[15, '15s'],
			[120, '2m'],
			[5400, '1h30m'],
			[90, '1m30s'],
			[3630, '1h0m30s']
		])('renders %d seconds as %j', (secs, want) => {
			expect(toGoDuration(secs)).toBe(want);
			expect(toSeconds(want)).toBe(secs);
		});
	});

	describe('isDurationUnit', () => {
		it.each(['sec', 'min', 'hr'])('accepts %j', (u) => {
			expect(isDurationUnit(u)).toBe(true);
		});

		it.each(['s', 'hour', ''])('rejects %j', (u) => {
			expect(isDurationUnit(u)).toBe(false);
		});
	});

	describe('splitDuration', () => {
		it.each([
			['2m', { amount: '2', unit: 'min' }],
			['90s', { amount: '90', unit: 'sec' }],
			['1h30m', { amount: '90', unit: 'min' }],
			['3h', { amount: '3', unit: 'hr' }],
			['1h0m30s', { amount: '3630', unit: 'sec' }]
		])('splits %j into the largest whole unit', (value, want) => {
			expect(splitDuration(value, DURATIONS.slideshowInterval)).toEqual(want);
		});

		it('leaves the amount blank for an off value, in the minimum unit', () => {
			expect(splitDuration('', DURATIONS.blankAfter)).toEqual({ amount: '', unit: 'min' });
		});
	});

	describe('checkDurationParts', () => {
		const slides = DURATIONS.slideshowInterval;

		const accepted: [string, DurationUnit, string][] = [
			['5', 'sec', '5s'],
			['24', 'hr', '24h'],
			['15', 'sec', '15s'],
			['65', 'min', '1h5m'],
			[' 2 ', 'hr', '2h'],
			['120', 'sec', '2m']
		];
		it.each(accepted)('accepts %j %s as %j', (amount, unit, value) => {
			expect(checkDurationParts(amount, unit, slides)).toEqual({ value });
		});

		const rejected: [string, DurationUnit, string][] = [
			['', 'sec', 'Enter a whole number.'],
			['1.5', 'hr', 'Enter a whole number.'],
			['-5', 'sec', 'Enter a whole number.'],
			['abc', 'sec', 'Enter a whole number.'],
			['3', 'sec', 'Minimum is 5 sec.'],
			['0', 'sec', 'Minimum is 5 sec.'],
			['25', 'hr', 'Maximum is 1 day.']
		];
		it.each(rejected)('rejects %j %s', (amount, unit, error) => {
			expect(checkDurationParts(amount, unit, slides)).toEqual({ error });
		});

		it('accepts zero where the field can be turned off', () => {
			expect(checkDurationParts('0', 'min', DURATIONS.blankAfter)).toEqual({ value: '' });
			expect(checkDurationParts('10', 'min', DURATIONS.blankAfter)).toEqual({ value: '10m' });
		});
	});

	describe('durationHint', () => {
		it('states the range', () => {
			expect(durationHint(DURATIONS.slideshowInterval, 'Off')).toBe('5 sec to 1 day');
		});

		it('mentions zero where the field can be turned off', () => {
			expect(durationHint(DURATIONS.blankAfter, 'Never')).toBe('1 min to 1 day, or 0 for Never');
		});
	});

	describe('sliderOptions', () => {
		it('adds a value that is not a stop', () => {
			expect(sliderOptions(['30s', '1m'], '2m0s', undefined)).toEqual(['30s', '1m', '2m0s']);
		});

		it('matches stops by duration, not spelling', () => {
			expect(sliderOptions(['30s', '1m', '2m'], '2m0s', undefined)).toEqual(['30s', '1m', '2m']);
		});

		it('keeps the saved custom value after moving to a stop', () => {
			expect(sliderOptions(['30s', '1m'], '1m', '10s')).toEqual(['10s', '30s', '1m']);
		});

		it('keeps both a saved and a different current custom value', () => {
			expect(sliderOptions(['30s', '1m'], '15s', '10s')).toEqual(['10s', '15s', '30s', '1m']);
		});
	});

	describe('DURATIONS', () => {
		it('keeps every stop ascending by seconds', () => {
			for (const { stops } of Object.values(DURATIONS)) {
				const seconds = stops.map(toSeconds);
				const sorted = [...seconds].sort((a, b) => a - b);
				expect(seconds).toEqual(sorted);
			}
		});

		it('keeps every stop within the typed-value limits', () => {
			for (const { stops, min, max } of Object.values(DURATIONS)) {
				const nonZero = stops.filter((s) => s !== '').map(toSeconds);
				expect(Math.min(...nonZero)).toBeGreaterThanOrEqual(toSeconds(min));
				expect(Math.max(...nonZero)).toBeLessThanOrEqual(toSeconds(max));
			}
		});

		it('pins the exact stops and limits for every field', () => {
			expect(DURATIONS).toEqual({
				slideshowInterval: {
					stops: ['30s', '1m', '2m', '5m', '10m', '15m', '30m', '1h'],
					min: '5s',
					max: '24h'
				},
				blankAfter: {
					stops: ['', '5m', '10m', '15m', '20m', '30m', '45m', '1h', '2h'],
					min: '1m',
					max: '24h'
				},
				immichSync: {
					stops: ['5m', '10m', '15m', '30m', '1h', '2h', '6h', '12h', '24h'],
					min: '1m',
					max: '168h'
				},
				weatherPoll: { stops: ['10m', '15m', '30m', '1h'], min: '2m', max: '24h' },
				weatherRetry: { stops: ['10s', '30s', '1m', '2m', '5m'], min: '5s', max: '1h' },
				mqttStale: { stops: ['', '1m', '5m', '10m', '30m', '1h'], min: '30s', max: '24h' },
				sensorPoll: { stops: ['10s', '30s', '1m', '2m', '5m'], min: '5s', max: '1h' },
				sensorReset: { stops: ['', '1m', '5m', '10m', '30m'], min: '1m', max: '24h' }
			});
		});
	});
});
