import { describe, it, expect } from 'vitest';
import { toSeconds, formatDuration, DURATION_STOPS } from './duration';

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

	describe('DURATION_STOPS', () => {
		it('keeps every stop ascending by seconds', () => {
			for (const stops of Object.values(DURATION_STOPS)) {
				const seconds = stops.map(toSeconds);
				const sorted = [...seconds].sort((a, b) => a - b);
				expect(seconds).toEqual(sorted);
			}
		});

		it('pins the exact slider stops for every field', () => {
			expect(DURATION_STOPS).toEqual({
				slideshowInterval: ['30s', '1m', '2m', '5m', '10m', '15m', '30m', '1h'],
				blankAfter: ['', '5m', '10m', '15m', '20m', '30m', '45m', '1h', '2h'],
				immichSync: ['5m', '10m', '15m', '30m', '1h', '2h', '6h', '12h', '24h'],
				weatherPoll: ['10m', '15m', '30m', '1h'],
				weatherRetry: ['10s', '30s', '1m', '2m', '5m'],
				mqttStale: ['1m', '5m', '10m', '30m', '1h'],
				sensorPoll: ['10s', '30s', '1m', '2m', '5m'],
				sensorReset: ['', '1m', '5m', '10m', '30m']
			});
		});
	});
});
