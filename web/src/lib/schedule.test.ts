import { describe, it, expect } from 'vitest';
import { formatClock, nextChangeTime, scheduleError, scheduleSummary } from './schedule';

describe('schedule', () => {
	it('formats a time in 12-hour style for en-US', () => {
		expect(formatClock('07:05', 'en-US')).toBe('7:05 AM');
		expect(formatClock('23:00', 'en-US')).toBe('11:00 PM');
	});

	it('formats a time in 24-hour style for en-GB', () => {
		expect(formatClock('07:05', 'en-GB')).toBe('07:05');
	});

	it('falls back to the default locale for an invalid tag', () => {
		expect(formatClock('23:00', 'not a locale!')).toBe(formatClock('23:00', ''));
	});

	it('rejects equal off and on times', () => {
		expect(scheduleError('07:00', '07:00')).toBe("Off and on times can't be the same.");
	});

	it('accepts distinct, one-sided, or empty times', () => {
		expect(scheduleError('23:00', '07:00')).toBeNull();
		expect(scheduleError('23:00', '')).toBeNull();
		expect(scheduleError('', '07:00')).toBeNull();
		expect(scheduleError('', '')).toBeNull();
	});

	it('summarizes an overnight window', () => {
		expect(scheduleSummary('23:00', '07:00', 'en-GB')).toBe('Off every night from 23:00 to 07:00.');
	});

	it('summarizes a same-day window', () => {
		expect(scheduleSummary('09:00', '17:00', 'en-GB')).toBe('Off every day from 09:00 to 17:00.');
	});

	it('summarizes an off-only schedule', () => {
		expect(scheduleSummary('23:00', '', 'en-GB')).toBe(
			'Turns off every day at 23:00 and stays off until you turn it back on.'
		);
	});

	it('summarizes an on-only schedule', () => {
		expect(scheduleSummary('', '07:00', 'en-GB')).toBe('Turns on every day at 07:00.');
	});

	it('summarizes no schedule', () => {
		expect(scheduleSummary('', '', 'en-GB')).toBe(
			'No schedule. Set a time to turn the screen off or on every day.'
		);
	});

	it('formats summary times with the locale', () => {
		expect(scheduleSummary('23:00', '07:00', 'en-US')).toBe(
			'Off every night from 11:00 PM to 7:00 AM.'
		);
	});
	it('labels a next change later today with just the time', () => {
		const next = { at: '2026-10-17T23:00:00+02:00', on: false, time: '23:00' };
		expect(nextChangeTime(next, 'en-GB', Date.parse('2026-10-17T08:00:00+02:00'))).toBe('23:00');
	});

	it("labels a next change on the frame's next day as tomorrow", () => {
		const next = { at: '2026-10-18T07:20:00+02:00', on: false, time: '07:20' };
		expect(nextChangeTime(next, 'en-US', Date.parse('2026-10-17T07:22:00+02:00'))).toBe(
			'7:20 AM tomorrow'
		);
	});

	it("uses the frame's offset, not the browser's, to find its day", () => {
		// 23:30 UTC on the 17th is already the 18th at UTC+2.
		const next = { at: '2026-10-18T07:00:00+02:00', on: true, time: '07:00' };
		expect(nextChangeTime(next, 'en-GB', Date.parse('2026-10-17T23:30:00Z'))).toBe('07:00');
	});

	it('counts the minutes of a half-hour offset', () => {
		// 18:40 UTC is 00:10 on the 18th at +05:30, but still the 17th at +04:30.
		const next = { at: '2026-10-18T05:00:00+05:30', on: true, time: '05:00' };
		expect(nextChangeTime(next, 'en-GB', Date.parse('2026-10-17T18:40:00Z'))).toBe('05:00');
	});

	it('handles negative and UTC offsets', () => {
		const west = { at: '2026-10-17T20:00:00-05:00', on: false, time: '20:00' };
		expect(nextChangeTime(west, 'en-GB', Date.parse('2026-10-18T00:30:00Z'))).toBe('20:00');
		const utc = { at: '2026-10-18T07:00:00Z', on: true, time: '07:00' };
		expect(nextChangeTime(utc, 'en-GB', Date.parse('2026-10-17T22:00:00Z'))).toBe('07:00 tomorrow');
	});
});
