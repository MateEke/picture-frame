import type { SensorPayload, WeatherPayload } from '$lib/api/types.gen';

/** SSE sensors-map key for a reading: role (or device_id) + kind, e.g. "inside:temperature". */
export function sensorKey(p: SensorPayload): string {
	return `${p.role || p.device_id}:${p.kind}`;
}

// Builds a formatter in timeZone, falling back to the browser zone if it's empty
// or unknown (Go's and the browser's tz databases can drift).
function dateTimeFormat(
	locale: string,
	opts: Intl.DateTimeFormatOptions,
	timeZone?: string
): Intl.DateTimeFormat {
	if (timeZone) {
		try {
			return new Intl.DateTimeFormat(locale, { ...opts, timeZone });
		} catch {
			/* fall through to the browser zone */
		}
	}
	return new Intl.DateTimeFormat(locale, opts);
}

/**
 * Part-wise clock pieces so the separator is styleable; `period` is "" for
 * 24-hour locales, and `periodFirst` is true where it precedes the time (zh, ko).
 */
export function formatClockParts(
	date: Date,
	locale: string,
	timeZone?: string
): { hours: string; separator: string; minutes: string; period: string; periodFirst: boolean } {
	const parts = dateTimeFormat(
		locale,
		{ hour: '2-digit', minute: '2-digit' },
		timeZone
	).formatToParts(date);
	const hourIdx = parts.findIndex((p) => p.type === 'hour');
	const minuteIdx = parts.findIndex((p) => p.type === 'minute');
	const periodIdx = parts.findIndex((p) => p.type === 'dayPeriod');
	const between = parts
		.slice(hourIdx + 1, minuteIdx)
		.map((p) => p.value)
		.join('')
		.trim();
	return {
		hours: parts[hourIdx]?.value ?? '',
		separator: between || ':',
		minutes: parts[minuteIdx]?.value ?? '',
		period: periodIdx === -1 ? '' : parts[periodIdx].value,
		periodFirst: periodIdx !== -1 && periodIdx < hourIdx
	};
}

// Memoized short UTC-offset labels per zone; offsets only shift at DST boundaries,
// and the picker that uses this is a short-lived admin view.
const tzOffsets = new Map<string, string>();

/** Short UTC-offset label for an IANA zone, e.g. "GMT+9"; "" if the zone is unknown. */
export function timezoneOffsetLabel(timeZone: string): string {
	const cached = tzOffsets.get(timeZone);
	if (cached !== undefined) return cached;
	let label: string;
	try {
		const parts = new Intl.DateTimeFormat(undefined, {
			timeZone,
			timeZoneName: 'shortOffset'
		}).formatToParts(new Date());
		label = parts.find((p) => p.type === 'timeZoneName')?.value ?? '';
	} catch {
		label = '';
	}
	tzOffsets.set(timeZone, label);
	return label;
}

/** Localized full weekday name, e.g. "Saturday" / "szombat". */
export function formatWeekday(date: Date, locale: string, timeZone?: string): string {
	return dateTimeFormat(locale, { weekday: 'long' }, timeZone).format(date);
}

/** Localized short month + day, e.g. "Jun 06" / "j\u00fan. 06.". */
export function formatMonthDay(date: Date, locale: string, timeZone?: string): string {
	return dateTimeFormat(locale, { month: 'short', day: '2-digit' }, timeZone).format(date);
}

/** Formats a raw sensor value with its unit for display. */
export function formatSensorValue(kind: string, value: number): string {
	if (kind === 'temperature') return `${value.toFixed(1)} °C`;
	if (kind === 'humidity') return `${value.toFixed(0)} %`;
	if (kind === 'motion') return value !== 0 ? 'Detected' : 'Clear';
	return String(value);
}

/** Human-readable "X ago" for an ISO timestamp; `now` (ms) is injectable for tests. */
export function timeAgo(iso: string, now = Date.now()): string {
	const s = Math.round((now - new Date(iso).getTime()) / 1000);
	if (s < 60) return `${s}s ago`;
	if (s < 3600) return `${Math.floor(s / 60)}m ago`;
	return `${Math.floor(s / 3600)}h ago`;
}

// Threshold for treating a sensor reading as stale.
export const SENSOR_STALE_MS = 10 * 60 * 1000;

// True if the timestamp is absent or older than maxAgeMs. `now` is injectable so a
// reactive caller can pass a ticking clock (and for tests); defaults to the wall clock.
export function isSensorStale(
	timestamp: string | undefined,
	maxAgeMs = SENSOR_STALE_MS,
	now = Date.now()
): boolean {
	if (!timestamp) return true;
	return now - new Date(timestamp).getTime() > maxAgeMs;
}

/**
 * Outside temperature for the kiosk overlay: a fresh role="outside" sensor wins,
 * else OWM, else placeholder. (Icon stays OWM-only, local stations rarely map to it.)
 */
export function resolveOutsideTemp(
	sensors: Record<string, SensorPayload>,
	weather: WeatherPayload | null | undefined
): string {
	const s = sensors['outside:temperature'];
	if (s && !isSensorStale(s.timestamp)) return s.value.toFixed(1);
	const t = weather?.temp;
	return t !== undefined ? t.toFixed(1) : '--';
}
