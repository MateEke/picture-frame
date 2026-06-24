import { describe, it, expect, vi } from 'vitest';
import {
	sensorKey,
	formatClockParts,
	formatWeekday,
	formatMonthDay,
	timezoneOffsetLabel,
	isSensorStale,
	SENSOR_STALE_MS,
	formatSensorValue,
	timeAgo,
	resolveOutsideTemp
} from './helpers';
import type { SensorPayload, WeatherPayload } from '$lib/api/types.gen';

const base: SensorPayload = {
	device_id: 'living_room',
	kind: 'temperature',
	value: 21.5,
	timestamp: '2026-05-18T00:00:00Z'
};

describe('helpers', () => {
	describe('sensorKey', () => {
		it('uses role when set', () => {
			expect(sensorKey({ ...base, role: 'inside' })).toBe('inside:temperature');
		});

		it('falls back to device_id when role is absent', () => {
			expect(sensorKey(base)).toBe('living_room:temperature');
		});

		it('falls back to device_id when role is empty string', () => {
			expect(sensorKey({ ...base, role: '' })).toBe('living_room:temperature');
		});

		it('preserves kind', () => {
			expect(sensorKey({ ...base, role: 'inside', kind: 'humidity' })).toBe('inside:humidity');
		});

		it('works for motion', () => {
			expect(sensorKey({ ...base, role: 'inside', kind: 'motion' })).toBe('inside:motion');
		});
	});

	describe('formatClockParts', () => {
		it('returns 24-hour parts and no period for a 24-hour locale', () => {
			const d = new Date('2026-05-18T14:05:00');
			expect(formatClockParts(d, 'hu-HU')).toEqual({
				hours: '14',
				separator: ':',
				minutes: '05',
				period: '',
				periodFirst: false
			});
		});

		it('splits the AM/PM marker out for en-US, trailing', () => {
			const d = new Date('2026-05-18T14:05:00');
			const parts = formatClockParts(d, 'en-US');
			expect(parts.period).toBe('PM');
			expect(parts.periodFirst).toBe(false);
			expect(parts.hours).toMatch(/^0?2$/); // 12-hour
			expect(parts.minutes).toBe('05');
		});

		it('marks the period as leading for locales that put it before the time', () => {
			const d = new Date('2026-05-18T14:05:00');
			const zh = formatClockParts(d, 'zh-TW');
			expect(zh.period).toBe('下午');
			expect(zh.periodFirst).toBe(true);
		});

		it('picks the separator between hour and minute, not the first literal', () => {
			// ko-KR parts are [dayPeriod, literal ' ', hour, literal ':', minute]
			const parts = formatClockParts(new Date('2026-05-18T14:05:00'), 'ko-KR');
			expect(parts.separator).toBe(':');
			expect(parts.periodFirst).toBe(true);
		});

		it('exposes the locale separator without surrounding whitespace', () => {
			const sep = formatClockParts(new Date('2026-05-18T08:03:00'), 'en-US').separator;
			expect(sep).toBe(sep.trim());
			expect(sep).not.toBe('');
		});

		it('zero-pads hours and minutes', () => {
			const d = new Date('2026-05-18T08:03:00');
			const parts = formatClockParts(d, 'hu-HU');
			expect(parts.hours).toBe('08');
			expect(parts.minutes).toBe('03');
		});
	});

	describe('formatWeekday', () => {
		it('localizes the weekday (hu-HU)', () => {
			// 2026-05-18 is a Monday (hétfő in hu-HU).
			expect(formatWeekday(new Date('2026-05-18T12:00:00'), 'hu-HU')).toBe('hétfő');
		});

		it('localizes the weekday (en-US)', () => {
			expect(formatWeekday(new Date('2026-05-18T12:00:00'), 'en-US')).toBe('Monday');
		});
	});

	describe('formatMonthDay', () => {
		it('includes the day number', () => {
			expect(formatMonthDay(new Date('2026-05-18T12:00:00'), 'en-US')).toContain('18');
		});

		it('localizes the month abbreviation', () => {
			expect(formatMonthDay(new Date('2026-05-18T12:00:00'), 'hu-HU')).toContain('máj');
		});
	});

	describe('timezone', () => {
		const utcNoon = new Date('2026-05-18T12:00:00Z');

		it('empty timezone matches the no-arg formatting', () => {
			expect(formatClockParts(utcNoon, 'hu-HU', '')).toEqual(formatClockParts(utcNoon, 'hu-HU'));
			expect(formatWeekday(utcNoon, 'en-US', '')).toBe(formatWeekday(utcNoon, 'en-US'));
			expect(formatMonthDay(utcNoon, 'en-US', '')).toBe(formatMonthDay(utcNoon, 'en-US'));
		});

		it('shifts the clock to the given zone', () => {
			expect(formatClockParts(utcNoon, 'hu-HU', 'UTC').hours).toBe('12');
			expect(formatClockParts(utcNoon, 'hu-HU', 'Asia/Tokyo').hours).toBe('21'); // UTC+9
		});

		it('shifts the date across midnight', () => {
			// 23:00Z Monday is already Tuesday in Tokyo.
			const lateUtc = new Date('2026-05-18T23:00:00Z');
			expect(formatWeekday(lateUtc, 'en-US', 'UTC')).toBe('Monday');
			expect(formatWeekday(lateUtc, 'en-US', 'Asia/Tokyo')).toBe('Tuesday');
		});

		it('falls back to the no-arg formatting when the zone is invalid', () => {
			expect(formatClockParts(utcNoon, 'hu-HU', 'Not/AZone')).toEqual(
				formatClockParts(utcNoon, 'hu-HU')
			);
			expect(formatWeekday(utcNoon, 'en-US', 'Not/AZone')).toBe(formatWeekday(utcNoon, 'en-US'));
			expect(formatMonthDay(utcNoon, 'en-US', 'Not/AZone')).toBe(formatMonthDay(utcNoon, 'en-US'));
		});
	});

	describe('timezoneOffsetLabel', () => {
		it('returns a short UTC offset for a known zone', () => {
			expect(timezoneOffsetLabel('Asia/Tokyo')).toMatch(/\+0?9/);
		});

		it('returns empty for an unknown zone', () => {
			expect(timezoneOffsetLabel('Not/AZone')).toBe('');
		});

		it('memoizes per zone, building the formatter only once', () => {
			const spy = vi.spyOn(Intl, 'DateTimeFormat');
			// A zone unused elsewhere, so the module-level cache starts cold for it.
			timezoneOffsetLabel('Europe/Paris');
			timezoneOffsetLabel('Europe/Paris');
			expect(spy).toHaveBeenCalledTimes(1);
			spy.mockRestore();
		});
	});

	describe('formatSensorValue', () => {
		it('formats temperature to 1 decimal with unit', () => {
			expect(formatSensorValue('temperature', 22.55)).toBe('22.6 °C');
		});

		it('formats humidity to 0 decimals with unit', () => {
			expect(formatSensorValue('humidity', 48.7)).toBe('49 %');
		});

		it('formats non-zero motion as Detected', () => {
			expect(formatSensorValue('motion', 1)).toBe('Detected');
		});

		it('formats zero motion as Clear', () => {
			expect(formatSensorValue('motion', 0)).toBe('Clear');
		});

		it('falls back to string for unknown kind', () => {
			expect(formatSensorValue('pressure', 1013)).toBe('1013');
		});
	});

	describe('timeAgo', () => {
		const base = new Date('2026-05-18T12:00:00Z').getTime();

		it('shows seconds for < 1 minute', () => {
			expect(timeAgo(new Date(base - 30_000).toISOString(), base)).toBe('30s ago');
		});

		it('shows minutes for < 1 hour', () => {
			expect(timeAgo(new Date(base - 5 * 60_000).toISOString(), base)).toBe('5m ago');
		});

		it('shows hours for >= 1 hour', () => {
			expect(timeAgo(new Date(base - 2 * 3600_000).toISOString(), base)).toBe('2h ago');
		});

		it('switches from seconds to minutes exactly at 60s', () => {
			expect(timeAgo(new Date(base - 59_000).toISOString(), base)).toBe('59s ago');
			expect(timeAgo(new Date(base - 60_000).toISOString(), base)).toBe('1m ago');
		});

		it('switches from minutes to hours exactly at 3600s', () => {
			expect(timeAgo(new Date(base - 3_599_000).toISOString(), base)).toBe('59m ago');
			expect(timeAgo(new Date(base - 3_600_000).toISOString(), base)).toBe('1h ago');
		});
	});

	describe('SENSOR_STALE_MS', () => {
		it('is ten minutes in milliseconds', () => {
			expect(SENSOR_STALE_MS).toBe(10 * 60 * 1000);
		});
	});

	describe('isSensorStale', () => {
		it('returns true when timestamp is undefined', () => {
			expect(isSensorStale(undefined)).toBe(true);
		});

		it('returns true when timestamp is empty string', () => {
			expect(isSensorStale('')).toBe(true);
		});

		it('returns false for a just-received timestamp', () => {
			expect(isSensorStale(new Date().toISOString())).toBe(false);
		});

		it('returns true when timestamp exceeds default threshold', () => {
			const old = new Date(Date.now() - SENSOR_STALE_MS - 1000).toISOString();
			expect(isSensorStale(old)).toBe(true);
		});

		it('returns false just inside the default threshold', () => {
			const recent = new Date(Date.now() - SENSOR_STALE_MS + 5000).toISOString();
			expect(isSensorStale(recent)).toBe(false);
		});

		it('respects a custom maxAgeMs', () => {
			const ts = new Date(Date.now() - 5000).toISOString();
			expect(isSensorStale(ts, 3000)).toBe(true);
			expect(isSensorStale(ts, 10_000)).toBe(false);
		});

		it('measures staleness against an injected now (so a ticking clock fades it)', () => {
			const ts = new Date(1_000_000).toISOString();
			expect(isSensorStale(ts, SENSOR_STALE_MS, 1_000_000 + SENSOR_STALE_MS - 1)).toBe(false);
			expect(isSensorStale(ts, SENSOR_STALE_MS, 1_000_000 + SENSOR_STALE_MS + 1)).toBe(true);
		});
	});

	describe('resolveOutsideTemp', () => {
		const freshSensor = (value: number): SensorPayload => ({
			device_id: 'wittboy',
			role: 'outside',
			kind: 'temperature',
			value,
			timestamp: new Date().toISOString()
		});
		const staleSensor = (value: number): SensorPayload => ({
			device_id: 'wittboy',
			role: 'outside',
			kind: 'temperature',
			value,
			timestamp: new Date(Date.now() - SENSOR_STALE_MS - 1000).toISOString()
		});
		const owm: WeatherPayload = { icon_code: '01d', temp: 17.3, humidity: 55 };

		it('prefers a fresh outside sensor over OWM', () => {
			expect(resolveOutsideTemp({ 'outside:temperature': freshSensor(11.4) }, owm)).toBe('11.4');
		});

		it('falls back to OWM when the outside sensor is stale', () => {
			expect(resolveOutsideTemp({ 'outside:temperature': staleSensor(11.4) }, owm)).toBe('17.3');
		});

		it('falls back to OWM when no outside sensor is present', () => {
			expect(resolveOutsideTemp({}, owm)).toBe('17.3');
		});

		it('returns placeholder when neither sensor nor OWM provide a value', () => {
			expect(resolveOutsideTemp({}, null)).toBe('--');
		});

		it('returns placeholder when sensor stale and OWM absent', () => {
			expect(resolveOutsideTemp({ 'outside:temperature': staleSensor(11.4) }, null)).toBe('--');
		});

		it('ignores inside-temperature sensors', () => {
			const inside: SensorPayload = {
				device_id: 'living_room',
				role: 'inside',
				kind: 'temperature',
				value: 22.7,
				timestamp: new Date().toISOString()
			};
			expect(resolveOutsideTemp({ 'inside:temperature': inside }, owm)).toBe('17.3');
		});

		it('formats sensor value to 1 decimal', () => {
			expect(resolveOutsideTemp({ 'outside:temperature': freshSensor(11) }, owm)).toBe('11.0');
		});
	});
});
